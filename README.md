# dev

A fast, lightweight PHP development environment for Linux and macOS. Uses shared Docker containers (PHP-FPM, Caddy, MySQL, Redis) to keep RAM usage minimal across many projects. Only requires Docker on the host.

## Why

When working on multiple branches simultaneously via git worktrees, traditional container-per-project setups eat RAM fast (~6 GB each). `dev` takes a different approach:

- **One PHP-FPM container per version** — shared across all projects (~150 MB each)
- **One Caddy container** — reverse proxy for all `*.test` domains with automatic local HTTPS (~15 MB)
- **Shared database/cache containers** — one MySQL, Redis, Typesense instance instead of one per project

**RAM comparison for 5 projects:**

| | Container-per-project | dev |
|---|---|---|
| Web/PHP | 5 x ~4 GB | ~300 MB total |
| MySQL | 5 x ~500 MB | ~500 MB total |
| Redis | 5 x ~10 MB | ~10 MB total |
| **Total** | **~25 GB** | **~1.5 GB** |

## Requirements

| | Linux/WSL2 | macOS |
|---|---|---|
| **Docker** | [Docker Engine](https://docs.docker.com/engine/install/) | [OrbStack](https://orbstack.dev/) (recommended) or [Docker Desktop](https://www.docker.com/products/docker-desktop/) |
| **Go** | 1.25+ (for building from source) | 1.25+ |

> **OrbStack** is recommended on macOS for significantly better performance and lower resource usage compared to Docker Desktop.

## Install

```bash
git clone https://github.com/XBS-Nathan/apex-flow-dev-cli.git
cd dev-cli
go build -o dev .
./dev trust   # trust the Caddy local CA certificate
```

That's it. First `dev start` builds the PHP images automatically.

### WSL2

`dev trust` automatically detects WSL2 and:
- Installs the Caddy CA cert in both Linux and Windows trust stores
- `dev start` adds hosts entries to both `/etc/hosts` and the Windows hosts file

## Usage

```bash
cd /path/to/your/project
dev start
```

### Commands

| Command | Description |
|---------|-------------|
| `dev start` | Start the current project (builds images, starts containers, links Caddy, creates DB) |
| `dev stop` | Stop the current project (unlinks from Caddy, containers stay running) |
| `dev restart` | Stop + start |
| `dev down` | Stop all containers |
| `dev artisan [args]` | Run `php artisan` inside the PHP container (Laravel) |
| `dev composer [args]` | Run `composer` inside the PHP container |
| `dev exec [command...]` | Run any command in the project's PHP container |
| `dev snapshot [name]` | Create a database snapshot |
| `dev snapshot restore [name]` | Restore from a snapshot (latest if no name given) |
| `dev snapshot list` | List available snapshots |
| `dev info` | Show project URL, PHP version, DB, service status |
| `dev xdebug on/off` | Toggle Xdebug (sub-second, no container restart) |
| `dev share` | Share via Cloudflare Tunnel or ngrok |
| `dev use php <version>` | Set the PHP version for this project |
| `dev use node <version>` | Set the Node version for this project |
| `dev use db <mysql\|postgres>` | Set the database driver |
| `dev trust` | Trust the Caddy local CA certificate |
| `dev build` | Force rebuild PHP images |
| `dev services up` | Start shared Docker services |
| `dev services down` | Stop shared Docker services |

### Shell completions

```bash
# Bash
dev completion bash > ~/.local/share/bash-completion/completions/dev

# Zsh
dev completion zsh > ~/.zsh/completions/_dev

# Fish
dev completion fish > ~/.config/fish/completions/dev.fish
```

## Configuration

### Global: `~/.dev/config.yaml`

```yaml
# Parent directory mounted into containers (default: ~/Projects)
projects_dir: ~/Projects

# PHP versions to keep available
php_versions:
  - "8.2"
  - "8.3"

# Service image versions (default: latest)
versions:
  mysql: "latest"
  redis: "latest"
  typesense: "latest"
  postgres: "latest"
  mailpit: "latest"
```

### Per-project: `.dev.yaml`

Drop a `.dev.yaml` in your project root to override defaults. Everything is optional.

```yaml
# PHP version (default: 8.2)
php: "8.1"

# Node version (default: 22)
node: "22"

# Database driver: mysql or postgres (default: mysql)
db_driver: mysql

# Database name (default: derived from directory name)
db: my_project

# PHP extensions to install (added to the shared PHP image)
extensions:
  - imagick
  - swoole

# MySQL connection (defaults shown)
mysql:
  user: root
  pass: root
  host: 127.0.0.1
  port: "3306"

# PostgreSQL connection (defaults shown)
postgres:
  user: postgres
  pass: postgres
  host: 127.0.0.1
  port: "5432"

# Hooks run inside the PHP container after start/stop
hooks:
  post-start:
    - "php artisan horizon &"
    - "yarn run hot &"
  post-stop: []
```

### Directory structure

```
~/.dev/
├── caddy/
│   ├── Caddyfile              # Main config (auto-generated)
│   ├── data/                  # Caddy CA certificates
│   └── sites/                 # Per-project site configs
├── dockerfiles/
│   └── php/
│       └── 8.2/
│           ├── Dockerfile     # Generated from extensions
│           └── php.ini
├── php/
│   └── 8.2/
│       └── conf.d/
│           └── xdebug.ini     # Written by dev xdebug on
├── docker-compose.yml         # Generated dynamically
├── config.yaml                # Global config
└── snapshots/                 # Database snapshots
```

## How it works

```
Browser → project.test → /etc/hosts → 127.0.0.1
                                          ↓
                                    Caddy (Docker, ports 80/443)
                                          ↓
                                    PHP-FPM (Docker, per-version)
                                          ↓
                                MySQL / Redis / Typesense (Docker)
```

1. **DNS**: `dev start` adds `127.0.0.1 project.test` to `/etc/hosts` (+ Windows hosts on WSL2)
2. **Caddy**: Routes each `*.test` domain to the correct PHP-FPM container over the Docker network. Automatic local HTTPS via built-in CA.
3. **PHP-FPM**: One container per PHP version, shared across all projects. Extensions configurable per project (unioned into the shared image). Xdebug toggled via mounted ini file + FPM reload signal.
4. **Docker Compose**: Generated dynamically based on which PHP versions and services are needed.

## Database Snapshots

Snapshots use parallel tools for speed when available:

| Driver | Snapshot | Restore | Fallback |
|--------|----------|---------|----------|
| MySQL | `mydumper` (4 threads) | `myloader` (4 threads) | `mysqldump`/`mysql` + gzip |
| Postgres | `pg_dump -Fd -j4` (lz4) | `pg_restore -j4` | -- (built-in) |

You can also drop `.sql` or `.sql.gz` files into `~/.dev/snapshots/<db_name>/` and restore them with `dev snapshot restore <filename>`.

## Building from source

```bash
# Requires Go 1.25+
go build -o dev .
```

### Run tests

```bash
go test ./...
```

## License

MIT
