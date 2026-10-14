// Exercise the published SDK implementation and a freshly built CLI against
// the same loopback server. No configured organizations or API keys are read.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { spawn } = require('node:child_process');
const { once } = require('node:events');
const { SyllableSDK, HTTPClient } = require('syllable-sdk');
const sdkRoot = path.dirname(require.resolve('syllable-sdk/package.json'));
const components = require(path.join(sdkRoot, 'models/components/index.js'));
const operations = require(path.join(sdkRoot, 'models/operations/index.js'));
const z = require('node:module').createRequire(path.join(sdkRoot, 'index.js'))('zod/v3');
const manifest = require('./sdk-operations.json');
const repo = path.resolve(__dirname, '../..');
const moduleDir = path.join(repo, 'scripts/syllable-cli');
const spec = JSON.parse(fs.readFileSync(path.join(moduleDir, 'internal/spec/openapi.json')));
const specOperations = Object.fromEntries(Object.values(spec.paths).flatMap(methods => Object.values(methods).filter(operation => operation.operationId).map(operation => [operation.operationId, operation])));
const exclusions = {
  organizations_create: 'Organization provisioning is intentionally console-only (AGENTS.md).',
  organizations_delete: 'Organization deletion is intentionally console-only (AGENTS.md).',
};
const binaryPayload = Buffer.from([0, 1, 17, 127, 128, 255, 10]);
const files = Buffer.from('name,phone\nAudit,+12025550123\n');
const fileName = 'audit.csv';
const values = {
  tool_name: 'audit', test_id: 'audit-test', job_id: 'audit-job', file_name: 'audit.bin',
  user_email: 'audit@example.invalid', email: 'audit@example.invalid',
  channel_manager_service: 'twilio', channel_manager_sid: 'audit-sid', tool_call_id: 'audit-call',
  call_id: 'audit-call', token: 'audit', dashboard_name: 'audit', display_name: 'audit',
  ip_range: '203.0.113.0/24', timezone: 'UTC',
  start_datetime: '2030-12-24T09:30:00Z', end_datetime: '2030-12-25T09:30:00Z',
};

function schemaFor(reference, direction) {
  const [namespace, name] = reference.split('.');
  return { components, operations }[namespace][`${name}$${direction}Schema`];
}

