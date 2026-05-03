# FrankenPHP Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a per-project `runtime: frankenphp` option (with optional `octane: true`) to nova, fronted by the existing shared Caddy. Default behavior (FPM) is unchanged.

**Architecture:** Extend `internal/phpimage` with a `Runtime` field; emit one per-project FrankenPHP service in `internal/docker` when the active project opts in; teach `internal/caddy` to emit `reverse_proxy` instead of `php_fastcgi`; teach `internal/lifecycle` to plumb the right upstream and target the right exec container.

**Tech Stack:** Go 1.22+, Cobra, Docker Compose, FrankenPHP (`dunglas/frankenphp:php8.3-alpine`), Laravel Octane (project-level).

**Spec:** `docs/superpowers/specs/2026-05-02-frankenphp-runtime-design.md`

**Working directory:** `/home/nathan/Projects/dev-cli`

---

## Conventions used throughout

- Every code-change task follows TDD: write failing test → run it failing → implement → run it passing → commit.
- Tests live next to the code (`<file>_test.go`), use table-driven style for pure functions and subtests with `t.Helper()` / `t.Cleanup()` for behavior tests, per project standards in CLAUDE.md.
- After every task, run `go vet ./...` and `go test ./...` from the repo root before committing. The plan's commit step assumes both pass.
- Commit messages follow the existing style — short subject in lowercase imperative, optional body. Use the `feat:` / `fix:` / `refactor:` / `test:` prefixes already in `git log`.
- Module path is `github.com/XBS-Nathan/nova` (despite the binary being called `dev` historically — the codebase uses `nova`).

---

## Task 1: Config — add Runtime and Octane fields with constants

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write failing tests for Runtime/Octane parsing and defaults**

Append the following to `internal/config/config_test.go` (above the last closing brace if there's package-level helpers, otherwise at the bottom of the file):

```go
func TestLoad_RuntimeDefault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Runtime != RuntimeFPM {
		t.Errorf("Runtime default = %q, want %q", cfg.Runtime, RuntimeFPM)
	}
	if cfg.Octane {
		t.Error("Octane default = true, want false")
	}
}

func TestLoad_RuntimeFrankenPHP(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	novaDir := filepath.Join(dir, ".nova")
	if err := os.MkdirAll(novaDir, 0755); err != nil {
		t.Fatal(err)
	}
	yaml := "runtime: frankenphp\noctane: true\n"
	if err := os.WriteFile(filepath.Join(novaDir, "config.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Runtime != RuntimeFrankenPHP {
		t.Errorf("Runtime = %q, want %q", cfg.Runtime, RuntimeFrankenPHP)
	}
	if !cfg.Octane {
		t.Error("Octane = false, want true")
	}
}
```

If `os` and `path/filepath` aren't yet imported in the test file, add them.

- [ ] **Step 2: Run the new tests — expect failures**

Run: `go test ./internal/config/ -run 'TestLoad_Runtime' -v`
Expected: compile error / unknown identifier `RuntimeFPM`, `RuntimeFrankenPHP`, `cfg.Runtime`, `cfg.Octane`.

- [ ] **Step 3: Add fields and constants to `internal/config/config.go`**

In the `const (...)` block that contains `DefaultPHP`, add:

```go
RuntimeFPM        = "fpm"
RuntimeFrankenPHP = "frankenphp"
```

In `ProjectConfig`, add these fields directly under `PHP`:

```go
Runtime string `yaml:"runtime"`
Octane  bool   `yaml:"octane"`
```

In `fillDefaults`, after the `cfg.PHP` default is set, add:

```go
if cfg.Runtime == "" {
    cfg.Runtime = RuntimeFPM
}
```

- [ ] **Step 4: Re-run tests — expect pass**

Run: `go test ./internal/config/ -run 'TestLoad_Runtime' -v`
Expected: PASS for both subtests.

- [ ] **Step 5: Run the full config suite to make sure nothing else broke**

Run: `go test ./internal/config/ -v`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add runtime and octane fields"
```

---

## Task 2: Config — validate that octane requires frankenphp

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/config/config_test.go`:

```go
func TestLoad_OctaneRequiresFrankenPHP(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	novaDir := filepath.Join(dir, ".nova")
	if err := os.MkdirAll(novaDir, 0755); err != nil {
		t.Fatal(err)
	}
	yaml := "octane: true\n" // runtime omitted → defaults to fpm
	if err := os.WriteFile(filepath.Join(novaDir, "config.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load: want error, got nil")
	}
	if !strings.Contains(err.Error(), "octane") {
		t.Errorf("error = %v, want it to mention octane", err)
	}
}
```

If `strings` isn't imported in the test file, add it.

- [ ] **Step 2: Run — expect failure**

Run: `go test ./internal/config/ -run TestLoad_OctaneRequiresFrankenPHP -v`
Expected: FAIL — "Load: want error, got nil".

- [ ] **Step 3: Implement validation in `Load`**

In `internal/config/config.go`, change `Load`'s tail. After the call to `fillDefaults(cfg, projectDir)` and before `return cfg, nil`, insert:

```go
if cfg.Octane && cfg.Runtime != RuntimeFrankenPHP {
    return nil, fmt.Errorf("parsing %s: octane: true requires runtime: frankenphp", path)
}
```

Note: `path` is in scope only when the file existed. For the no-file branch (where `fillDefaults` is also called), `cfg.Octane` will be false because zero-valued, so no validation needed there. Move the validation to *after* the only return that loaded a file. Concretely, the existing flow is:

```go
if err := yaml.Unmarshal(data, cfg); err != nil { ... }
fillDefaults(cfg, projectDir)
return cfg, nil
```

Make it:

```go
if err := yaml.Unmarshal(data, cfg); err != nil { ... }
fillDefaults(cfg, projectDir)
if cfg.Octane && cfg.Runtime != RuntimeFrankenPHP {
    return nil, fmt.Errorf("parsing %s: octane: true requires runtime: frankenphp", path)
}
return cfg, nil
```

- [ ] **Step 4: Run — expect pass**

Run: `go test ./internal/config/ -run TestLoad_OctaneRequiresFrankenPHP -v`
Expected: PASS.

- [ ] **Step 5: Full config suite**

Run: `go test ./internal/config/ -v`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): reject octane=true without runtime=frankenphp"
```

---

## Task 3: phpimage — add Runtime field and namespace ImageTag

**Files:**
- Modify: `internal/phpimage/phpimage.go`
- Test: `internal/phpimage/phpimage_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/phpimage/phpimage_test.go`:

```go
func TestImageTag_FPM(t *testing.T) {
	t.Parallel()
	tag := ImageTag(ImageConfig{PHPVersion: "8.3", Runtime: "fpm"})
	if !strings.HasPrefix(tag, "nova-fpm:8.3-") {
		t.Errorf("ImageTag = %q, want prefix nova-fpm:8.3-", tag)
	}
}

func TestImageTag_FrankenPHP(t *testing.T) {
	t.Parallel()
	tag := ImageTag(ImageConfig{PHPVersion: "8.3", Runtime: "frankenphp"})
	if !strings.HasPrefix(tag, "nova-frankenphp:8.3-") {
		t.Errorf("ImageTag = %q, want prefix nova-frankenphp:8.3-", tag)
	}
}

