# CLAUDE.md

## Project

`dev` — a fast, lightweight CLI for managing PHP development environments on Linux and macOS. Uses shared Docker containers (PHP-FPM, Caddy, MySQL, Redis, Postgres) so multiple projects share one set of services. Only requires Docker on the host.

## Quick Reference

```bash
# Build
go build ./...

# Run all unit tests
go test ./...

# Run with race detector
go test ./... -race -count=1

# Run fuzz tests (10s each)
go test ./internal/db/ -fuzz=FuzzSanitizeDBName -fuzztime=10s
go test ./internal/config/ -fuzz=FuzzDbNameFromDir -fuzztime=10s

# Run integration tests (requires Docker + mysql-client + postgresql-client)
go test -tags=integration -count=1 ./internal/db/ -timeout 300s

# Vet
go vet ./...
```

## Architecture

```
cmd/                     # Thin cobra command wrappers — no business logic
internal/
  config/                # .dev.yaml + ~/.dev/config.yaml loading
  project/               # Project detection (walks up looking for markers)
  lifecycle/             # Orchestration: Start, Stop, Down (testable via interfaces)
  db/                    # db.Store interface + MySQLStore/PostgresStore adapters
  caddy/                 # Caddy site config generation, reload via docker exec
  docker/                # Dynamic docker-compose.yml generation, Exec for running in containers
  hosts/                 # /etc/hosts management, WSL2 detection
  phpimage/              # PHP Dockerfile generation, extension union, image building
```

**Dependency direction:** `cmd/ → lifecycle → {caddy, docker, db, hosts, project} → config`

**Key interfaces (defined in lifecycle package):**
- `DockerService` — `Up(phpVersions)`, `Down()`, `Exec(service, workdir, args...)`
- `CaddyService` — `Start()`, `Stop()`, `Link(site, docroot, phpService)`, `Unlink(site)`, `Reload()`
- `HostsService` — `Ensure(domain)`
- `db.Store` — `CreateIfNotExists`, `Drop`, `Snapshot`, `Restore`

**Service adapters:** `caddy.Service{}`, `docker.Service{ProjectsDir}`, `hosts.Service{}` wrap package-level functions to satisfy lifecycle interfaces.

## Conventions

### Go Standards

- **Imports:** stdlib, blank line, external, blank line, internal
- **Errors:** always wrap with context (`fmt.Errorf("doing X: %w", err)`), never bare `return nil, err`
- **Ignored errors:** must use `_ = fn()` with a comment explaining why
- **Line length:** soft limit 99 characters
- **Tests:** table-driven for pure functions, subtests for behavior with setup, `t.Parallel()` where safe, `t.Helper()` on all helpers, `t.Cleanup()` over `defer`

### Project Patterns

- **cmd/ must be thin** — all orchestration lives in `internal/lifecycle`
- **Adapter pattern for DB** — `db.NewStore(config)` returns `MySQLStore` or `PostgresStore` based on `db_driver`
- **Service wrappers** — adapter structs wrap package functions to satisfy lifecycle interfaces
- **Config defaults** — `config.Load()` fills all defaults; `config.LoadGlobal()` handles `~/.dev/config.yaml`
- **Dynamic compose** — `docker.generateCompose()` builds docker-compose.yml based on needed PHP versions
- **PHP images** — built from generated Dockerfiles; extensions unioned across projects sharing a version; content-addressed tags prevent unnecessary rebuilds
- **Hooks run in containers** — via `docker.Exec` into the PHP container, not on the host
- **Xdebug** — toggled by writing/removing an ini file in a mounted conf.d directory + PHP-FPM reload signal

### Database Support

- MySQL: `mydumper`/`myloader` for parallel snapshot/restore, falls back to `mysqldump`/`mysql`
- Postgres: `pg_dump -Fd -j4` / `pg_restore -j4` with lz4 compression
- Configured via `db_driver: mysql|postgres` in `.dev.yaml`

### Integration Tests

- Guarded by `//go:build integration` tag
- Use `testcontainers-go` for real MySQL/Postgres containers
- Skip gracefully if CLI tools (`mysql`, `psql`) aren't in PATH
- Never run with `go test ./...` — require explicit `-tags=integration`