// Walk the pinned SDK's Zod input schemas. Include every reachable optional
// property, with false/0/empty values where the declared type allows them.
// Recursive/opaque records and unions receive representative finite values;
// this is transport coverage, not exhaustive backend semantic validation.
function fixture(schema, key = '', seen = new Map()) {
  const d = schema._def;
  const depth = seen.get(schema) || 0;
  if (depth > 1) return undefined;
  const next = new Map(seen).set(schema, depth + 1);
  const child = (s, k = key) => fixture(s, k, next);
  switch (d.typeName) {
    case 'ZodEffects': return child(d.schema);
    case 'ZodLazy': return child(d.getter());
    case 'ZodOptional': return child(d.innerType);
    case 'ZodDefault': return child(d.innerType);
    case 'ZodNullable': return child(d.innerType) ?? null;
    case 'ZodCatch': return child(d.innerType);
    case 'ZodUnion': {
      for (const option of d.options) {
        try {
          const value = child(option);
          if (option.safeParse(value).success) return value;
        } catch (_) { /* try next union member */ }
      }
      return undefined;
    }
    case 'ZodDiscriminatedUnion': return child(d.options[0]);
    case 'ZodIntersection': return { ...child(d.left), ...child(d.right) };
    case 'ZodObject': {
      const shape = d.shape();
      if ('fileName' in shape && 'content' in shape) return { fileName, content: new Uint8Array(files) };
      return Object.fromEntries(Object.entries(shape).map(([k, s]) => [k, child(s, k)]).filter(([, v]) => v !== undefined));
    }
    case 'ZodRecord': {
      const name = fixture(d.keyType, 'record_key', next);
      return { [name || 'audit']: child(d.valueType, 'record_value') ?? 'audit' };
    }
    case 'ZodArray': {
      if (key === 'search_field_values') return ['audit, value + / &= ü', ''];
      const item = child(d.type, key);
      if (item === undefined) return [];
      const enumSchema = unwrap(d.type);
      if (['fields', 'search_fields'].includes(key)) {
        if (enumSchema._def.typeName === 'ZodEnum') return enumSchema._def.values.slice(0, 2);
        if (enumSchema._def.typeName === 'ZodNativeEnum') return Object.values(enumSchema._def.values).filter(v => typeof v === 'string').slice(0, 2);
      }
      return Array.from({ length: Math.max(d.minLength?.value || 0, 1) }, () => item);
    }
    case 'ZodTuple': return d.items.map(s => child(s));
    case 'ZodEnum': return d.values[0];
    case 'ZodNativeEnum': return Object.values(d.values).find(v => typeof v === 'string') || 0;
    case 'ZodLiteral': return d.value;
    case 'ZodBoolean': return false;
    case 'ZodNumber': {
      let value = /(^id$|_id$|^version$|version_number)/.test(key) ? 17 : 0;
      if (key === 'limit') value = 17;
      for (const check of d.checks || []) {
        if (check.kind === 'min' && value < check.value) value = check.value + (check.inclusive ? 0 : 1);
        if (check.kind === 'max' && value > check.value) value = check.value - (check.inclusive ? 0 : 1);
        if (check.kind === 'multipleOf') value = Math.ceil(value / check.value) * check.value;
      }
      return value;
    }
    case 'ZodString': {
      if (values[key] !== undefined) return values[key];
      if (key === 'id' || key.endsWith('_id')) return '17';
      if (key === 'sunset_date') return '2030-12-25';
      const checks = d.checks || [];
      if (checks.some(c => c.kind === 'datetime')) return '2030-12-25T09:30:00.000Z';
      if (checks.some(c => c.kind === 'date')) return '2030-12-25';
      if (checks.some(c => c.kind === 'email') || key.includes('email')) return 'audit@example.invalid';
      if (checks.some(c => c.kind === 'url')) return 'https://example.invalid/audit';
      if (checks.some(c => c.kind === 'uuid')) return '00000000-0000-4000-8000-000000000017';
      let value = ['name', 'tool_name', 'provider', 'model', 'type', 'target'].includes(key) ? 'audit' : '';
      for (const check of checks) {
        if (check.kind === 'min' && value.length < check.value) value = 'a'.repeat(check.value);
        if (check.kind === 'max') value = value.slice(0, check.value);
      }
      return value;
    }
    case 'ZodDate': return new Date('2030-12-25T09:30:00.000Z');
    case 'ZodAny': case 'ZodUnknown': return { audit: false, zero: 0, empty: '', nullable: null };
    case 'ZodNull': return null;
    case 'ZodVoid': case 'ZodUndefined': return undefined;
    default: return undefined;
  }
}

function unwrap(schema) {
  for (;;) {
    const d = schema._def;
    if (d.schema) schema = d.schema;
    else if (d.innerType) schema = d.innerType;
    else if (d.typeName === 'ZodLazy') schema = d.getter();
    else return schema;
  }
}

// Populate explicit nulls in nullable request properties. Preserve identifying
// fields because collection PUTs route by their body identifier.
function nullFixture(schema, value, key = '') {
  const d = schema._def;
  if (d.typeName === 'ZodNullable' && key !== 'id' && !key.endsWith('_id')) return null;
  if (d.schema) return nullFixture(d.schema, value, key);
  if (d.innerType) return nullFixture(d.innerType, value, key);
  if (d.typeName === 'ZodLazy') return nullFixture(d.getter(), value, key);
  if (d.typeName === 'ZodObject' && value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([k, v]) => [k, d.shape()[k] ? nullFixture(d.shape()[k], v, k) : v]));
  }
  if (d.typeName === 'ZodArray' && Array.isArray(value)) return value.map(v => nullFixture(d.type, v, key));
  if (d.typeName === 'ZodUnion') {
    const option = d.options.find(s => s.safeParse(value).success);
    if (option) return nullFixture(option, value, key);
  }
  return value;
}

