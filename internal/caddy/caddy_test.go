package caddy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateSiteConfig(t *testing.T) {
	got := generateSiteConfig("myproject", "/srv/myproject/public",
		Upstream{Kind: "fastcgi", Address: "php82:9000"}, nil)

	if !strings.Contains(got, "myproject.test {") {
		t.Error("missing site domain")
	}
	if !strings.Contains(got, "/srv/myproject/public") {
		t.Error("missing docroot")
	}
	if !strings.Contains(got, "php_fastcgi php82:9000") {
		t.Error("missing php_fastcgi directive")
	}
}

func TestGenerateMainCaddyfile(t *testing.T) {
	got := generateMainCaddyfile()

	if !strings.Contains(got, "local_certs") {
		t.Error("missing local_certs directive")
	}
	if !strings.Contains(got, "import /etc/caddy/sites/*.caddy") {
		t.Error("missing import directive")
	}
}

func TestWriteSiteConfig(t *testing.T) {
	dir := t.TempDir()
	sitesDir := filepath.Join(dir, "sites")
	os.MkdirAll(sitesDir, 0755)

	err := writeSiteConfig(dir, "myproject", "/srv/myproject/public",
		Upstream{Kind: "fastcgi", Address: "php82:9000"}, nil)

	if err != nil {
		t.Fatalf("writeSiteConfig() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(sitesDir, "myproject.caddy"))
	if err != nil {
		t.Fatalf("reading site config: %v", err)
	}
	if !strings.Contains(string(data), "myproject.test") {
		t.Error("site config missing domain")
	}
}

func TestRemoveSiteConfig(t *testing.T) {
	dir := t.TempDir()
	sitesDir := filepath.Join(dir, "sites")
	os.MkdirAll(sitesDir, 0755)

	path := filepath.Join(sitesDir, "myproject.caddy")
	os.WriteFile(path, []byte("test"), 0644)

	removeSiteConfig(dir, "myproject")

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("site config should be removed")
	}
}

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
	for _, unwant := range []string{"root * ", "php_fastcgi", "file_server", "encode gzip"} {
		if strings.Contains(got, unwant) {
			t.Errorf("reverse_proxy site should not contain %q, got:\n%s", unwant, got)
		}
	}
}
