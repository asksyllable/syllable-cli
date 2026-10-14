# SDK/CLI differential contracts

This suite runs the published `syllable-sdk@1.0.68` implementation and a freshly
built CLI against the same ephemeral loopback HTTP server. It reads no real CLI
configuration, uses synthetic credentials, and makes no Syllable platform calls.

```bash
npm ci --prefix tests/sdk-parity --ignore-scripts --no-audit --no-fund
npm test --prefix tests/sdk-parity
```

The pinned SDK manifest snapshots the implementation's operation IDs, generated
function exports, schemas, and source hashes. The CLI invocations come separately
from the reviewed `sdkOperationCommands` registry. A removed, unmapped, or changed
operation fails the suite. All 188 allowed operations must execute without skips;
organization create/delete remain console-only under `AGENTS.md`.

For each operation the suite checks the actual HTTP method, decoded path segments,
query values and repeated arrays, authentication header, Accept header, body
content type, and semantic body. JSON key order, query-key order, multipart
boundaries, and equivalent escaping are normalized. Explicit `false`, `0`, empty
strings, commas, spaces, plus signs, slashes, ampersands, equals signs, and Unicode
search values are included. File uploads compare filename, part content type, and
bytes. Nullable JSON properties receive an additional explicit-null case. The
suite also checks omitted query options, normalizing only defaults documented by
the embedded OpenAPI description; explicit query cases remain strict. Custom
file names and MIME types are checked for both file uploads and stdin.
The SDK's explicitly anonymous `users_send_email` request (`skip_auth=true`) is
also compared with no API key or configured organization, against loopback only.

Successful fixtures exercise the SDK's response decoding and the CLI's raw JSON,
binary streams, and empty responses. HTTP 400/401/403/404/409/422/500 responses are
checked across JSON reads/writes, JSON/form deletes, multipart uploads, binary
responses, and a body-free action: both clients must preserve the status and raw
error body, and the CLI must exit nonzero.

Fixture generation walks the pinned SDK's Zod schemas, including reachable
optional properties. Opaque records, recursive types, and union types receive
finite representative values. This verifies transport contracts, rather than
every union branch, backend business rule, traffic-routing behavior, or live
resource lifecycle. Live experiment routing, populated workflow results, and
card interactions still require dedicated platform fixtures.

For focused debugging, use `SYLLABLE_PARITY_FILTER=agent_list` or set
`SYLLABLE_PARITY_BINARY` to a previously built branch binary. CI uses the complete
suite and builds from source. If a restricted local sandbox blocks listening on
localhost, run with the normal local execution permission for this mock suite.

When deliberately updating the SDK, update the exact dependency version and
lockfile, refresh the SDK source commit in `generate-manifest.cjs`, and run
`npm run refresh-manifest --prefix tests/sdk-parity`. Review the manifest changes
alongside CLI changes; do not refresh it merely to silence a failed test.
