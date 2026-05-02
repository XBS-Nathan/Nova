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
