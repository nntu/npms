# Dependency review

Dependency versions are pinned exactly in `frontend/package.json` and
`frontend/package-lock.json` so Linux and Windows installs resolve the same
frontend toolchain.

## Frontend

| Package | Version | Decision |
| --- | ---: | --- |
| React / React DOM | 19.3.0 | Current stable major used by the app |
| TanStack Query | 5.104.1 | Current v5 line |
| Vite | 8.3.4 | Current npm resolution |
| `@vitejs/plugin-react` | 6.1.2 | Compatible with Vite 8 |
| ESLint | 10.12.0 | Current npm resolution |
| TypeScript | 5.9.3 | Latest compatible with `typescript-eslint` 8.71.1 |

Vite 8 and ESLint 10 require Node.js `^20.19.0`, `^22.13.0` or a newer
supported release. Use Node.js 20.19+ LTS on both Linux and Windows. The
lockfile is committed; use `npm ci` for reproducible installation.

Verified locally:

```text
npm run typecheck  PASS
npm run lint       PASS
npm run build      PASS
npm audit          blocked by registry DNS in the current environment
```

## Backend

The backend currently requests GoSNMP `v1.45.0`, `modernc.org/sqlite v1.60.1` and
`gopkg.in/yaml.v3 v3.0.1`. YAML has no newer stable v3 release. GoSNMP and the
SQLite driver were advanced in `go.mod`, but completing the upgrade requires a
working Go module proxy to regenerate `go.sum` and resolve the matching
`modernc.org/libc` dependency set. The current environment cannot resolve
`proxy.golang.org`; therefore the repository currently has a known dependency
lockfile gap for clean environments. The local backend test suite passes when
the required module artifacts are already cached.

Before the next backend release, run from `backend/` with network access:

```bash
go get github.com/gosnmp/gosnmp@latest modernc.org/sqlite@latest
go mod tidy
go test ./...
go vet ./...
```

Review the resulting `go.mod` and `go.sum` on a clean Linux and Windows build.
The SQLite driver remains pure Go; no Docker, PostgreSQL service or system
SQLite library is required at runtime.

## HTTP server decision

NPMS uses Go's standard `net/http` server and `http.NewServeMux`. Since Go 1.22,
the standard mux supports method-aware and wildcard route patterns while
remaining dependency-free. It also provides the `http.Server`, HTTP/2/TLS
support, `embed` integration and `http.FileServer` used by the single-binary
deployment.

If the route surface becomes large, the preferred compatible addition is
`github.com/go-chi/chi/v5`: it remains a small `net/http` router and its
middleware can be composed with existing handlers. We do not add it yet because
the current API is small and the standard mux keeps the Linux/Windows binary
and module graph smaller.

`fasthttp` is not selected despite its benchmark focus because its API is not
identical to `net/http` and it does not provide the same HTTP/2 compatibility
path. Gin is also unnecessary for this small standalone service and brings a
larger framework surface. Benchmark changes are not a reason to replace the
server unless NPMS has measured a real API bottleneck.