function cliMap() {
  const source = fs.readFileSync(path.join(moduleDir, 'cmd/sdk_operations.go'), 'utf8');
  const result = {};
  for (const m of source.matchAll(/"([^"]+)":\s*\{([^\n]+)\},/g)) {
    const args = [...m[2].matchAll(/"((?:\\.|[^"\\])*)"/g)].map(m => JSON.parse(`"${m[1]}"`));
    result[m[1]] = args;
  }
  assert(Object.keys(result).length > 0, 'CLI operation registry is empty');
  return result;
}

function flagName(key, id) {
  const aliases = { fields: 'api-fields', search_fields: 'search-fields', search_field_values: 'search-values', start_datetime: 'start-date', end_datetime: 'end-date', dashboard_name: 'name', response_format: 'format' };
  if (key === 'selected_model') return 'selected-model';
  if (key === 'selected_voice') return 'selected-voice';
  if (key === 'selected_provider') return 'selected-provider';
  if (key === 'response_format' && !['directory_member_list', 'directory_member_download'].includes(id)) return 'response-format';
  return aliases[key] || key.replaceAll('_', '-');
}

function setFlag(argv, name, value) {
  for (let i = argv.length - 1; i >= 0; i--) {
    if (argv[i] === `--${name}`) argv.splice(i, 2);
  }
  if (Array.isArray(value)) {
    if (name === 'api-fields') argv.push(`--${name}=${value.join(',')}`);
    else for (const v of value) argv.push(`--${name}=${v}`);
  } else argv.push(`--${name}=${value}`);
}

function normalizeQuery(url) {
  const query = {};
  for (const [k, v] of url.searchParams) (query[k] ||= []).push(v);
  return Object.fromEntries(Object.entries(query).sort(([a], [b]) => a.localeCompare(b)));
}

function equivalentDefaults(request, id) {
  const result = { ...request, query: { ...request.query } };
  // Only this omitted-options scenario uses documented default equivalence.
  // Explicit-options scenarios above remain strict and cannot hide omissions.
  for (const p of specOperations[id].parameters || []) {
    if (p.in !== 'query' || p.schema.default === undefined) continue;
    const actual = result.query[p.name];
    const expected = Array.isArray(p.schema.default) ? p.schema.default.map(String) : [String(p.schema.default)];
    if (actual && JSON.stringify(actual) === JSON.stringify(expected)) delete result.query[p.name];
  }
  return result;
}

async function normalizeRequest(request) {
  const url = new URL(request.url);
  const ctype = request.headers.get('content-type')?.split(';')[0] || null;
  let body = null;
  if (ctype === 'application/json') body = await request.json();
  else if (ctype === 'application/x-www-form-urlencoded') body = normalizeQuery(new URL(`http://localhost/?${await request.text()}`));
  else if (ctype === 'multipart/form-data') {
    body = {};
    for (const [key, value] of await request.formData()) {
      const entry = typeof value === 'string' ? value : { filename: value.name, contentType: value.type, content: Buffer.from(await value.arrayBuffer()).toString('base64') };
      (body[key] ||= []).push(entry);
    }
    body = Object.fromEntries(Object.entries(body).sort(([a], [b]) => a.localeCompare(b)));
  } else {
    const bytes = Buffer.from(await request.arrayBuffer());
    body = bytes.length ? bytes.toString('base64') : null;
  }
  // Decode each segment independently: @ and percent-escaped @ are the same
  // route, while an escaped slash remains distinguishable from a delimiter.
  return { method: request.method, path: url.pathname.split('/').map(decodeURIComponent), query: normalizeQuery(url), contentType: ctype, accept: request.headers.get('accept'), apiKey: request.headers.get('syllable-api-key'), body };
}

