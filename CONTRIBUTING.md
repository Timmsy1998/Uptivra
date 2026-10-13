# Contributing

Start with the [architecture](docs/architecture.md), [security model](docs/security.md) and [native development setup](docs/deployment.md#native-development). Small, complete changes are easier to review than speculative abstractions. For a substantial feature, open an issue describing the operator problem and expected behavior first.

Use Go 1.27.2, Node 24.11 and PostgreSQL 16. Keep dependencies pinned, explain additions, and never commit credentials, environment files, database dumps, build artifacts or local agent instruction files. Contributions are licensed under Apache-2.0.

## Verification

```sh
# Disposable database; your role must be able to create schemas.
createdb uptivra_test
export TEST_DATABASE_URL='postgres:///uptivra_test?host=/var/run/postgresql'
gofmt -w api cmd internal examples/go
go mod verify
go vet ./...
go test -race -count=1 ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
npm --prefix web ci
npm --prefix web run format:check
npm --prefix web run typecheck
npm --prefix web run build
npm --prefix web audit --audit-level=moderate
```

Adapt the database URL to your installation. Tests create isolated schemas and remove them afterward; they skip database coverage when the variable is absent. CI supplies PostgreSQL and always runs them. Network unit tests inject controlled transports; browser workflows also make real public requests.

## Browser workflows

Use a separate disposable database and **never a real installation**: the browser tests create/delete monitors, channels and keys and edit global settings. Test fixtures use example.com and explicit test-only credentials.

```sh
createdb uptivra_e2e
export DATABASE_URL='postgres:///uptivra_e2e?host=/var/run/postgresql'
export APP_ORIGIN=http://localhost:3000 API_ORIGIN=http://localhost:8080
export ENCRYPTION_KEY="$(openssl rand -base64 32)"
export CRON_SECRET="$(openssl rand -hex 32)"
export ADMIN_EMAIL=browser@example.com
export ADMIN_PASSWORD=browser-test-password-only
go build -o bin/uptivra ./cmd/uptivra
./bin/uptivra migrate
./bin/uptivra bootstrap
npm --prefix web ci
(cd web && npx playwright install --with-deps chromium)
```

Keep these variables available in each terminal. Run `./bin/uptivra serve` and either the development frontend or the [production standalone frontend](docs/deployment.md#native-development). Do not run a separate scheduler: the workflow invokes authenticated cron directly. Then run:

```sh
npm --prefix web run test:e2e
```

Coverage includes login, SSRF rejection, monitor CRUD/pause, real checks, history timestamps, channels, settings, API scope/revocation, public redaction, failed refreshes, mobile overflow, dark mode and oversized proxy bodies. Tests require working public DNS/HTTPS. Failed workflows retain Playwright traces locally; traces can contain test configuration and should be treated as sensitive when sharing.

To deliberately refresh the README's actual application screenshots, use `UPDATE_SCREENSHOTS=1 npm --prefix web run test:e2e`, inspect all generated images in `docs/screenshots`, and commit only the intentional visual update. Never substitute mockups or invented measurements.

## Review expectations

Keep domain rules in Go, SQL parameterized and public responses explicitly projected. Network changes need SSRF/redirect/timeout regression coverage. Persistence changes need transactional PostgreSQL tests for concurrent/duplicate/interrupted work. UI changes need accessible states, keyboard interactions and responsive light/dark review. Add tests for meaningful behavior rather than mirroring implementation details.

Preserve v1 wire compatibility, update `docs/openapi.json` alongside API changes, and add numbered migrations for schema changes. The contract test checks registered routes and core response fields; it does not replace consumer compatibility review. The Laravel example is a reviewed integration snippet, not a fully executed Laravel test project.

Use cohesive commits such as `fix(scheduler): preserve minute cadence`. PRs should explain the concrete problem, resulting behavior, relevant validation and material limitations. Complete the checklist in the PR template. Follow the [code of conduct](CODE_OF_CONDUCT.md) and send vulnerabilities through [private reporting](SECURITY.md).