func TestImageTag_DefaultRuntimeIsFPM(t *testing.T) {
	t.Parallel()
	// Empty Runtime should be treated as fpm so existing callers keep working.
	tag := ImageTag(ImageConfig{PHPVersion: "8.3"})
	if !strings.HasPrefix(tag, "nova-fpm:8.3-") {
		t.Errorf("ImageTag = %q, want prefix nova-fpm:8.3-", tag)
	}
}

func TestImageTag_RuntimesProduceDifferentHashes(t *testing.T) {
	t.Parallel()
	fpm := ImageTag(ImageConfig{PHPVersion: "8.3", Runtime: "fpm", Extensions: []string{"gd"}})
	franken := ImageTag(ImageConfig{PHPVersion: "8.3", Runtime: "frankenphp", Extensions: []string{"gd"}})
	if fpm == franken {
		t.Errorf("expected different tags, got %q for both", fpm)
	}
}
```

If `strings` isn't already imported in the test file, add it.

- [ ] **Step 2: Run — expect failure**

Run: `go test ./internal/phpimage/ -run TestImageTag -v`
Expected: compile error — `Runtime` is not a field of `ImageConfig`.

- [ ] **Step 3: Add `Runtime` field, normalize default, namespace tag**

In `internal/phpimage/phpimage.go`:

1. Update the struct:

```go
type ImageConfig struct {
    PHPVersion string
    Extensions []string
    Runtime    string // "fpm" (default) or "frankenphp"
}
```

2. Add a helper near the top of the file (after the var blocks, before `EnsureBuilt`):

```go
// runtime returns the runtime, defaulting to "fpm" when unset.
func (c ImageConfig) runtime() string {
    if c.Runtime == "" {
        return "fpm"
    }
    return c.Runtime
}
```

3. Change `ImageTag`:

```go
func ImageTag(cfg ImageConfig) string {
    hash := imageHash(cfg)
    return fmt.Sprintf("nova-%s:%s-%s", cfg.runtime(), cfg.PHPVersion, hash)
}
```

- [ ] **Step 4: Run — expect pass**

Run: `go test ./internal/phpimage/ -run TestImageTag -v`
Expected: PASS for all four subtests.

- [ ] **Step 5: Run full phpimage suite — confirm no regressions**

Run: `go test ./internal/phpimage/ -v`
Expected: all PASS. The existing `TestImageTag` (if any) still passes because empty `Runtime` defaults to fpm.

- [ ] **Step 6: Commit**

```bash
git add internal/phpimage/phpimage.go internal/phpimage/phpimage_test.go
git commit -m "feat(phpimage): namespace image tag by runtime"
```

---

## Task 4: phpimage — branch Dockerfile generation on runtime

**Files:**
- Modify: `internal/phpimage/phpimage.go`
- Test: `internal/phpimage/phpimage_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/phpimage/phpimage_test.go`:

```go
func TestGenerateDockerfile_FPM_BaseImage(t *testing.T) {
	t.Parallel()
	df := generateDockerfile(ImageConfig{PHPVersion: "8.3", Runtime: "fpm"})
	if !strings.Contains(df, "FROM php:8.3-fpm-alpine") {
		t.Errorf("FPM Dockerfile missing fpm base, got:\n%s", df)
	}
	if !strings.Contains(df, "php-fpm.d/www.conf") {
		t.Errorf("FPM Dockerfile missing www.conf strip, got:\n%s", df)
	}
}

func TestGenerateDockerfile_FrankenPHP_BaseImage(t *testing.T) {
	t.Parallel()
	df := generateDockerfile(ImageConfig{PHPVersion: "8.3", Runtime: "frankenphp"})
	if !strings.Contains(df, "FROM dunglas/frankenphp:php8.3-alpine") {
		t.Errorf("FrankenPHP Dockerfile missing frankenphp base, got:\n%s", df)
	}
	if strings.Contains(df, "php-fpm.d/www.conf") {
		t.Errorf("FrankenPHP Dockerfile should not strip www.conf, got:\n%s", df)
	}
	if !strings.Contains(df, "COPY Caddyfile /etc/caddy/Caddyfile") {
		t.Errorf("FrankenPHP Dockerfile missing Caddyfile copy, got:\n%s", df)
	}
}

