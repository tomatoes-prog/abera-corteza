# Security Audit Notes

SPDX-License-Identifier: Apache-2.0

This repository is licensed under the Apache License 2.0. The security hardening
changes documented here are intended to remain public and auditable under the
same license.

## 2026-07-28 Hardening Pass

The following repository hygiene issues were identified and addressed:

- Removed `extra/server-discovery/server-discovery`, a checked-in Linux ELF
  executable. Binary artifacts should be rebuilt from source instead of
  committed to the repository.
- Removed `extra/server-discovery/discovery.log`, a local runtime log containing
  operational output. Runtime logs should not be committed.
- Added ignore rules for local logs and the `extra/server-discovery` binary.
- Fixed compile-time formatting errors in `extra/server-discovery` so the tool
  can be rebuilt from source after removing the checked-in binary.

The following items were identified but intentionally not changed in this pass:

- Test fixture keys under `server/app/test_files/` and `server/tests/system/`.
  They appear to be test fixtures, but should be reviewed before removal or
  regeneration because tests may depend on their exact contents.
- JavaScript dynamic execution patterns such as `eval` and `new Function`.
  Corteza uses configurable client-side/server-side extension mechanisms, so
  these require design-level review before changing.
- Vue 2 dependencies. The frontend still uses Vue 2.7.16, which is past upstream
  end of life. Migrating to Vue 3 is a larger compatibility project.

Local file-type scanning after the cleanup did not identify additional ELF,
PE, or Mach-O executable binaries outside vendored dependencies. Remaining
notable file types are expected project assets or test fixtures:

- Shell scripts: `client/web/entrypoint.sh`, `server/provision/update.sh`.
- Test import archives under `server/tests/compose/testdata/namespace_import/`.
- Test PEM materials under `server/app/test_files/` and
  `server/tests/system/static/`.

## Go Dependency Remediation

`govulncheck` was executed on 2026-07-28 with dependency metadata access
approved for the public Go vulnerability database:

```sh
cd server
govulncheck ./...
```

Result: the server code is affected by 21 reachable vulnerabilities across 9
modules. The vulnerable modules reported by the tool were:

- `google.golang.org/grpc v1.61.1`, fixed in `v1.82.1`.
- `golang.org/x/text v0.21.0`, fixed in `v0.39.0`.
- `github.com/go-chi/chi/v5 v5.0.7`, fixed in `v5.3.0`.
- `golang.org/x/image v0.18.0`, fixed in `v0.43.0`.
- `golang.org/x/net v0.33.0`, fixed in `v0.55.0`.
- `github.com/russellhaering/goxmldsig v1.4.0`, fixed in `v1.6.0`.
- `github.com/gorilla/csrf v1.7.1`, fixed in `v1.7.3`.
- `github.com/golang-jwt/jwt/v4 v4.5.1`, fixed in `v4.5.2`.
- `github.com/golang-jwt/jwt v3.2.2+incompatible`, no fixed version
  reported by `govulncheck`.

The scan also reported an indirect vulnerable path through
`github.com/766b/chi-prometheus`, which imports the legacy `github.com/go-chi/chi`
module. That dependency should be replaced or removed because `govulncheck`
reported no fixed legacy `github.com/go-chi/chi` version.

The Go dependency update pass was completed on branch `dev-security-go-deps`.
The affected modules with fixed versions were updated, `server/vendor/` was
regenerated, and the two remaining legacy dependencies without direct fixed
versions were removed from the reachable graph:

- Replaced `github.com/766b/chi-prometheus` with a local Prometheus middleware
  implemented against `github.com/go-chi/chi/v5`.
- Replaced the external `github.com/go-chi/jwtauth` dependency with a small
  internal compatibility package under `server/pkg/auth/jwtauth` that uses the
  repository's existing `github.com/lestrrat-go/jwx` v1 types.
- Updated `github.com/go-oauth2/oauth2/v4` to `v4.5.4`, which moves its JWT
  generator off `github.com/golang-jwt/jwt` v3.

After remediation, `govulncheck ./...` reported:

```text
No vulnerabilities found.
Your code is affected by 0 vulnerabilities.
```

The update raises the server module's Go directive from `1.24.1` to `1.25.0`
because current fixed versions of `golang.org/x/image`, `golang.org/x/net`,
`golang.org/x/text`, and `google.golang.org/grpc` require Go 1.25.

## Node Dependency Follow-up

For Node dependencies, regenerate lockfiles and run the package manager audit
for each package after confirming the supported Yarn version.

Observed Node dependency issues:

- The main Vue 2 web applications pin `vue` and `vue-template-compiler` to
  `2.7.16`. Vue 2 reached upstream end of life on 2023-12-31.
- Most JavaScript lockfiles resolve `axios@^1.8.3` to `1.8.4`, which should be
  reviewed against current Axios advisories.