function run(program, argv, cwd, env, input) {
  return new Promise((resolve, reject) => {
    const child = spawn(program, argv, { cwd, env, stdio: ['pipe', 'pipe', 'pipe'] });
    const stdout = [], stderr = [];
    child.stdout.on('data', d => stdout.push(d));
    child.stderr.on('data', d => stderr.push(d));
    child.stdin.end(input);
    child.on('error', reject);
    child.on('close', code => resolve({ code, stdout: Buffer.concat(stdout), stderr: Buffer.concat(stderr).toString() }));
  });
}

async function main() {
  assert.equal(require(path.join(sdkRoot, 'package.json')).version, manifest.version);
    const commands = cliMap();
    // Expected totals derive from the pinned manifest, never from constants, so
    // neither a spec sync nor a deliberate SDK pin bump needs a manual count edit.
    const allowed = Object.keys(manifest.operations).filter(id => !exclusions[id]);
    const unmapped = allowed.filter(id => !commands[id]);
    assert.deepEqual(unmapped, [], 'Every allowed operation in the pinned SDK must have a CLI mapping');
    // A spec sync can add commands before the pinned SDK knows them. The Go
    // coverage tests still gate those; parity here waits for the next pin bump.
    const pending = Object.keys(commands).filter(id => !manifest.operations[id]);
    if (pending.length) console.log(`Parity pending until the SDK pin moves past ${manifest.version}: ${pending.sort().join(', ')}`);
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'syllable-parity-'));
  const binary = process.env.SYLLABLE_PARITY_BINARY || path.join(temp, 'syllable');
  let scenario, captured = [], server;
  try {
    if (!process.env.SYLLABLE_PARITY_BINARY) {
      const build = await run('go', ['build', '-o', binary, '.'], moduleDir, process.env);
      assert.equal(build.code, 0, build.stderr);
    }
    server = http.createServer(async (req, res) => {
      try {
        const chunks = [];
        for await (const chunk of req) chunks.push(chunk);
        const body = Buffer.concat(chunks);
        const request = new Request(`http://127.0.0.1${req.url}`, { method: req.method, headers: req.headers, ...(body.length ? { body } : {}) });
        captured.push(await normalizeRequest(request));
        res.writeHead(scenario.status, { 'Content-Type': scenario.contentType });
        res.end(scenario.body);
      } catch (error) {
        res.writeHead(500); res.end(String(error));
      }
    });
    server.listen(0, '127.0.0.1');
    await once(server, 'listening');
    const origin = `http://127.0.0.1:${server.address().port}`;
    const config = path.join(temp, 'mock.yaml');
    fs.writeFileSync(config, `default_org: parity\ndefault_env: parity\norgs:\n  parity:\n    api_key: unit-test\nenvironments:\n  parity:\n    base_url: ${origin}\n`);
    const cliEnv = { ...process.env, SYLLABLE_API_KEY: 'unit-test', SYLLABLE_ORG: 'parity', SYLLABLE_ENV: 'parity' };
    // Reject any non-loopback SDK request before fetch. The CLI receives only
    // the generated mock config and cannot resolve the user's real settings.
    const sdk = new SyllableSDK({ serverURL: origin, apiKeyHeader: 'unit-test', httpClient: new HTTPClient({ fetcher: (request) => {
      assert.equal(new URL(request.url).origin, origin, 'SDK attempted a non-mock request');
      return fetch(request);
    } }) });
    const prefix = ['--config', config, '--org', 'parity', '--env', 'parity', '--output', 'json', '--yes'];
    const failures = [];
    const exchanges = new Map();
    let passed = 0, jsonResponses = 0, streams = 0, noContent = 0, nullableBodies = 0, errorCases = 0, alternateFiles = 0, defaultQueries = 0, anonymousEmail = 0;
    for (const [id, entry] of Object.entries(manifest.operations)) {
      if (exclusions[id]) continue;
      if (process.env.SYLLABLE_PARITY_FILTER && !id.includes(process.env.SYLLABLE_PARITY_FILTER)) continue;
      try {
        const source = fs.readFileSync(path.join(sdkRoot, 'funcs', `${entry.module}.js`));
        assert.equal(crypto.createHash('sha256').update(source).digest('hex'), entry.sourceSHA256, `SDK source drift: ${id}`);
        assert(commands[id], `unmapped CLI operation ${id}`);
        const inputSchema = entry.input && schemaFor(entry.input, 'inbound');
        const input = inputSchema ? fixture(inputSchema) : undefined;
        if (id === 'send_test_message') input.agent_id = '17';
        if (input?.search_fields && input?.search_field_values) input.search_field_values = input.search_field_values.slice(0, input.search_fields.length);
        const typed = inputSchema ? inputSchema.parse(input) : undefined;
        const wire = entry.input ? schemaFor(entry.input, 'outbound').parse(typed) : undefined;
        const argv = [...commands[id]];
        const payload = entry.jsonBody === '$request' ? wire : wire?.[entry.jsonBody];
        const bodyFile = path.join(temp, `${id}.json`);
        fs.writeFileSync(bodyFile, JSON.stringify(payload ?? {}));
        if (!entry.jsonBody && !entry.multipart) {
          const fileFlag = argv.indexOf('--file');
          if (fileFlag >= 0) argv.splice(fileFlag, 2);
        }
        for (let i = 0; i < argv.length; i++) if (argv[i] === '$body') argv[i] = entry.multipart ? path.join(temp, fileName) : bodyFile;
        fs.writeFileSync(path.join(temp, fileName), files);
        if (entry.jsonBody && !argv.includes('--file') && !['channels_twilio_numbers_a2p_compliance_check', 'directory_member_restore', 'organization_sip_ip_ranges_create', 'organization_sip_ip_ranges_update'].includes(id)) {
          // Exercise full SDK request-body input instead of convenience flags.
          const firstFlag = argv.findIndex(v => v.startsWith('--'));
          if (firstFlag !== -1) argv.splice(firstFlag);
          argv.push('--file', bodyFile);
        }
        if (id === 'send_test_message') setFlag(argv, 'file', bodyFile);
        if (id === 'insights_workflow_activate') setFlag(argv, 'file', bodyFile);
        if (id === 'users_delete') setFlag(argv, 'file', bodyFile);
        if (id === 'directory_member_restore') setFlag(argv, 'comments', payload.comments);
        if (id === 'channels_twilio_numbers_a2p_compliance_check') setFlag(argv, 'phone', payload.phone);
        if (id === 'outbound_batch_delete') setFlag(argv, 'reason', wire[entry.formBody].delete_reason);
        for (const key of entry.query) {
          if (key === 'target_id' && id === 'channel_targets_delete') continue;
          if (wire?.[key] === null || wire?.[key] === undefined) continue;
          setFlag(argv, flagName(key, id), wire[key]);
        }
        if (id === 'organization_sip_ip_ranges_create' || id === 'organization_sip_ip_ranges_update') {
          for (const [key, value] of Object.entries(payload)) setFlag(argv, key.replaceAll('_', '-'), value);
        }
        if (id === 'organizations_update') {
          for (const [key, value] of Object.entries(wire)) {
            if (key === 'logo') setFlag(argv, 'logo', path.join(temp, fileName));
            else if (value !== null && value !== undefined) setFlag(argv, key.replaceAll('_', '-'), value);
          }
        }
        const responseSchema = new Function('components', 'operations', 'z', `return ${entry.response.schema}`)(components, operations, z);
        const response = fixture(responseSchema);
        if (entry.response.type === 'json') responseSchema.parse(response);
        scenario = { status: entry.response.status, contentType: entry.response.contentType, body: entry.response.type === 'stream' ? binaryPayload : entry.response.type === 'nil' ? '' : JSON.stringify(response) };
        const func = require(path.join(sdkRoot, 'funcs', `${entry.module}.js`))[entry.export];
        captured = [];
        const sdkResult = await (entry.input ? func(sdk, typed) : func(sdk));
        assert(sdkResult.ok, `SDK fixture rejected: ${sdkResult.error?.message}`);
        assert.equal(captured.length, 1, 'SDK must issue exactly one mock request');
        const sdkRequest = captured[0];
        if (entry.response.type === 'stream') {
          const bytes = Buffer.from(await new Response(sdkResult.value).arrayBuffer());
          assert.deepEqual(bytes, binaryPayload);
        } else if (entry.response.type === 'json') assert.deepEqual(sdkResult.value, responseSchema.parse(response));
        captured = [];
        const cli = await run(binary, [...prefix, ...argv], temp, cliEnv);
        assert.equal(cli.code, 0, `CLI failed: ${cli.stderr}`);
        assert.equal(captured.length, 1, `CLI must issue exactly one mock request: ${JSON.stringify(captured)}`);
        assert.deepEqual(captured[0], sdkRequest, `wire request mismatch for ${id}`);
        if (entry.response.type === 'stream') { assert.deepEqual(cli.stdout, binaryPayload); streams++; }
        else if (entry.response.type === 'json') { assert.deepEqual(JSON.parse(cli.stdout.toString()), response); jsonResponses++; }
        else noContent++;
        exchanges.set(id, { entry, func, typed, argv, input, inputSchema, response });
        if (entry.jsonBody && argv.includes('--file')) {
          const nullableInput = entry.jsonBody === '$request' ? nullFixture(inputSchema, input) : {
            ...input,
            [entry.jsonBody]: nullFixture(unwrap(inputSchema)._def.shape()[entry.jsonBody], input[entry.jsonBody]),
          };
          const nullableTyped = inputSchema.parse(nullableInput);
          const nullableWire = schemaFor(entry.input, 'outbound').parse(nullableTyped);
          const nullablePayload = entry.jsonBody === '$request' ? nullableWire : nullableWire[entry.jsonBody];
          if (JSON.stringify(nullablePayload) !== JSON.stringify(payload)) {
            fs.writeFileSync(bodyFile, JSON.stringify(nullablePayload));
            captured = [];
            const sdkNull = await func(sdk, nullableTyped);
            assert(sdkNull.ok, `SDK nullable fixture rejected: ${sdkNull.error?.message}`);
            const sdkNullRequest = captured[0];
            captured = [];
            const cliNull = await run(binary, [...prefix, ...argv], temp, cliEnv);
            assert.equal(cliNull.code, 0, cliNull.stderr);
            assert.equal(captured.length, 1);
            assert.deepEqual(captured[0], sdkNullRequest, `explicit-null body mismatch for ${id}`);
            nullableBodies++;
            fs.writeFileSync(bodyFile, JSON.stringify(payload));
          }
        }
        passed++;
      } catch (error) {
        failures.push({ id, error });
        console.error(`FAIL ${id}: ${error.stack || error}`);
      }
    }
    const email = exchanges.get('users_send_email');
    if (email) {
      try {
        // The SDK permits skip_auth=true without apiKeyHeader. Use a distinct
        // synthetic config with no org/key and remove credential environment
        // variables to prove the CLI does not depend on user setup for this
        // explicitly anonymous operation. Both requests remain on loopback.
        const anonymousConfig = path.join(temp, 'anonymous-mock.yaml');
        fs.writeFileSync(anonymousConfig, `default_env: parity\nenvironments:\n  parity:\n    base_url: ${origin}\n`);
        const anonymousEnv = { ...cliEnv };
        delete anonymousEnv.SYLLABLE_API_KEY;
        delete anonymousEnv.SYLLABLE_ORG;
        const anonymousSDK = new SyllableSDK({ serverURL: origin, httpClient: new HTTPClient({ fetcher: request => {
          assert.equal(new URL(request.url).origin, origin);
          return fetch(request);
        } }) });
        scenario = { status: email.entry.response.status, contentType: email.entry.response.contentType, body: JSON.stringify(email.response) };
        captured = [];
        const sdkEmail = await email.func(anonymousSDK, { ...email.typed, skipAuth: true });
        assert(sdkEmail.ok, sdkEmail.error?.message);
        assert.equal(captured.length, 1);
        const sdkRequest = captured[0];
        assert.equal(sdkRequest.apiKey, null);
        captured = [];
        const cliEmail = await run(binary, ['--config', anonymousConfig, '--env', 'parity', '--output', 'json', '--yes', 'users', 'send-email', 'audit@example.invalid', '--skip-auth=true'], temp, anonymousEnv);
        assert.equal(cliEmail.code, 0, cliEmail.stderr);
        assert.equal(captured.length, 1);
        assert.deepEqual(captured[0], sdkRequest);
        assert.deepEqual(JSON.parse(cliEmail.stdout.toString()), email.response);
        anonymousEmail++;
      } catch (error) {
        failures.push({ id: 'users_send_email:anonymous', error });
        console.error(`FAIL anonymous users_send_email: ${error.stack || error}`);
      }
    }
    for (const [id, exchange] of exchanges) {
      const { entry, input, inputSchema } = exchange;
      if (entry.jsonBody || entry.multipart || !entry.query.length) continue;
      try {
        const minimal = { ...input };
        const shape = unwrap(inputSchema)._def.shape();
        const args = [...commands[id]];
        for (const key of entry.query) {
          if (shape[key]?.safeParse(undefined).success) delete minimal[key];
          else if (!(id === 'channel_targets_delete' && key === 'target_id')) setFlag(args, flagName(key, id), minimal[key]);
        }
        const fileFlag = args.indexOf('--file');
        if (fileFlag >= 0) args.splice(fileFlag, 2);
        const minimalTyped = inputSchema.parse(minimal);
        scenario = { status: entry.response.status, contentType: entry.response.contentType, body: entry.response.type === 'stream' ? binaryPayload : entry.response.type === 'nil' ? '' : JSON.stringify(exchange.response) };
        captured = [];
        const sdkDefault = await exchange.func(sdk, minimalTyped);
        assert(sdkDefault.ok, sdkDefault.error?.message);
        if (entry.response.type === 'stream') await new Response(sdkDefault.value).arrayBuffer();
        assert.equal(captured.length, 1);
        const sdkRequest = captured[0];
        captured = [];
        const cliDefault = await run(binary, [...prefix, ...args], temp, cliEnv);
        assert.equal(cliDefault.code, 0, cliDefault.stderr);
        assert.equal(captured.length, 1);
        assert.deepEqual(equivalentDefaults(captured[0], id), equivalentDefaults(sdkRequest, id));
        defaultQueries++;
      } catch (error) {
        failures.push({ id: `${id}:defaults`, error });
        console.error(`FAIL ${id} omitted queries: ${error.stack || error}`);
      }
    }
    // Exercise SDK-native File input with a custom filename/MIME type, and
    // the CLI's corresponding upload overrides, including stdin bytes.
    const upload = exchanges.get('directory_member_bulk_load');
    if (upload) {
      for (const stdin of [false, true]) {
        try {
          scenario = { status: upload.entry.response.status, contentType: upload.entry.response.contentType, body: JSON.stringify(upload.response) };
          const name = 'arbitrary.audio';
          const contentType = 'audio/x-parity-test';
          captured = [];
          const sdkUpload = await upload.func(sdk, { file: new File([files], name, { type: contentType }) });
          assert(sdkUpload.ok, sdkUpload.error?.message);
          const sdkRequest = captured[0];
          const args = [...upload.argv];
          setFlag(args, 'upload-filename', name);
          setFlag(args, 'upload-content-type', contentType);
          if (stdin) setFlag(args, 'file', '-');
          captured = [];
          const cliUpload = await run(binary, [...prefix, ...args], temp, cliEnv, stdin ? files : undefined);
          assert.equal(cliUpload.code, 0, cliUpload.stderr);
          assert.equal(captured.length, 1);
          assert.deepEqual(captured[0], sdkRequest);
          assert.deepEqual(JSON.parse(cliUpload.stdout.toString()), upload.response);
          alternateFiles++;
        } catch (error) {
          failures.push({ id: `directory_member_bulk_load:stdin=${stdin}`, error });
          console.error(`FAIL custom upload stdin=${stdin}: ${error.stack || error}`);
        }
      }
    }
    // The SDK exposes typed exceptions; the CLI exposes exit status plus a
    // structured JSON envelope. Compare their status and preserved raw body,
    // rather than requiring identical language-specific error classes.
    const errorIDs = ['agent_list', 'agent_create', 'users_delete', 'outbound_batch_delete', 'directory_member_bulk_load', 'voice_groups_create_voice_sample', 'users_delete_account'];
    for (const id of errorIDs) {
      const exchange = exchanges.get(id);
      if (!exchange) continue;
      for (const status of [400, 401, 403, 404, 409, 422, 500]) {
        try {
          const errorBody = { detail: [{ loc: ['body', 'audit'], msg: 'Synthetic validation error', type: 'value_error' }], diagnostic: { nullable: null, enabled: false, zero: 0, empty: '' } };
          scenario = { status, contentType: 'application/json', body: JSON.stringify(errorBody) };
          captured = [];
          const sdkError = await (exchange.entry.input ? exchange.func(sdk, exchange.typed) : exchange.func(sdk));
          assert.equal(sdkError.ok, false);
          assert.equal(sdkError.error.statusCode, status);
          assert.deepEqual(JSON.parse(sdkError.error.body), errorBody);
          assert.equal(captured.length, 1);
          const sdkErrorRequest = captured[0];
          captured = [];
          const cliError = await run(binary, [...prefix, ...exchange.argv], temp, cliEnv);
          assert.notEqual(cliError.code, 0);
          assert.equal(captured.length, 1);
          assert.deepEqual(captured[0], sdkErrorRequest);
          const errorLine = cliError.stderr.split('\n').find(line => line.startsWith('{"error":'));
          assert(errorLine, `missing JSON error: ${cliError.stderr}`);
          const parsed = JSON.parse(errorLine).error;
          assert.equal(parsed.status_code, status);
          assert.deepEqual(parsed.detail, errorBody);
          errorCases++;
        } catch (error) {
          failures.push({ id: `${id}:status=${status}`, error });
          console.error(`FAIL ${id} status=${status}: ${error.stack || error}`);
        }
      }
    }
    console.log(`SDK ${manifest.version}: ${passed} operation requests matched; ${jsonResponses} JSON responses, ${streams} binary streams, ${noContent} empty responses; ${nullableBodies} explicit-null request bodies; ${defaultQueries} omitted-query defaults; ${alternateFiles} custom-file/stdin uploads; ${anonymousEmail} anonymous skip-auth email request; ${errorCases} HTTP error cases; ${Object.keys(exclusions).length} documented console-only exclusions.`);
    assert.equal(failures.length, 0, `${failures.length} SDK/CLI contract failures`);
    if (!process.env.SYLLABLE_PARITY_FILTER) {
      assert.equal(passed, allowed.length, 'All allowed operations must execute without skips');
      assert(nullableBodies > 0, 'Mapped nullable JSON bodies must receive explicit-null cases');
      assert(defaultQueries > 0, 'Body-free query operations must exercise omitted defaults');
      assert.equal(errorCases, errorIDs.filter(id => allowed.includes(id)).length * 7, 'All seven status codes must be checked across every error-transport operation');
      assert.equal(alternateFiles, 2, 'Custom multipart metadata must work for files and stdin');
      assert.equal(anonymousEmail, 1, 'Explicit skip-auth email must work without configured credentials');
    }
  } finally {
    server?.close();
    fs.rmSync(temp, { recursive: true, force: true });
  }
}
main().catch(error => { console.error(error.stack || error); process.exitCode = 1; });
