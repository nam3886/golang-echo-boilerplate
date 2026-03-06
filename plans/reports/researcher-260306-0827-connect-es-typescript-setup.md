# Connect-ES TypeScript Setup (2026)

**Date:** 2026-03-06
**Backend:** connectrpc.com/connect v1.19.1 (Go), buf.build codegen

---

## 1. Current State: v2 is GA

Connect-ES 2.0 (latest: v2.1.1, released Nov 2025) is the current stable version.
**NOT backward-compatible with v1.x** — requires Protobuf-ES 2.0.

Key architectural shift: messages are now plain JS objects (not ES6 classes).

---

## 2. buf.gen.yaml — Plugin Configuration

### v2: Single plugin (protoc-gen-es handles both messages AND services)

```yaml
version: v2
plugins:
  - local: protoc-gen-es
    out: src/gen
    opt:
      - target=ts
```

### With remote plugin (BSR):

```yaml
version: v2
plugins:
  - remote: buf.build/bufbuild/es
    out: src/gen
    opt:
      - target=ts
```

**Remote plugin path:** `buf.build/bufbuild/es`
Pin a version: `buf.build/bufbuild/es:VERSION` (check https://buf.build/bufbuild/es for latest)

### v1 (legacy — DO NOT use for new projects):

```yaml
# v1 required TWO plugins — now obsolete
plugins:
  - plugin: es          # messages only
  - plugin: connect-es  # service definitions
```

**`protoc-gen-connect-es` is no longer needed in v2.**

---

## 3. npm Packages

### Dev dependencies

```bash
npm install --save-dev @bufbuild/buf @bufbuild/protoc-gen-es
```

### Runtime dependencies

```bash
npm install @connectrpc/connect @connectrpc/connect-web @bufbuild/protobuf
```

| Package | Purpose |
|---|---|
| `@connectrpc/connect` | Core RPC client/server |
| `@connectrpc/connect-web` | Browser transport (fetch/grpc-web) |
| `@bufbuild/protobuf` | Protobuf runtime (v2) |
| `@bufbuild/protoc-gen-es` | Codegen plugin (dev only) |
| `@bufbuild/buf` | buf CLI (dev only) |

For Node.js server-side clients: also add `@connectrpc/connect-node`.

---

## 4. Generated Code Structure

For `user.v1` proto package, buf generates into `src/gen/`:

```
src/gen/
  user/v1/
    user_pb.ts        # messages (UserRequest, UserResponse, etc.)
    user_connect.ts   # UserService definition (client stubs)
```

In v2, `user_pb.ts` contains BOTH message types AND service definitions.
The separate `*_connect.ts` file may or may not be generated depending on config.

---

## 5. Minimal Usage Example

```typescript
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { UserService } from "./gen/user/v1/user_pb";  // v2: service in _pb.ts

const transport = createConnectTransport({
  baseUrl: "https://api.example.com",
});

const client = createClient(UserService, transport);

// Unary RPC
const user = await client.getUser({ id: "123" });
console.log(user.name);

// List (server-streaming or unary)
const { users } = await client.listUsers({ page: 1 });

// Create
const created = await client.createUser({
  name: "Alice",
  email: "alice@example.com",
});

// Server-streaming (if applicable)
for await (const chunk of client.listUsers({ page: 1 })) {
  console.log(chunk);
}
```

**Transport selection:**
- Browser: `createConnectTransport` from `@connectrpc/connect-web`
- Node.js client: `createConnectTransport` from `@connectrpc/connect-node`

---

## 6. Breaking Changes / Version Notes

| Change | Impact |
|---|---|
| Messages = plain objects, not classes | Cannot use `new UserRequest()`, use `{}` literals |
| Single plugin replaces two | Remove `protoc-gen-connect-es` from buf.gen.yaml |
| Node.js 18+ required | Drop Node 16 support |
| TypeScript 4.9.6+ required | Older TS versions unsupported |
| Service defs now in `*_pb.ts` | Import path changes if migrating from v1 |

**Migration tool:** `npx @connectrpc/connect-migrate@latest`

---

## 7. buf.gen.yaml for This Project

Recommended config for `user.v1` / UserService:

```yaml
version: v2
plugins:
  - remote: buf.build/bufbuild/es
    out: src/gen
    opt:
      - target=ts
      - import_extension=none
inputs:
  - directory: proto
```

Run: `npx buf generate`

---

## Sources

- [Connect-ES v2.0 GA announcement](https://buf.build/blog/connect-es-v2)
- [Connect Web: Generating code](https://connectrpc.com/docs/web/generating-code/)
- [Connect Web: Using clients](https://connectrpc.com/docs/web/using-clients/)
- [buf.build/bufbuild/es plugin registry](https://buf.build/bufbuild/es)
- [Buf remote plugins usage](https://buf.build/docs/bsr/remote-plugins/usage/)
- [buf.gen.yaml v2 config docs](https://buf.build/docs/configuration/v2/buf-gen-yaml/)
- [connect-es GitHub](https://github.com/connectrpc/connect-es)

---

## Unresolved Questions

1. Exact `buf.build/bufbuild/es` version number to pin (check registry at https://buf.build/bufbuild/es — versions not scraped successfully).
2. Whether Go backend at v1.19.1 requires any proto compatibility annotations for Connect-ES v2 clients — likely none, wire protocol unchanged.
3. If using gRPC-Web transport vs Connect protocol: default `createConnectTransport` uses Connect protocol; if backend only speaks gRPC-Web, use `createGrpcWebTransport` instead.
