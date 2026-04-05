package phpimage

import (
	"strings"
	"testing"
)

func TestGenerateDockerfile_BaseExtensionsOnly(t *testing.T) {
	got := generateDockerfile("8.2", nil)

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
	got := generateDockerfile("8.3", []string{"imagick", "swoole"})

	if !strings.Contains(got, "FROM php:8.3-fpm-alpine") {
		t.Error("missing base image")
	}
	if !strings.Contains(got, "pecl install imagick swoole") {
		t.Errorf("missing extra extensions in:\n%s", got)
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

func TestExtensionHash(t *testing.T) {
	h1 := extensionHash([]string{"imagick", "swoole"})
	h2 := extensionHash([]string{"swoole", "imagick"})
	h3 := extensionHash([]string{"imagick"})

	if h1 != h2 {
		t.Error("hash should be order-independent")
	}
	if h1 == h3 {
		t.Error("different extensions should produce different hashes")
	}
	if len(h1) != 8 {
		t.Errorf("hash length = %d, want 8", len(h1))
	}
}
