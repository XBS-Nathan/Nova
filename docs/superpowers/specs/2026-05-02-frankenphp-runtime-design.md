# FrankenPHP Runtime Option — Design

**Date:** 2026-05-02
**Status:** Approved, ready for implementation planning
**Motivation:** One project is moving to Laravel Octane. We want to opt that project (and future projects) into a FrankenPHP-backed runtime with worker mode, while leaving every other project on the existing shared PHP-FPM model untouched.

## Goals

- Per-project opt-in to FrankenPHP via `.nova.yaml` (`runtime: frankenphp`).
- Independent toggle for Laravel Octane worker mode (`octane: true`).
- Reuse the existing shared Caddy as the only public TLS endpoint.
- Reuse the existing extension-installation logic so FrankenPHP and FPM images stay in lockstep on what's available.
- Zero behavior change for projects that don't opt in.

## Non-goals

- Replacing the shared Caddy with FrankenPHP as the front door.
- Production deployment patterns (HTTP/3 to client, end-to-end early hints).
- Watching files / hot reload beyond what Octane already does.
- Integration tests that boot FrankenPHP in CI.
- A knob for Octane worker count (use Octane's `auto` default until someone asks).

## Architecture

```
              ┌────────────────────┐
   :443 ──►   │   Shared Caddy     │  TLS, hosts routing — unchanged
              └──────────┬─────────┘
                         │
        ┌────────────────┼─────────────────┐
        │                │                 │
        ▼                ▼                 ▼
  php_fastcgi      php_fastcgi       reverse_proxy
   php82:9000       php83:9000      myapp_frankenphp:8000
        │                │                 │
        ▼                ▼                 ▼
 ┌───────────┐    ┌───────────┐    ┌──────────────────────┐
 │  php82    │    │  php83    │    │  myapp_frankenphp    │
 │  (FPM,    │    │  (FPM,    │    │  (FrankenPHP, Octane │
 │  shared)  │    │  shared)  │    │  worker, per-project)│
 └───────────┘    └───────────┘    └──────────────────────┘
```

- Shared Caddy stays the single TLS endpoint. No changes to ports, hosts file, certificates.
- The current project's PHP runtime container goes up when you `nova start`/`nova use`. Today this is one FPM container; with FrankenPHP it's one FrankenPHP container instead. The compose file is rewritten on each invocation; `--remove-orphans` removes the previously active PHP runtime if the active project's PHP version or runtime changed (existing behavior).
- For `runtime: fpm` (the default), the active container is `phpXX` (FPM) — exactly as today.
- For `runtime: frankenphp`, the active container is `<sanitized_project>_frankenphp`. Listens on `:8000` inside the `nova` Docker network. No host port mapping. Worker mode binds to one app, hence the per-project name.
- Per-project Caddy site config emits **either** `php_fastcgi phpXX:9000` (fpm) **or** `reverse_proxy <project>_frankenphp:8000` (frankenphp). Mutually exclusive per project.
- The `nova-frankenphp:<version>-<hash>` image is content-addressed by Dockerfile contents, so two projects on the same PHP version + extensions reuse the same image whenever each becomes active. That's the same caching benefit the FPM image already gets.

### Why FrankenPHP behind shared Caddy (not fronted directly)

The hop is shared Caddy → FrankenPHP across the `nova` bridge — sub-millisecond per request and dwarfed by Octane's framework-boot savings (~30–50 ms → ~1–5 ms). Going direct would force two TLS systems, port juggling, and changes to how hosts/ports are wired. The current shape is a smaller change for the same end-user win.

## Configuration schema

Per-project `.nova.yaml` adds two fields:

```yaml
php: "8.3"
runtime: frankenphp   # default: "fpm"
octane: true          # default: false; only meaningful when runtime=frankenphp
extensions: [gd, zip, intl, exif]
```

In `internal/config/config.go`:

```go
type ProjectConfig struct {
    // ...existing...
    Runtime string `yaml:"runtime"`   // "fpm" | "frankenphp"
    Octane  bool   `yaml:"octane"`
}

const (
    RuntimeFPM        = "fpm"
    RuntimeFrankenPHP = "frankenphp"
)
```

Defaults filled in `fillDefaults`:

- `Runtime == ""` → `RuntimeFPM`. This preserves all existing project behavior.
- `Octane && Runtime != RuntimeFrankenPHP` → validation error at `Load` time: *"octane: true requires runtime: frankenphp"*. Loud failure beats silent surprise.

No changes to `~/.nova/config.yaml`.

## Image building (`internal/phpimage`)

`ImageConfig` gains a `Runtime` field; the rest of the package branches on it.

```go
type ImageConfig struct {
    PHPVersion string
    Extensions []string
    Runtime    string  // RuntimeFPM | RuntimeFrankenPHP
}
```

### Tag scheme

`nova-<runtime>:<phpVersion>-<hash>`

- `nova-fpm:8.3-abc123` (renamed from current `nova-php`; tags are content-addressed, so old images garbage-collect on next build).
- `nova-frankenphp:8.3-def456`.

### Dockerfile generation

Branches at the top, but the extension/build-dep computation, conf.d directory, php.ini, and my.cnf logic stay shared (factored into helpers used by both branches).

| | FPM | FrankenPHP |
|---|---|---|
| Base | `php:8.3-fpm-alpine` | `dunglas/frankenphp:1-php8.3-alpine` |
| Extension install | `docker-php-ext-install` + `pecl` (unchanged) | identical (FrankenPHP image ships `docker-php-ext-*` helpers) |
| `php-fpm.d/www.conf` UID strip | yes | n/a |
| Caddyfile copy | n/a | `COPY Caddyfile /etc/caddy/Caddyfile` |
| `WORKDIR` | `/srv` | `/srv` |
| Entrypoint | image default (`php-fpm`) | image default (`frankenphp run`); Octane overrides via compose `command:` |

The existing hash function (`sha256` over the full Dockerfile content) keeps cache invalidation correct for either runtime — different runtimes produce different hashes naturally.

### Bundled Caddyfile (FrankenPHP only)

Written next to the Dockerfile and `COPY`'d into the image:

```
{
    frankenphp
    auto_https off
    admin off
}
:8000 {
    root * /srv/{$NOVA_APP}/public
    php_server
}
```

- `auto_https off` — TLS terminates at the shared Caddy.
- `{$NOVA_APP}` is set per-container in compose (the sanitized project name) so the same image serves any project on the same PHP version.
- `php_server` is FrankenPHP's high-level directive that handles classic-mode requests correctly.
- For Octane, this file is effectively unused: the compose `command:` runs `php artisan octane:start --server=frankenphp`, which manages its own listen socket.

## Compose generation (`internal/docker`)

The current model: each `nova start` / `nova use` constructs a `[]docker.PHPVersion` for the *active* project only and passes it to `Service.Up`. Shared services (MySQL, Postgres, Redis, Mailpit, project shared services) are unioned across all projects via `config.CollectVersions`. PHP is not unioned today.

For FrankenPHP we add a parallel optional field for the active project (when `runtime: frankenphp`). The two fields are mutually exclusive per call.

```go
type FrankenPHPProject struct {
    Name        string   // sanitized project name; service name and NOVA_APP
    PHPVersion  string
    Extensions  []string
    Octane      bool
    Workdir     string  // "/srv/<name>"
    Ports       []string // mirrors PHPVersion.Ports for parity
}

type ComposeOptions struct {
    // ...existing...
    PHP        []PHPVersion        // empty when active project is frankenphp
    FrankenPHP []FrankenPHPProject // empty when active project is fpm; today, length 0 or 1
}
```

`Service.Up`'s signature changes to take both slices (or, more cleanly, a small struct that names them) so callers in `cmd/` can pass the right one based on `cfg.Runtime`. The `lifecycle.DockerService.Up` interface signature changes to match; the existing `mockDocker` in `internal/lifecycle/lifecycle_test.go` is updated accordingly.

### Image build path

`cmd/start.go`, `cmd/use.go`, `cmd/restart.go`, `cmd/slow.go`, `cmd/services.go` each call `phpimage.EnsureBuilt`. Each call site that today builds a single FPM `ImageConfig` learns to set `Runtime: cfg.Runtime` on the config. The cache key (the content hash) ensures that switching back to a previously-built combination is instant.

We don't add cross-project extension unioning in this design — that would be a separate change applicable to FPM too.

### Service emitted for the active FrankenPHP project

```yaml
myapp_frankenphp:
  image: nova-frankenphp:8.3-<hash>
  pull_policy: never
  user: "1000:1000"
  restart: unless-stopped
  working_dir: /srv/myapp
  environment:
    NOVA: "true"
    NOVA_APP: "myapp"
  command: ["php", "artisan", "octane:start",
           "--server=frankenphp", "--host=0.0.0.0", "--port=8000",
           "--workers=auto", "--max-requests=500"]
  # Classic mode (octane: false) omits `command:` and uses image default.
  volumes:
    - <projectsDir>:/srv
    - <globalDir>/php/8.3/conf.d:/usr/local/etc/php/conf.custom
  networks: [nova]
  # throttle (deploy.resources.limits) applied identically to FPM
```

Notes:

- Same `<projectsDir>:/srv` mount as FPM, so `nova logs`, `nova php`, hooks, and workers can all `docker compose exec` into the project's FrankenPHP container with `-w /srv/<project>`.
- Same `conf.d` mount, so `nova xdebug` toggles work identically (the FrankenPHP image inherits `PHP_INI_SCAN_DIR=/usr/local/etc/php/conf.d:/usr/local/etc/php/conf.custom`).
- Port 8000 is internal only (no `ports:`). Only the shared Caddy reaches it via the `nova` network. No host-port juggling.
- `--workers=auto` lets FrankenPHP/Octane pick a sensible default; no knob until someone asks.

## Caddy integration (`internal/caddy`)

The site config generator picks `php_fastcgi` or `reverse_proxy` based on the project's runtime.

`Link` signature change — instead of a string `phpService`, take a small union value so the call site is explicit about which mode it's in:

```go
type Upstream struct {
    Kind    string  // "fastcgi" | "reverse_proxy"
    Address string  // "php83:9000" or "myapp_frankenphp:8000"
}

func Link(siteName, docroot string, upstream Upstream, portProxies []PortProxy) error
```

The `lifecycle.CaddyService.Link` interface signature changes to match; the mock in `internal/lifecycle/lifecycle_test.go` is updated.

### Generated site configs

FrankenPHP variant:

```caddy
myapp.test {
    reverse_proxy myapp_frankenphp:8000
}
```

No `root *`, no `file_server` — FrankenPHP serves static assets and front-controller routing itself. Including them would cause double resolution.

FPM variant (unchanged from today):

```caddy
myapp.test {
    root * <docroot>
    php_fastcgi php83:9000
    file_server
    encode gzip
}
```

Extra port proxies (`PortProxy{Port, Backend}`) keep working unchanged.

### Caller (`internal/lifecycle`)

Lifecycle is the only place that calls `Link`. It builds the `Upstream` from the project config:

```go
upstream := caddy.Upstream{Kind: "fastcgi", Address: phpSvc + ":9000"}
if cfg.Runtime == config.RuntimeFrankenPHP {
    upstream = caddy.Upstream{Kind: "reverse_proxy", Address: project + "_frankenphp:8000"}
}
```

## Auxiliary commands, hooks, and workers

Anything that today does `docker compose exec phpXX -w /srv/<project> ...` needs to know which container to target for a FrankenPHP project. One helper in `lifecycle`:

```go
// PHPContainer returns the compose service name to exec into for a project.
func PHPContainer(cfg *config.ProjectConfig, project string) string {
    if cfg.Runtime == config.RuntimeFrankenPHP {
        return project + "_frankenphp"
    }
    return docker.PHPServiceName(cfg.PHP) // existing "php83" etc.
}
```

Touch list:

| Command | Change |
|---|---|
| `nova php` / `nova artisan` / `nova composer` / `nova npm` / `nova yarn` / `nova pnpm` | Target `PHPContainer(cfg)` instead of hardcoding `phpXX`. |
| `nova logs` (no arg / `nova logs <project>`) | Already matches by prefix; works as-is once the FrankenPHP service exists. |
| `nova logs horizon` (worker prefix match) | Workers run inside the project's container — see below. |
| Hooks (`post-start`, `post-stop`) | Run via `docker.Exec(PHPContainer(cfg), ...)`. |
| Workers (`workers:` map) | Same. |
| `nova xdebug` toggle | Writes to the per-version `conf.d` (both FPM and FrankenPHP containers mount it). Reload differs: FPM = `kill -USR2`; FrankenPHP-classic = container restart; FrankenPHP-Octane = `php artisan octane:reload`. Picked by runtime. |
| `nova snapshot` / `nova share` | Untouched — they don't exec PHP. |

### Workers and Octane interaction

Octane *is* the web worker. Other workers (Horizon, queue listeners, schedulers) still run as separate processes via `docker exec` into the FrankenPHP container — they're CLI processes that share the same image. No change to the workers config schema.

### Octane reload caveat

Octane reloads on file changes via `php artisan octane:reload`, but config changes (e.g. `.env`) need a container restart. Out of scope for this design; users `nova restart` as today.

## Testing

### Unit tests

| Package | Tests |
|---|---|
| `internal/config` | `Load` parses `runtime`/`octane`; defaults to `fpm`; rejects `octane: true` with `runtime != frankenphp`. |
| `internal/phpimage` | `generateDockerfile` produces correct base image / install steps / Caddyfile copy per runtime; `ImageTag` namespaces by runtime; identical extension lists across runtimes produce different hashes. |
| `internal/docker` | `generateCompose` emits FrankenPHP service with correct `command:` for Octane on/off, correct `working_dir`, no host port mapping, reuses image tag. Existing FPM compose output unchanged when no FrankenPHP projects exist (golden test). |
| `internal/caddy` | `generateSiteConfig` emits `reverse_proxy` block (no `root`/`file_server`) for `Upstream{Kind: "reverse_proxy"}` and the existing FPM block for `Kind: "fastcgi"`. |
| `internal/lifecycle` | `PHPContainer` returns the right service name per runtime. `Start` plumbs `Upstream` into `Link` correctly (mock `CaddyService` captures the argument). |

### Integration tests

None added in this round. The existing integration suite covers DB only; spinning up FrankenPHP + Octane in CI is a much bigger lift and isn't needed to verify the wiring.

### Manual verification checklist

1. `runtime: fpm` project (default) — site loads as today.
2. `runtime: frankenphp, octane: false` project — site loads, classic-mode FrankenPHP serves it.
3. `runtime: frankenphp, octane: true` project — site loads, repeated requests show no Laravel boot in logs.
4. `nova xdebug on/off` works in all three modes (xdebug attaches in FPM and Octane).
5. `nova logs <project>` follows the right container.
6. Switching the active project between two FrankenPHP projects on the same PHP version + extensions reuses the cached image (no rebuild).
7. Switching the active project between an FPM project and a FrankenPHP project on the same PHP version works correctly: the previous PHP runtime container is removed by `--remove-orphans`, the other's Caddy site config remains linked but its upstream is unreachable until that project is active again (matches today's behavior for FPM-only switches between PHP versions).

## Deferred

- Octane worker count knob.
- File-watch / `.env` auto-reload.
- HTTP/3 / early hints end-to-end (would require fronting FrankenPHP directly).
- Integration tests for FrankenPHP + Octane.