func TestGenerateDockerfile_FrankenPHP_KeepsExtensionInstall(t *testing.T) {
	t.Parallel()
	df := generateDockerfile(ImageConfig{PHPVersion: "8.3", Runtime: "frankenphp", Extensions: []string{"gd"}})
	if !strings.Contains(df, "docker-php-ext-install") {
		t.Errorf("FrankenPHP Dockerfile missing docker-php-ext-install, got:\n%s", df)
	}
	if !strings.Contains(df, "pecl install") {
		t.Errorf("FrankenPHP Dockerfile missing pecl install, got:\n%s", df)
	}
}
```

- [ ] **Step 2: Run — expect failure**

Run: `go test ./internal/phpimage/ -run TestGenerateDockerfile -v`
Expected: FAIL on the FrankenPHP tests — base image is `php:8.3-fpm-alpine` for both.

- [ ] **Step 3: Implement the branch in `generateDockerfile`**

Replace the existing `generateDockerfile` in `internal/phpimage/phpimage.go` with:

```go
func generateDockerfile(cfg ImageConfig) string {
    var native, pecl []string
    var buildDeps, runtimeDeps []string

    sorted := make([]string, len(cfg.Extensions))
    copy(sorted, cfg.Extensions)
    sort.Strings(sorted)

    for _, ext := range sorted {
        if deps, ok := nativeExtDeps[ext]; ok {
            native = append(native, ext)
            buildDeps = append(buildDeps, deps...)
            runtimeDeps = append(runtimeDeps, nativeExtRuntime[ext]...)
        } else {
            pecl = append(pecl, ext)
        }
    }

    var b strings.Builder

    switch cfg.runtime() {
    case "frankenphp":
        fmt.Fprintf(&b, "FROM dunglas/frankenphp:php%s-alpine\n\n", cfg.PHPVersion)
    default:
        fmt.Fprintf(&b, "FROM php:%s-fpm-alpine\n\n", cfg.PHPVersion)
    }

    allRuntimeDeps := append(baseRuntimeDeps, runtimeDeps...)
    fmt.Fprintf(&b, "RUN apk add --no-cache %s\n\n", strings.Join(allRuntimeDeps, " "))

    allBuildDeps := append(baseBuildDeps, buildDeps...)
    fmt.Fprintf(&b, "RUN apk add --no-cache --virtual .build-deps linux-headers $PHPIZE_DEPS %s",
        strings.Join(allBuildDeps, " "))
    fmt.Fprintf(&b, " \\\n")

    allNative := baseExtensions
    if len(native) > 0 {
        allNative += " " + strings.Join(native, " ")
    }

    if hasGD(native) {
        fmt.Fprintf(&b, "    && docker-php-ext-configure gd --with-freetype --with-jpeg \\\n")
    }

    fmt.Fprintf(&b, "    && docker-php-ext-install %s \\\n", allNative)
    fmt.Fprintf(&b, "    && pecl install redis xdebug \\\n")
    fmt.Fprintf(&b, "    && docker-php-ext-enable redis \\\n")

    if len(pecl) > 0 {
        fmt.Fprintf(&b, "    && pecl install %s \\\n", strings.Join(pecl, " "))
        fmt.Fprintf(&b, "    && docker-php-ext-enable %s \\\n", strings.Join(pecl, " "))
    }

    fmt.Fprintf(&b, "    && apk del .build-deps\n\n")

    fmt.Fprintf(&b, "RUN mkdir -p /usr/local/etc/php/conf.custom\n")
    fmt.Fprintf(&b, "ENV PHP_INI_SCAN_DIR=/usr/local/etc/php/conf.d:/usr/local/etc/php/conf.custom\n\n")

    if cfg.runtime() == "fpm" {
        fmt.Fprintf(&b, "RUN sed -i '/^user = /d; /^group = /d' /usr/local/etc/php-fpm.d/www.conf\n\n")
    }

    fmt.Fprintf(&b, "COPY --from=composer:latest /usr/bin/composer /usr/bin/composer\n")
    fmt.Fprintf(&b, "COPY php.ini /usr/local/etc/php/php.ini\n")
    fmt.Fprintf(&b, "COPY my.cnf /etc/my.cnf.d/dev.cnf\n")

    if cfg.runtime() == "frankenphp" {
        fmt.Fprintf(&b, "COPY Caddyfile /etc/caddy/Caddyfile\n")
    }

    fmt.Fprintf(&b, "\nWORKDIR /srv\n")

    return b.String()
}
```

- [ ] **Step 4: Run — expect pass**

Run: `go test ./internal/phpimage/ -run TestGenerateDockerfile -v`
Expected: PASS.

- [ ] **Step 5: Full phpimage suite**

Run: `go test ./internal/phpimage/ -v`
Expected: all PASS. The existing `generateDockerfile` test (if any) using empty Runtime still produces FPM output → unchanged.

- [ ] **Step 6: Commit**

```bash
git add internal/phpimage/phpimage.go internal/phpimage/phpimage_test.go
git commit -m "feat(phpimage): branch dockerfile on runtime"
```

---

## Task 5: phpimage — write Caddyfile alongside Dockerfile for FrankenPHP

**Files:**
- Modify: `internal/phpimage/phpimage.go`
- Test: `internal/phpimage/phpimage_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/phpimage/phpimage_test.go`:

```go
func TestWriteDockerfile_FrankenPHPWritesCaddyfile(t *testing.T) {
	t.Parallel()
	t.Setenv("HOME", t.TempDir()) // GlobalDir() resolves under HOME

	dir, err := writeDockerfile(ImageConfig{PHPVersion: "8.3", Runtime: "frankenphp"})
	if err != nil {
		t.Fatalf("writeDockerfile: %v", err)
	}

	caddyfilePath := filepath.Join(dir, "Caddyfile")
	data, err := os.ReadFile(caddyfilePath)
	if err != nil {
		t.Fatalf("reading Caddyfile: %v", err)
	}
	content := string(data)
	for _, want := range []string{"frankenphp", "auto_https off", "{$NOVA_APP}", "php_server"} {
		if !strings.Contains(content, want) {
			t.Errorf("Caddyfile missing %q, got:\n%s", want, content)
		}
	}
}

