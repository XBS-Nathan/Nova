package phpimage

import (
	"strings"
	"testing"
)

func baseCfg(t *testing.T, version string, extensions []string) ImageConfig {
	t.Helper()
	return ImageConfig{
		PHPVersion:     version,
		Extensions:     extensions,
		NodeVersion:    "22",
		PackageManager: "npm",
	}
}

func TestGenerateDockerfile_BaseExtensionsOnly(t *testing.T) {
	got := generateDockerfile(baseCfg(t, "8.2", nil))

	if !strings.Contains(got, "FROM php:8.2-fpm-alpine") {
		t.Error("missing base image")
	}
	if !strings.Contains(got, "pdo_mysql") {
		t.Error("missing base extension pdo_mysql")
	}
	if strings.Contains(got, "pecl install imagick") {
		t.Error("should not contain extra extensions")
	}
}

func TestGenerateDockerfile_WithExtraExtensions(t *testing.T) {
	got := generateDockerfile(baseCfg(t, "8.3", []string{"imagick", "swoole"}))

	if !strings.Contains(got, "FROM php:8.3-fpm-alpine") {
		t.Error("missing base image")
	}
	if !strings.Contains(got, "pecl install imagick swoole") {
		t.Errorf("missing extra extensions in:\n%s", got)
	}
}

func TestGenerateDockerfile_WithNativeExtensions(t *testing.T) {
	got := generateDockerfile(baseCfg(t, "8.2", []string{"gd", "zip", "intl", "exif"}))

	for _, dep := range []string{"libpng-dev", "libzip-dev", "icu-dev"} {
		if !strings.Contains(got, dep) {
			t.Errorf("missing build dep %q in:\n%s", dep, got)
		}
	}
	if !strings.Contains(got, "docker-php-ext-configure gd") {
		t.Error("missing gd configure step")
	}
	if !strings.Contains(got, "docker-php-ext-install "+baseExtensions+" gd zip intl exif") {
		t.Errorf("native extensions not in ext-install:\n%s", got)
	}
}

func TestGenerateDockerfile_MixedExtensions(t *testing.T) {
	got := generateDockerfile(baseCfg(t, "8.3", []string{"gd", "imagick"}))

	if !strings.Contains(got, "docker-php-ext-install "+baseExtensions+" gd") {
		t.Errorf("native gd not in ext-install:\n%s", got)
	}
	if !strings.Contains(got, "pecl install imagick") {
		t.Errorf("pecl imagick missing:\n%s", got)
	}
}

func TestGenerateDockerfile_IncludesNodeJS(t *testing.T) {
	got := generateDockerfile(baseCfg(t, "8.2", nil))

	if !strings.Contains(got, "nodejs") {
		t.Errorf("missing nodejs install:\n%s", got)
	}
	if !strings.Contains(got, "npm") {
		t.Errorf("missing npm:\n%s", got)
	}
}

func TestGenerateDockerfile_YarnPackageManager(t *testing.T) {
	cfg := baseCfg(t, "8.2", nil)
	cfg.PackageManager = "yarn"
	got := generateDockerfile(cfg)

	if !strings.Contains(got, "npm install -g yarn") {
		t.Errorf("missing yarn install:\n%s", got)
	}
}

func TestGenerateDockerfile_PnpmPackageManager(t *testing.T) {
	cfg := baseCfg(t, "8.2", nil)
	cfg.PackageManager = "pnpm"
	got := generateDockerfile(cfg)

	if !strings.Contains(got, "npm install -g pnpm") {
		t.Errorf("missing pnpm install:\n%s", got)
	}
}

func TestUnionExtensions(t *testing.T) {
	projects := [][]string{
		{"imagick", "swoole"},
		{"swoole", "mongodb"},
		nil,
	}

	got := unionExtensions(projects...)
	want := []string{"imagick", "mongodb", "swoole"}

	if len(got) != len(want) {
		t.Fatalf("unionExtensions() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("unionExtensions()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestImageHash(t *testing.T) {
	h1 := imageHash(ImageConfig{Extensions: []string{"imagick", "swoole"}, NodeVersion: "22", PackageManager: "npm"})
	h2 := imageHash(ImageConfig{Extensions: []string{"swoole", "imagick"}, NodeVersion: "22", PackageManager: "npm"})
	h3 := imageHash(ImageConfig{Extensions: []string{"imagick"}, NodeVersion: "22", PackageManager: "npm"})
	h4 := imageHash(ImageConfig{Extensions: []string{"imagick", "swoole"}, NodeVersion: "22", PackageManager: "yarn"})

	if h1 != h2 {
		t.Error("hash should be order-independent")
	}
	if h1 == h3 {
		t.Error("different extensions should produce different hashes")
	}
	if h1 == h4 {
		t.Error("different package managers should produce different hashes")
	}
	if len(h1) != 8 {
		t.Errorf("hash length = %d, want 8", len(h1))
	}
}
