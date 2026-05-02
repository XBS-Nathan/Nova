package lifecycle

import (
	"testing"

	"github.com/XBS-Nathan/nova/internal/caddy"
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

func TestUpstreamFor_FPM(t *testing.T) {
	t.Parallel()
	cfg := &config.ProjectConfig{Runtime: config.RuntimeFPM}
	got := upstreamFor(cfg, "php82")
	want := caddy.Upstream{Kind: "fastcgi", Address: "php82:9000"}
	if got != want {
		t.Errorf("upstreamFor(fpm) = %#v, want %#v", got, want)
	}
}

func TestUpstreamFor_FrankenPHP(t *testing.T) {
	t.Parallel()
	cfg := &config.ProjectConfig{Runtime: config.RuntimeFrankenPHP}
	got := upstreamFor(cfg, "myapp_frankenphp")
	want := caddy.Upstream{Kind: "reverse_proxy", Address: "myapp_frankenphp:8000"}
	if got != want {
		t.Errorf("upstreamFor(frankenphp) = %#v, want %#v", got, want)
	}
}
