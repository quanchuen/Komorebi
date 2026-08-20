# Contributing to Komorebi

Thanks for your interest. A few things to know before you open a pull request.

## Contributor License Agreement

Komorebi requires every contributor to agree to the
[Contributor License Agreement](CLA.md). It lets the project keep its open-core
structure: you retain copyright in your contribution, and you grant the
maintainer the rights needed to distribute it under the project's licenses
(Apache-2.0 for the engine, AGPL-3.0 for the app) and under future commercial
terms.

How to sign: in your first pull request, add a line to `CONTRIBUTORS` with your
name and the date, and state in the PR description:

> I have read and agree to the Contributor License Agreement.

(If a CLA bot is configured later, it will replace this manual step.)

## Which license covers your change

See [`LICENSING.md`](LICENSING.md). In short: changes under `internal/domain/`,
`internal/app/`, `internal/infra/`, `pipelines/`, and `migrations/` are
Apache-2.0; changes under `cmd/`, `internal/api/`, and `web/` are AGPL-3.0.
**Do not** add imports from `cmd/` or `internal/api/` into the engine packages —
that boundary is what keeps the engine permissively licensed.

## Development

See [`README.md`](README.md) for bootstrap and [`CLAUDE.md`](CLAUDE.md) for
architecture and conventions.

```bash
make test                     # Fast tests without external services
make test-web                 # Frontend checks and production build
TEST_DB_DSN="$DATABASE_URL" make test-integration
go build ./cmd/api && (cd web && npm run build)
```

Use `make support-up` for Martin and Valhalla, `make project-up` for the
repository-owned containers, or `make stack-up` for both Compose profiles.

Follow the existing patterns: hexagonal layering, hand-written test stubs (no
mocking library), table-driven tests, TDD for domain logic.

## Secrets

Run `make hooks` once after cloning. It installs
[gitleaks](https://github.com/gitleaks/gitleaks) (pinned, built from source and
verified against `sum.golang.org`) and points `core.hooksPath` at
`web/.husky`, whose `pre-commit` hook scans every staged change and refuses the
commit if it finds a credential. Real keys belong in `.envrc` or `*.env`
(gitignored), never in the tree. False positives get a narrowly scoped entry in
`.gitleaks.toml`; real leaks get rotated, not allowlisted. `make secrets-audit`
scans the whole history.

## Pull requests

- One logical change per PR.
- Tests for new behavior.
- `go vet ./...` and `gofmt` clean.
- Reference the design spec (`docs/specs/`) when changing modeled behavior.