- `server/webconsole/package.json` requests `axios@^1.8.3`, but
  `server/webconsole/yarn.lock` still resolves `axios@^0.28.0` to `0.28.0`.

An external Yarn audit was not completed in this pass because it requires
explicit approval to send Node lockfile dependency metadata to npm/Yarn advisory
services.

## Verification

The following commands were run after the cleanup and Go dependency remediation:

```sh
cd server
GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache go mod tidy
GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache go mod vendor
GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache GOBIN=/tmp/gobin \
  go run golang.org/x/vuln/cmd/govulncheck@latest ./...
GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache go build -mod=vendor \
  -o /tmp/corteza-server ./cmd/corteza
GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache \
  LOCALE_PATH=/mnt/d/Repositorios/abera/abera-corteza/locale \
  ENVIRONMENT=dev LOCALE_DEVELOPMENT_MODE=true \
  go test -mod=vendor ./...

cd ../extra/server-discovery
GOCACHE=/tmp/gocache go test ./...
```

Full `go test -mod=vendor ./...` was executed with an absolute `LOCALE_PATH`
because Go runs each package test from that package directory. The suite passed
the main server packages and integration packages including `tests/apigw`,
`tests/automation`, `tests/compose`, `tests/dal`, `tests/federation`,
`tests/messagebus`, and `tests/system`.

The only remaining full-suite failure observed locally is in
`server/tests/workflows`, where iterator workflow tests can return an empty
stacktrace when the whole package is run together. The affected tests pass when
run in isolation or in their iterator group, which points to test isolation or
workflow-service cache state rather than a dependency compile/runtime failure.
This should be tracked separately before treating the workflow package as a
fully reliable regression gate.

A local smoke test was run with SQLite, development locales, and webapps
disabled:

```sh
ENVIRONMENT=dev LOCALE_DEVELOPMENT_MODE=true \
DB_DSN='sqlite3://file:/tmp/corteza-smoke.db?cache=shared&mode=rwc' \
HTTP_ADDR=127.0.0.1:18080 HTTP_WEBAPP_ENABLED=false \
/tmp/corteza-server serve-api

curl --fail http://127.0.0.1:18080/healthcheck
```

The healthcheck returned `PASS` for scheduler, mail, Corredor, primary store,
and object stores.

The full local frontend build was also executed with Yarn 1.22.22 after linking
the local `@cortezaproject/corteza-js` and `@cortezaproject/corteza-vue`
packages. The following packages built successfully:

- `lib/js`
- `lib/vue`
- `client/web/one`
- `client/web/admin`
- `client/web/compose`
- `client/web/workflow`
- `client/web/reporter`
- `client/web/discovery`
- `client/web/privacy`

A full local smoke test was then run with the compiled webapps enabled on
`127.0.0.1:18083`. The port and `AUTH_BASE_URL` value were runtime-only test
configuration, not committed source configuration. The smoke test validated:

- `/healthcheck`
- `/`
- `/admin/`
- `/compose/`
- `/workflow/`
- `/reporter/`
- `/discovery/`
- `/privacy/`
- OAuth2 authorization redirect generation with
  `AUTH_BASE_URL=http://127.0.0.1:18083/auth`

Docker backend compilation was validated with the checked-out source and
vendored dependencies:

```sh
docker run --rm \
  -v /mnt/d/repositorios/abera/abera-corteza:/src \
  -w /src/server \
  -e GOMODCACHE=/tmp/gomodcache \
  -e GOCACHE=/tmp/gocache \
  golang:1.25.0 \
  go build -buildvcs=false -mod=vendor -o /tmp/corteza-server ./cmd/corteza
```

The first Docker backend build without `-buildvcs=false` failed only while
obtaining Git VCS metadata from the bind-mounted checkout. Disabling VCS
stamping allowed the same source compilation to pass.

Frontend Docker compilation was attempted with the local
`abera-development:dev` image. That image provides Node 24.14.1, Yarn 1.22.22,
TypeScript 5.8.2, and `rollup-plugin-typescript2` 0.36.0. `lib/js` compiled,
but `lib/vue` failed inside Docker with:

```text
TS2307: Cannot find module '@cortezaproject/corteza-js' or its corresponding
type declarations.
```

The same `lib/vue` build passes locally with Node 24.18.0, Yarn 1.22.22,
TypeScript 5.8.2, and `rollup-plugin-typescript2` 0.36.0. A separate TypeScript
resolution trace inside Docker successfully resolved
`@cortezaproject/corteza-js` to `lib/js/dist/index.d.ts`, so the remaining
Docker frontend issue appears specific to the Rollup/TypeScript plugin behavior
in that container/mount setup and should be tracked separately from the Go
dependency remediation.
