// Refresh only when deliberately updating the pinned SDK. The checked-in
// manifest is an independent snapshot of the published SDK implementation,
// rather than a derivation of the CLI's embedded OpenAPI description.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const sdkRoot = path.dirname(require.resolve('syllable-sdk/package.json'));
const version = require(path.join(sdkRoot, 'package.json')).version;
const operations = {};
for (const file of fs.readdirSync(path.join(sdkRoot, 'funcs')).filter(f => f.endsWith('.js'))) {
  const source = fs.readFileSync(path.join(sdkRoot, 'funcs', file), 'utf8');
  const id = source.match(/operationID: "([^"]+)"/)?.[1];
  if (!id) continue;
  if (operations[id]) throw new Error(`duplicate SDK operation ${id}`);
  const input = source.match(/(components|operations)\.(\w+)\$outboundSchema\s*\.parse\(value\)/);
  const response = source.match(/M\.match\(M\.(json|stream|nil)\((\d+), (.+?)(?:, \{\n|\), M\.)/s);
  if (!response) throw new Error(`unrecognized response for ${id}`);
  const query = source.match(/encodeFormQuery\)\(\{([\s\S]*?)\n    \}\)/)?.[1] || '';
  const body = source.match(/encodeJSON\)\("body", payload(?:\.([\w]+))?[,)]/);
  operations[id] = {
    module: file.replace(/\.js$/, ''),
    export: source.match(/exports\.(\w+) = \w+;/)[1],
    sourceSHA256: crypto.createHash('sha256').update(source).digest('hex'),
    input: input ? `${input[1]}.${input[2]}` : null,
    query: [...query.matchAll(/"([^"]+)": payload\./g)].map(m => m[1]),
    jsonBody: body ? (body[1] || '$request') : null,
    formBody: source.match(/Object\.entries\(payload\.([\w]+) \|\| \{\}\)/)?.[1] || null,
    multipart: /const body = new FormData\(\)/.test(source),
    accept: source.match(/Accept: "([^"]+)"/)?.[1] || null,
    response: {
      type: response[1], status: Number(response[2]), schema: response[3],
      contentType: source.match(/ctype: "([^"]+)"/)?.[1] || (response[1] === 'json' ? 'application/json' : 'application/octet-stream'),
    },
  };
}
fs.writeFileSync(path.join(__dirname, 'sdk-operations.json'), JSON.stringify({
  package: 'syllable-sdk', version,
  commit: '6c6fc6f906f103b763dcffa65ccc6ea2403883d4',
  operations: Object.fromEntries(Object.entries(operations).sort(([a], [b]) => a.localeCompare(b))),
}, null, 2) + '\n');
console.log(`Snapshotted ${Object.keys(operations).length} operations from syllable-sdk@${version}`);