func TestWriteDockerfile_FPMDoesNotWriteCaddyfile(t *testing.T) {
	t.Parallel()
	t.Setenv("HOME", t.TempDir())

	dir, err := writeDockerfile(ImageConfig{PHPVersion: "8.3", Runtime: "fpm"})
	if err != nil {
		t.Fatalf("writeDockerfile: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "Caddyfile")); !os.IsNotExist(err) {
		t.Errorf("FPM should not write Caddyfile, stat err = %v", err)
	}
}
```

If `os` and `path/filepath` aren't imported in the test file, add them.

- [ ] **Step 2: Run — expect failure**

Run: `go test ./internal/phpimage/ -run TestWriteDockerfile -v`
Expected: FAIL — Caddyfile missing on FrankenPHP path.

- [ ] **Step 3: Implement Caddyfile write in `writeDockerfile`**

In `writeDockerfile`, after the `my.cnf` write block and before `return dir, nil`, insert:

```go
if cfg.runtime() == "frankenphp" {
    caddyfile := `{
    frankenphp
    auto_https off
    admin off
}
:8000 {
    root * /srv/{$NOVA_APP}/public
    php_server
}
`
    if err := os.WriteFile(filepath.Join(dir, "Caddyfile"), []byte(caddyfile), 0644); err != nil {
        return "", fmt.Errorf("writing Caddyfile: %w", err)
    }
}
```

- [ ] **Step 4: Run — expect pass**

Run: `go test ./internal/phpimage/ -run TestWriteDockerfile -v`
Expected: PASS.

- [ ] **Step 5: Full phpimage suite**

Run: `go test ./internal/phpimage/ -v`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/phpimage/phpimage.go internal/phpimage/phpimage_test.go
git commit -m "feat(phpimage): write FrankenPHP Caddyfile next to Dockerfile"
```

---

## Task 6: caddy — introduce Upstream type and branch site config

**Files:**
- Modify: `internal/caddy/caddy.go`
- Modify: `internal/caddy/service.go`
- Test: `internal/caddy/caddy_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/caddy/caddy_test.go`:

```go
func TestGenerateSiteConfig_FastCGI(t *testing.T) {
	t.Parallel()
	got := generateSiteConfig("myapp", "/srv/myapp/public",
		Upstream{Kind: "fastcgi", Address: "php83:9000"}, nil)

	for _, want := range []string{
		"myapp.test {",
		"root * /srv/myapp/public",
		"php_fastcgi php83:9000",
		"file_server",
		"encode gzip",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestGenerateSiteConfig_ReverseProxy(t *testing.T) {
	t.Parallel()
	got := generateSiteConfig("myapp", "/srv/myapp/public",
		Upstream{Kind: "reverse_proxy", Address: "myapp_frankenphp:8000"}, nil)

	if !strings.Contains(got, "reverse_proxy myapp_frankenphp:8000") {
		t.Errorf("missing reverse_proxy directive, got:\n%s", got)
	}
	for _, unwant := range []string{"root * ", "php_fastcgi", "file_server"} {
		if strings.Contains(got, unwant) {
			t.Errorf("reverse_proxy site should not contain %q, got:\n%s", unwant, got)
		}
	}
}
```

If `strings` isn't imported, add it.

- [ ] **Step 2: Run — expect failure**

Run: `go test ./internal/caddy/ -run TestGenerateSiteConfig -v`
Expected: compile error — `Upstream` type doesn't exist; `generateSiteConfig` signature mismatch.

- [ ] **Step 3: Implement Upstream and update generators**

In `internal/caddy/caddy.go`, add the type near the top after `PortProxy`:

```go
// Upstream describes how Caddy reaches the project's PHP runtime.
// Kind is "fastcgi" (PHP-FPM via php_fastcgi) or "reverse_proxy" (FrankenPHP via plain HTTP).
type Upstream struct {
    Kind    string
    Address string
}
```

Replace `Link`'s signature and body:

```go
func Link(siteName, docroot string, upstream Upstream, portProxies []PortProxy) error {
    caddyDir := filepath.Join(config.GlobalDir(), "caddy")
    if err := writeSiteConfig(caddyDir, siteName, docroot, upstream, portProxies); err != nil {
        return err
    }
    if err := writeMainCaddyfile(caddyDir); err != nil {
        return err
    }
    return Reload()
}
```

Replace `writeSiteConfig`:

```go
func writeSiteConfig(caddyDir, siteName, docroot string, upstream Upstream, portProxies []PortProxy) error {
    sitesDir := filepath.Join(caddyDir, "sites")
    if err := os.MkdirAll(sitesDir, 0755); err != nil {
        return fmt.Errorf("creating sites dir: %w", err)
    }

    content := generateSiteConfig(siteName, docroot, upstream, portProxies)
    path := filepath.Join(sitesDir, siteName+".caddy")
    if err := os.WriteFile(path, []byte(content), 0644); err != nil {
        return fmt.Errorf("writing site config: %w", err)
    }
    return nil
}
```

Replace `generateSiteConfig`:

```go
func generateSiteConfig(siteName, docroot string, upstream Upstream, portProxies []PortProxy) string {
    var b strings.Builder

    fmt.Fprintf(&b, "%s.test {\n", siteName)
    switch upstream.Kind {
    case "reverse_proxy":
        fmt.Fprintf(&b, "\treverse_proxy %s\n", upstream.Address)
    default: // "fastcgi"
        fmt.Fprintf(&b, "\troot * %s\n", docroot)
        fmt.Fprintf(&b, "\tphp_fastcgi %s\n", upstream.Address)
        b.WriteString("\tfile_server\n")
        b.WriteString("\tencode gzip\n")
    }
    b.WriteString("}\n")

    for _, pp := range portProxies {
        fmt.Fprintf(&b, "\n%s.test:%s {\n", siteName, pp.Port)
        fmt.Fprintf(&b, "\treverse_proxy %s:%s\n", pp.Backend, pp.Port)
        b.WriteString("}\n")
    }

    return b.String()
}
```

- [ ] **Step 4: Update `internal/caddy/service.go` to match the new signature**

Replace the file contents:

```go
package caddy

// Service wraps the caddy package functions for the lifecycle interface.
type Service struct{}

func (Service) Start() error { return nil }
func (Service) Stop() error  { return nil }
func (Service) Link(siteName, docroot string, upstream Upstream, portProxies []PortProxy) error {
	return Link(siteName, docroot, upstream, portProxies)
}
func (Service) Unlink(siteName string) error { return Unlink(siteName) }
func (Service) Reload() error                { return Reload() }
```

- [ ] **Step 5: Run — expect pass**

Run: `go test ./internal/caddy/ -v`
Expected: all PASS.

- [ ] **Step 6: Build everything (will surface broken callers in lifecycle/cmd, fixed in later tasks)**

Run: `go build ./internal/caddy/...`
Expected: PASS. The wider repo still won't build until lifecycle/cmd are updated; that's fine — those tasks come next.

- [ ] **Step 7: Commit**

```bash
git add internal/caddy/caddy.go internal/caddy/service.go internal/caddy/caddy_test.go
git commit -m "feat(caddy): add Upstream type and branch site config"
```

---

## Task 7: docker — add FrankenPHPProject type and ComposeOptions field

**Files:**
- Modify: `internal/docker/docker.go`

- [ ] **Step 1: No new behavior yet — this task just adds the type.** Skip the test step; we'll cover behavior in Task 8.

- [ ] **Step 2: Add `FrankenPHPProject` and the field**

In `internal/docker/docker.go`, after the `PHPVersion` struct, add:

```go
// FrankenPHPProject describes a per-project FrankenPHP service.
// Only the active project's FrankenPHP service is included in the compose file.
type FrankenPHPProject struct {
    Name       string   // sanitized project name, used for service name and NOVA_APP
    PHPVersion string
    Extensions []string
    Octane     bool
    Workdir    string   // "/srv/<name>"
    Ports      []string // mirrors PHPVersion.Ports for parity
}
```

Add the field to `ComposeOptions` (place it directly under `PHP`):

```go
FrankenPHP []FrankenPHPProject
```

- [ ] **Step 3: Build to confirm it compiles**

Run: `go build ./internal/docker/...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/docker/docker.go
git commit -m "feat(docker): add FrankenPHPProject type"
```

---

## Task 8: docker — emit FrankenPHP service in generateCompose

**Files:**
- Modify: `internal/docker/docker.go`
- Test: `internal/docker/docker_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/docker/docker_test.go`:

```go
func TestGenerateCompose_FrankenPHP_Octane(t *testing.T) {
	t.Parallel()
	t.Setenv("HOME", t.TempDir())

	got := generateCompose(ComposeOptions{
		ProjectsDir:    "/proj",
		MailpitVersion: "v1.20",
		FrankenPHP: []FrankenPHPProject{{
			Name:       "myapp",
			PHPVersion: "8.3",
			Extensions: []string{"gd"},
			Octane:     true,
			Workdir:    "/srv/myapp",
		}},
	})

	for _, want := range []string{
		"myapp_frankenphp:",
		"working_dir: /srv/myapp",
		"NOVA_APP: \"myapp\"",
		"octane:start",
		"--server=frankenphp",
		"--host=0.0.0.0",
		"--port=8000",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in compose:\n%s", want, got)
		}
	}

	// No host port mapping — internal only.
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "8000:8000") {
			t.Errorf("FrankenPHP service should not expose host port 8000, line: %q", line)
		}
	}
}

func TestGenerateCompose_FrankenPHP_ClassicMode(t *testing.T) {
	t.Parallel()
	t.Setenv("HOME", t.TempDir())

	got := generateCompose(ComposeOptions{
		ProjectsDir:    "/proj",
		MailpitVersion: "v1.20",
		FrankenPHP: []FrankenPHPProject{{
			Name:       "myapp",
			PHPVersion: "8.3",
			Octane:     false,
			Workdir:    "/srv/myapp",
		}},
	})

	if strings.Contains(got, "octane:start") {
		t.Errorf("classic mode should not emit octane:start command, got:\n%s", got)
	}
	if !strings.Contains(got, "myapp_frankenphp:") {
		t.Errorf("classic mode missing myapp_frankenphp service, got:\n%s", got)
	}
}

func TestGenerateCompose_NoFrankenPHP_NoChange(t *testing.T) {
	t.Parallel()
	t.Setenv("HOME", t.TempDir())

	// Without any FrankenPHP entries, the output should not mention frankenphp.
	got := generateCompose(ComposeOptions{
		ProjectsDir:    "/proj",
		MailpitVersion: "v1.20",
		PHP: []PHPVersion{{Version: "8.3"}},
	})
	if strings.Contains(got, "frankenphp") {
		t.Errorf("compose should not mention frankenphp when no FrankenPHP projects, got:\n%s", got)
	}
}
```

- [ ] **Step 2: Run — expect failure**

Run: `go test ./internal/docker/ -run TestGenerateCompose_FrankenPHP -v`
Expected: FAIL — service block not present.

- [ ] **Step 3: Emit the FrankenPHP block in `generateCompose`**

In `internal/docker/docker.go`, locate the PHP-services loop (`for _, php := range opts.PHP`). Directly after that loop's closing brace, insert:

```go
// FrankenPHP services (one per active opted-in project)
for _, fp := range opts.FrankenPHP {
    img := phpimage.ImageTag(phpimage.ImageConfig{
        PHPVersion: fp.PHPVersion,
        Extensions: fp.Extensions,
        Runtime:    "frankenphp",
    })
    fmt.Fprintf(&b, "  %s_frankenphp:\n", fp.Name)
    fmt.Fprintf(&b, "    image: %s\n", img)
    b.WriteString("    pull_policy: never\n")
    fmt.Fprintf(&b, "    user: \"%d:%d\"\n", os.Getuid(), os.Getgid())
    b.WriteString("    restart: unless-stopped\n")
    fmt.Fprintf(&b, "    working_dir: %s\n", fp.Workdir)
    b.WriteString("    environment:\n")
    b.WriteString("      NOVA: \"true\"\n")
    fmt.Fprintf(&b, "      NOVA_APP: %q\n", fp.Name)
    if fp.Octane {
        b.WriteString("    command: [\"php\", \"artisan\", \"octane:start\",")
        b.WriteString(" \"--server=frankenphp\", \"--host=0.0.0.0\", \"--port=8000\",")
        b.WriteString(" \"--workers=auto\", \"--max-requests=500\"]\n")
    }
    if throttle := config.LoadThrottle(); throttle != nil {
        b.WriteString("    deploy:\n")
        b.WriteString("      resources:\n")
        b.WriteString("        limits:\n")
        if throttle.CPUs != "" {
            fmt.Fprintf(&b, "          cpus: \"%s\"\n", throttle.CPUs)
        }
        if throttle.Memory != "" {
            fmt.Fprintf(&b, "          memory: %s\n", throttle.Memory)
        }
    }
    b.WriteString("    volumes:\n")
    fmt.Fprintf(&b, "      - %s:/srv\n", opts.ProjectsDir)
    fmt.Fprintf(&b, "      - %s/php/%s/conf.d:/usr/local/etc/php/conf.custom\n",
        globalDir, fp.PHPVersion)
    b.WriteString("    networks: [nova]\n\n")
}
```

- [ ] **Step 4: Run — expect pass**

Run: `go test ./internal/docker/ -run TestGenerateCompose -v`
Expected: PASS.

- [ ] **Step 5: Full docker suite**

Run: `go test ./internal/docker/ -v`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/docker/docker.go internal/docker/docker_test.go
git commit -m "feat(docker): emit per-project FrankenPHP service in compose"
```

---

## Task 9: docker — change Service.Up to accept FrankenPHP slice

**Files:**
- Modify: `internal/docker/service.go`

- [ ] **Step 1: Update Service.Up signature**

In `internal/docker/service.go`, replace `Up`:

```go
func (s Service) Up(php []PHPVersion, frankenphp []FrankenPHPProject, forceRecreate bool) error {
	return Up(ComposeOptions{
		ProjectsDir:      s.ProjectsDir,
		PHP:              php,
		FrankenPHP:       frankenphp,
		MySQLVersions:    s.Collected.MySQL,
		PostgresVersions: s.Collected.Postgres,
		RedisVersions:    s.Collected.Redis,
		MailpitVersion:   s.MailpitVersion,
		SharedServices:   s.Collected.SharedServices,
		ForceRecreate:    forceRecreate,
	})
}
```

- [ ] **Step 2: Build — expect callers in lifecycle/cmd to break (they're fixed in Tasks 10–14)**

Run: `go build ./internal/docker/...`
Expected: PASS for the docker package itself.

Run: `go build ./...`
Expected: FAIL with errors about `Up` argument count in `internal/lifecycle` and `cmd/`. That's planned and gets fixed in the upcoming tasks.

- [ ] **Step 3: Don't commit yet — wait until callers compile**

We commit after Task 13 so the repo is buildable at every commit boundary.

---

## Task 10: lifecycle — add PHPContainer helper

**Files:**
- Create: `internal/lifecycle/runtime.go`
- Test: `internal/lifecycle/runtime_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/lifecycle/runtime_test.go`:

```go
package lifecycle

import (
	"testing"

	"github.com/XBS-Nathan/nova/internal/config"
)

func TestPHPContainer_FPM(t *testing.T) {
	t.Parallel()
	cfg := &config.ProjectConfig{PHP: "8.3", Runtime: config.RuntimeFPM}
	got := PHPContainer(cfg, "myapp")
	if got != "php83" {
		t.Errorf("PHPContainer fpm = %q, want %q", got, "php83")
	}
}

func TestPHPContainer_FrankenPHP(t *testing.T) {
	t.Parallel()
	cfg := &config.ProjectConfig{PHP: "8.3", Runtime: config.RuntimeFrankenPHP}
	got := PHPContainer(cfg, "myapp")
	if got != "myapp_frankenphp" {
		t.Errorf("PHPContainer frankenphp = %q, want %q", got, "myapp_frankenphp")
	}
}
```

- [ ] **Step 2: Run — expect compile failure**

Run: `go test ./internal/lifecycle/ -run TestPHPContainer -v`
Expected: undefined: `PHPContainer`.

- [ ] **Step 3: Create the helper**

Create `internal/lifecycle/runtime.go`:

```go
package lifecycle

import (
	"github.com/XBS-Nathan/nova/internal/config"
	"github.com/XBS-Nathan/nova/internal/docker"
)

// PHPContainer returns the docker compose service name to exec into for a project.
// FPM projects share the global "phpXX" container; FrankenPHP projects have their
// own per-project container.
func PHPContainer(cfg *config.ProjectConfig, projectName string) string {
	if cfg.Runtime == config.RuntimeFrankenPHP {
		return projectName + "_frankenphp"
	}
	return docker.PHPServiceName(cfg.PHP)
}
```

- [ ] **Step 4: Run — expect pass**

Run: `go test ./internal/lifecycle/ -run TestPHPContainer -v`
Expected: PASS.

- [ ] **Step 5: Don't commit yet — interface changes in next task break the build.**

---

## Task 11: lifecycle — change DockerService and CaddyService interfaces

**Files:**
- Modify: `internal/lifecycle/lifecycle.go`
- Modify: `internal/lifecycle/lifecycle_test.go`

- [ ] **Step 1: Update interface signatures**

In `internal/lifecycle/lifecycle.go`, replace the `DockerService` and `CaddyService` interfaces:

```go
// DockerService manages shared and per-project Docker containers.
type DockerService interface {
	Up(php []docker.PHPVersion, frankenphp []docker.FrankenPHPProject, forceRecreate bool) error
	Down() error
	Exec(service string, workdir string, args ...string) error
	ExecDetached(service string, workdir string, args ...string) error
	UpProject(projectName, projectDir string, services map[string]config.ServiceDefinition) error
	DownProject(projectName, projectDir string) error
}

// CaddyService manages the Caddy reverse proxy.
type CaddyService interface {
	Start() error
	Stop() error
	Link(siteName, docroot string, upstream caddy.Upstream, portProxies []caddy.PortProxy) error
	Unlink(siteName string) error
	Reload() error
}
```

- [ ] **Step 2: Update `Lifecycle.Start` to plumb both arguments and pick the upstream**

Replace `Lifecycle.Start`'s function signature and body. The new signature takes a `frankenphp` slice in addition to `php`:

```go
func (l *Lifecycle) Start(
	p *project.Project,
	php []docker.PHPVersion,
	frankenphp []docker.FrankenPHPProject,
	forceRecreate bool,
) error {
	pterm.DefaultSection.Printfln("Starting %s", p.Name)

	phpSvc := PHPContainer(p.Config, p.Name)
	docroot := l.Docroot(p)
	projectRoot := strings.TrimSuffix(docroot, "/public")

	if err := l.spin("Starting services", func() error {
		return l.Docker.Up(php, frankenphp, forceRecreate)
	}); err != nil {
		return fmt.Errorf("starting services: %w", err)
	}
```

(Leave the bullet-list rendering, port-proxies, and database-creation logic between this and the Caddy.Link block exactly as it was.)

Replace the Caddy.Link block:

```go
	upstream := caddy.Upstream{Kind: "fastcgi", Address: phpSvc + ":9000"}
	if p.Config.Runtime == config.RuntimeFrankenPHP {
		upstream = caddy.Upstream{Kind: "reverse_proxy", Address: phpSvc + ":8000"}
	}

	if err := l.spin("Linking site", func() error {
		return l.Caddy.Link(p.Name, docroot, upstream, portProxies)
	}); err != nil {
		return fmt.Errorf("linking site: %w", err)
	}
```

In `Lifecycle.Stop`, replace the `phpSvc := l.PHPService(p.Config.PHP)` line with:

```go
	phpSvc := PHPContainer(p.Config, p.Name)
```

The `PHPService` field on `Lifecycle` is now unused — delete its declaration in the struct and the `phpSvc := l.PHPService(...)` shadowing is replaced wherever it appeared. Search for `l.PHPService(` and replace each with `PHPContainer(p.Config, p.Name)`.

- [ ] **Step 3: Update mocks in `internal/lifecycle/lifecycle_test.go`**

Open `internal/lifecycle/lifecycle_test.go` and update the mock structs. The existing `mockDocker.Up` becomes:

```go
func (m *mockDocker) Up(php []docker.PHPVersion, frankenphp []docker.FrankenPHPProject, forceRecreate bool) error {
	m.upPHP = php
	m.upFrankenPHP = frankenphp
	m.upForceRecreate = forceRecreate
	return m.upErr
}
```

Add the new field to the `mockDocker` struct:

```go
upFrankenPHP []docker.FrankenPHPProject
```

Update the `mockCaddy.Link` method (find it and replace):

```go
func (m *mockCaddy) Link(siteName, docroot string, upstream caddy.Upstream, portProxies []caddy.PortProxy) error {
	m.linkSite = siteName
	m.linkDocroot = docroot
	m.linkUpstream = upstream
	m.linkPortProxies = portProxies
	return m.linkErr
}
```

Add the new field to `mockCaddy`:

```go
linkUpstream caddy.Upstream
```

Search for any test that calls `lc.Start(p, []docker.PHPVersion{...}, false)` (the existing signature) and update each call site to:

```go
lc.Start(p, []docker.PHPVersion{...}, nil, false)
```

(Just add `nil` between the PHP slice and the bool. The grep from earlier shows the lines: `lifecycle_test.go:152, 178, 201, 221, 242, 411`.)

If any test asserts on `linkPHPSvc` or the old `phpService` Link argument, change those assertions to read `linkUpstream.Address` (compare to `"php82:9000"` to keep the existing semantics).

- [ ] **Step 4: Run lifecycle tests**

Run: `go test ./internal/lifecycle/ -v`
Expected: PASS. If any pre-existing test depends on `Lifecycle.PHPService`, remove that field reference from the test setup (we removed the field from the struct).

- [ ] **Step 5: Commit (this is the first compilable commit since Task 9)**

Note: at this point `cmd/` still won't compile. Don't run `go build ./...` for the commit gate yet — Task 12 finishes the wiring. We commit at this boundary because lifecycle is internally consistent and tested.

```bash
git add internal/lifecycle/lifecycle.go internal/lifecycle/lifecycle_test.go internal/lifecycle/runtime.go internal/lifecycle/runtime_test.go internal/docker/service.go
git commit -m "refactor(lifecycle): plumb runtime through Up and Link"
```

---

## Task 12: cmd — runtime-aware exec helper, build, services, slow

**Files:**
- Modify: `cmd/artisan.go` (the central `runInContainer` lives here)
- Modify: `cmd/xdebug.go`
- Modify: `cmd/build.go`
- Modify: `cmd/services.go`
- Modify: `cmd/slow.go`

- [ ] **Step 1: Update `runInContainer` in `cmd/artisan.go`**

Replace the body of `runInContainer`:

```go
func runInContainer(args ...string) error {
	p, err := project.Detect()
	if err != nil {
		return err
	}
	global, err := config.LoadGlobal()
	if err != nil {
		return err
	}
	workdir, err := containerWorkdir(global.ProjectsDir, p.Dir)
	if err != nil {
		return err
	}
	svc := lifecycle.PHPContainer(p.Config, p.Name)
	return docker.Exec(svc, workdir, args...)
}
```

Add `"github.com/XBS-Nathan/nova/internal/lifecycle"` to the import block.

The previously-imported `docker.PHPServiceName` reference is now unused in this function but `docker` is still imported for `docker.Exec`. Leave it.

- [ ] **Step 2: Update `cmd/xdebug.go` for runtime-aware reload**

Replace the `RunE` body:

```go
RunE: func(cmd *cobra.Command, args []string) error {
    p, err := project.Detect()
    if err != nil {
        return err
    }

    version := p.Config.PHP
    iniDir := filepath.Join(config.GlobalDir(), "php", version, "conf.d")
    iniPath := filepath.Join(iniDir, "xdebug.ini")
    svc := lifecycle.PHPContainer(p.Config, p.Name)

    switch args[0] {
    case "on":
        fmt.Printf("Enabling Xdebug for PHP %s...\n", version)
        if err := os.MkdirAll(iniDir, 0755); err != nil {
            return fmt.Errorf("creating conf.d dir: %w", err)
        }
        ini := "zend_extension=xdebug\nxdebug.mode=debug\nxdebug.client_host=host.docker.internal\nxdebug.start_with_request=yes\n"
        if err := os.WriteFile(iniPath, []byte(ini), 0644); err != nil {
            return fmt.Errorf("writing xdebug.ini: %w", err)
        }
    case "off":
        fmt.Printf("Disabling Xdebug for PHP %s...\n", version)
        _ = os.Remove(iniPath) // may not exist
    default:
        return fmt.Errorf("usage: dev xdebug [on|off]")
    }

    fmt.Printf("  → Reloading PHP runtime...\n")
    switch p.Config.Runtime {
    case config.RuntimeFrankenPHP:
        if p.Config.Octane {
            // Octane reload re-execs PHP workers with the new ini.
            if err := docker.Exec(svc, "/srv", "php", "artisan", "octane:reload"); err != nil {
                return fmt.Errorf("reloading octane: %w", err)
            }
        } else {
            // Classic FrankenPHP: restart the container.
            cmdRestart := exec.Command("docker", "compose", "-f", docker.ComposeFile(), "restart", svc)
            cmdRestart.Stdout = os.Stdout
            cmdRestart.Stderr = os.Stderr
            if err := cmdRestart.Run(); err != nil {
                return fmt.Errorf("restarting frankenphp: %w", err)
            }
        }
    default:
        if err := docker.Exec(svc, "/srv", "kill", "-USR2", "1"); err != nil {
            return fmt.Errorf("reloading PHP-FPM: %w", err)
        }
    }

    fmt.Printf("✓ Xdebug %s for PHP %s\n", args[0], version)
    return nil
},
```

Update the imports at the top of the file. Add `"os/exec"` and `"github.com/XBS-Nathan/nova/internal/lifecycle"`.

- [ ] **Step 3: Update `cmd/build.go` to honor runtime**

Replace the `RunE` body in `cmd/build.go`:

```go
RunE: func(cmd *cobra.Command, args []string) error {
    p, err := project.Detect()
    if err != nil {
        return err
    }

    cfg := phpimage.ImageConfig{
        PHPVersion: p.Config.PHP,
        Extensions: p.Config.Extensions,
        Runtime:    p.Config.Runtime,
    }

    fmt.Printf("Building PHP %s (%s)...\n", p.Config.PHP, p.Config.Runtime)
    if err := phpimage.ForceBuild(cfg); err != nil {
        return err
    }

    fmt.Println("✓ PHP image built")
    return nil
},
```

Add `"github.com/XBS-Nathan/nova/internal/config"` import if not present (used transitively via `p.Config`, which is already imported via project).

- [ ] **Step 4: Update `cmd/services.go` to call the new Up signature**

In `servicesUpCmd`'s `RunE`, replace the call to `docker.Up` so it uses `ComposeOptions` with the new field shape (no FrankenPHP entries since this is shared-services-only). The existing call already builds a `ComposeOptions` literal — no changes needed beyond confirming nothing else in that file referenced the old `Service.Up` shape. If `Service.Up` is called anywhere here, change to:

```go
// (none expected — services.go uses docker.Up directly with ComposeOptions)
```

Run `go build ./cmd/...` after this step to confirm.

- [ ] **Step 5: Update `cmd/slow.go` `restartServices` to use new `lc.Start` signature**

Open `cmd/slow.go`. Find `restartServices` (after the snippet shown in research). Locate where it builds `[]docker.PHPVersion` and calls `lc.Start`. Update the call site to pass an empty (or single) FrankenPHP slice based on runtime:

```go
var php []docker.PHPVersion
var frankenphp []docker.FrankenPHPProject
if p.Config.Runtime == config.RuntimeFrankenPHP {
    workdir, err := containerWorkdir(global.ProjectsDir, p.Dir)
    if err != nil {
        return err
    }
    frankenphp = []docker.FrankenPHPProject{{
        Name:       p.Name,
        PHPVersion: p.Config.PHP,
        Extensions: p.Config.Extensions,
        Octane:     p.Config.Octane,
        Workdir:    workdir,
        Ports:      p.Config.Ports,
    }}
} else {
    php = []docker.PHPVersion{{
        Version:    p.Config.PHP,
        Extensions: p.Config.Extensions,
        Ports:      p.Config.Ports,
    }}
}
return lc.Start(p, php, frankenphp, true)
```

If the existing `restartServices` doesn't have a `global` and `p` in scope at that line, scroll up — they will be already, since the function detects the project and loads global config before calling `lc.Start`. Read the function to confirm.

- [ ] **Step 6: Build the cmd package**

Run: `go build ./cmd/...`
Expected: still likely failing on `start.go`, `restart.go`, `use.go` — those get fixed in the next task. Verify the failures are *only* in those files.

---

## Task 13: cmd — start, restart, use (active-project entry points)

**Files:**
- Modify: `cmd/start.go`
- Modify: `cmd/restart.go`
- Modify: `cmd/use.go`

- [ ] **Step 1: Add a small helper near the top of `cmd/start.go` to build the runtime payload from a project**

Add this function (near `nodeServiceForProject`):

```go
// runtimePayload returns the (php, frankenphp) slices to pass to lifecycle.Start
// based on the project's runtime configuration.
func runtimePayload(
    p *project.Project,
    global *config.GlobalConfig,
) ([]docker.PHPVersion, []docker.FrankenPHPProject, error) {
    if p.Config.Runtime == config.RuntimeFrankenPHP {
        rel, err := filepath.Rel(global.ProjectsDir, p.Dir)
        if err != nil {
            return nil, nil, fmt.Errorf("resolving project workdir: %w", err)
        }
        workdir := filepath.Join("/srv", rel)
        return nil, []docker.FrankenPHPProject{{
            Name:       p.Name,
            PHPVersion: p.Config.PHP,
            Extensions: p.Config.Extensions,
            Octane:     p.Config.Octane,
            Workdir:    workdir,
            Ports:      p.Config.Ports,
        }}, nil
    }
    return []docker.PHPVersion{{
        Version:    p.Config.PHP,
        Extensions: p.Config.Extensions,
        Ports:      p.Config.Ports,
    }}, nil, nil
}
```

- [ ] **Step 2: Update `startCmd` `RunE` to use it**

Replace the `phpimage.EnsureBuilt` and the `lc.Start` block:

```go
imgCfg := phpimage.ImageConfig{
    PHPVersion: p.Config.PHP,
    Extensions: p.Config.Extensions,
    Runtime:    p.Config.Runtime,
}
built, err := phpimage.EnsureBuilt(imgCfg)
if err != nil {
    return err
}

php, frankenphp, err := runtimePayload(p, global)
if err != nil {
    return err
}

lc := newLifecycle(global, p.Config)
return lc.Start(p, php, frankenphp, built)
```

- [ ] **Step 3: Remove `PHPService` field from `newLifecycle`**

The `Lifecycle` struct no longer has a `PHPService` field (Task 11). In `newLifecycle`, delete the `PHPService: docker.PHPServiceName,` line.

- [ ] **Step 4: Update `cmd/restart.go`**

Replace the body of `restartCmd.RunE`:

```go
RunE: func(cmd *cobra.Command, args []string) error {
    p, err := project.Detect()
    if err != nil {
        return err
    }
    global, err := config.LoadGlobal()
    if err != nil {
        return err
    }
    imgCfg := phpimage.ImageConfig{
        PHPVersion: p.Config.PHP,
        Extensions: p.Config.Extensions,
        Runtime:    p.Config.Runtime,
    }
    built, err := phpimage.EnsureBuilt(imgCfg)
    if err != nil {
        return err
    }
    lc := newLifecycle(global, p.Config)
    if err := lc.Stop(p); err != nil {
        return err
    }
    php, frankenphp, err := runtimePayload(p, global)
    if err != nil {
        return err
    }
    return lc.Start(p, php, frankenphp, built)
},
```

- [ ] **Step 5: Update `cmd/use.go`**

`cmd/use.go` builds an `ImageConfig` and calls `Service.Up` directly. Two changes:

1. Add `Runtime: p.Config.Runtime` to the `phpimage.ImageConfig{...}` literal.
2. Change the `svc.Up(php, false)` call to `svc.Up(php, nil, false)` (use is a PHP-version switch; FrankenPHP is per-project, so it stays empty here — the active project's full lifecycle goes through `nova start`/`restart`).

- [ ] **Step 6: Update `cmd/slow.go` if it builds an `ImageConfig`**

Search `cmd/slow.go` for `phpimage.ImageConfig`. If found, add `Runtime: p.Config.Runtime`.

Run: `grep -n "phpimage.ImageConfig" cmd/*.go`
For each match in `cmd/`, ensure the literal includes `Runtime: p.Config.Runtime`.

- [ ] **Step 7: Update `workerServicesForProject` in `cmd/start.go`**

The worker container reuses the project's PHP image. Make it runtime-aware:

```go
image := phpimage.ImageTag(phpimage.ImageConfig{
    PHPVersion: p.Config.PHP,
    Extensions: p.Config.Extensions,
    Runtime:    p.Config.Runtime,
})
```

(Just add the `Runtime` field; the call already exists.)

- [ ] **Step 8: Build everything**

Run: `go build ./...`
Expected: PASS.

- [ ] **Step 9: Run the full test suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 10: Run `go vet`**

Run: `go vet ./...`
Expected: no diagnostics.

- [ ] **Step 11: Commit**

```bash
git add cmd/
git commit -m "feat(cmd): plumb runtime through start, restart, use, and exec helpers"
```

---

## Task 14: docs and README — document the new option

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add a section to `README.md` describing the runtime/octane options**

Look at the existing structure of `README.md` and find the project-config / `.nova.yaml` section. Add a subsection:

````markdown
### Runtime: PHP-FPM (default) or FrankenPHP

By default each project uses the shared PHP-FPM container. Opt into FrankenPHP per project:

```yaml
runtime: frankenphp   # default: fpm
octane: true          # optional, requires runtime: frankenphp
```

- `runtime: frankenphp` runs FrankenPHP in classic mode. The site is fronted by the shared Caddy via `reverse_proxy` to a per-project `<project>_frankenphp` container on port 8000.
- `octane: true` additionally runs `php artisan octane:start --server=frankenphp`, giving you Laravel Octane's persistent worker mode.
- All other PHP-related commands (`nova php`, `nova artisan`, `nova xdebug`, hooks, workers) auto-target the right container.
````

- [ ] **Step 2: Add a paragraph to `CLAUDE.md` under "Project Patterns"**

Append a bullet under "Project Patterns":

```markdown
- **PHP runtime selection** — `.nova.yaml` `runtime` (`fpm` | `frankenphp`) selects the per-project PHP runtime. FPM uses the shared container by PHP version; FrankenPHP runs a per-project container fronted by the shared Caddy via `reverse_proxy`. `octane: true` enables Laravel Octane worker mode (requires `runtime: frankenphp`). `lifecycle.PHPContainer(cfg, name)` resolves the right exec target everywhere.
```

- [ ] **Step 3: Commit**

```bash
git add README.md CLAUDE.md
git commit -m "docs: document runtime and octane options"
```

---

## Task 15: Manual verification

The integration test suite doesn't cover FrankenPHP. Run through this checklist on a real machine before declaring done. Each failure → file an issue, don't paper over it.

- [ ] **Build, vet, test still green**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all PASS.

- [ ] **Backwards compatibility — existing FPM project unchanged**

In an existing `.nova.yaml` project, run `nova restart`. Expected: site loads as before, no new behavior.

- [ ] **FrankenPHP classic mode**

In a Laravel project, set `.nova.yaml`:
```yaml
php: "8.3"
runtime: frankenphp
```
Run `nova restart`. Expected:
- `docker compose -f ~/.nova/docker-compose.yml ps` shows a `<project>_frankenphp` container.
- `docker logs <project>_frankenphp` shows FrankenPHP/Caddy startup, no Octane mention.
- `https://<project>.test/` loads.

- [ ] **FrankenPHP + Octane**

Add to `.nova.yaml`:
```yaml
octane: true
```
Run `nova restart`. Expected:
- `docker logs <project>_frankenphp` shows Octane worker startup.
- Hit the site twice; second request shows no fresh Laravel boot in logs.

- [ ] **Misconfiguration is rejected**

Set `octane: true` with `runtime: fpm` (or omit `runtime`). Run `nova start`. Expected: error mentioning "octane: true requires runtime: frankenphp".

- [ ] **`nova php`, `nova artisan`, `nova composer` work in FrankenPHP project**

Inside the FrankenPHP project, run `nova artisan tinker` (or any artisan command). Expected: lands in the `<project>_frankenphp` container.

- [ ] **`nova xdebug on` works for FPM, classic FrankenPHP, and Octane FrankenPHP**

Toggle each combination, hit a breakpoint via your IDE.

- [ ] **`nova logs <project>` shows FrankenPHP container logs**

Run `nova logs <project>`. Expected: streams from the project's FrankenPHP container.

- [ ] **Switching active project works**

Start project A (FPM), then `cd` to project B (FrankenPHP) and `nova start`. Expected: A's PHP container removed by `--remove-orphans`, B's FrankenPHP container running. A's Caddy site config remains linked but its upstream is unreachable until A is reactivated — same as the existing behavior when switching FPM PHP versions.

- [ ] **Final commit (if any cleanup landed)**

If manual verification surfaced any small fixups, commit each as its own `fix(...)` commit referencing what failed and how it was fixed.

---

## Self-review (run this before handing off)

Before declaring the plan complete:

1. **Spec coverage:** every section of `docs/superpowers/specs/2026-05-02-frankenphp-runtime-design.md` mapped to at least one task above? Architecture (Tasks 7–11), config (1–2), image (3–5), compose (7–9), caddy (6), aux commands (10, 12), tests (interspersed), manual verification (15). ✓
2. **Type consistency:** field/method names used in later tasks match earlier tasks. `Runtime`, `Octane`, `Upstream{Kind,Address}`, `FrankenPHPProject{Name,PHPVersion,Extensions,Octane,Workdir,Ports}`, `PHPContainer(cfg, name)` — consistent throughout. ✓
3. **No placeholders:** every code-change step shows the literal code; no "add error handling" or "TBD". ✓
4. **Build at every commit:** Tasks 7, 9, 10 do not commit because they leave the build broken. Tasks 1–6, 8, 11–15 each end at a buildable, tested state. ✓
