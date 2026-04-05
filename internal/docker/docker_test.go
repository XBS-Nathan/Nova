package docker

import (
	"strings"
	"testing"
)

func TestGenerateCompose_IncludesCaddy(t *testing.T) {
	got := generateCompose("/home/user/Projects", []string{"8.2"})
	if !strings.Contains(got, "caddy:") {
		t.Error("missing caddy service")
	}
	if !strings.Contains(got, "caddy:2-alpine") {
		t.Error("missing caddy image")
	}
}

func TestGenerateCompose_IncludesPHPVersions(t *testing.T) {
	got := generateCompose("/home/user/Projects", []string{"8.2", "8.3"})
	if !strings.Contains(got, "php82:") {
		t.Error("missing php82 service")
	}
	if !strings.Contains(got, "php83:") {
		t.Error("missing php83 service")
	}
}

func TestGenerateCompose_MountsProjectsDir(t *testing.T) {
	got := generateCompose("/home/user/Code", []string{"8.2"})
	if !strings.Contains(got, "/home/user/Code:/srv") {
		t.Error("missing projects dir mount")
	}
}

func TestGenerateCompose_IncludesDBServices(t *testing.T) {
	got := generateCompose("/home/user/Projects", []string{"8.2"})
	if !strings.Contains(got, "mysql:") {
		t.Error("missing mysql service")
	}
	if !strings.Contains(got, "redis:") {
		t.Error("missing redis service")
	}
}

func TestPHPServiceName(t *testing.T) {
	tests := []struct {
		version string
		want    string
	}{
		{"8.2", "php82"},
		{"8.3", "php83"},
		{"7.4", "php74"},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := PHPServiceName(tt.version)
			if got != tt.want {
				t.Errorf("PHPServiceName(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}
