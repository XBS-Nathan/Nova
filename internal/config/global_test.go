package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadGlobal_DefaultsWhenNoFile(t *testing.T) {
	dir := t.TempDir()

	cfg, err := loadGlobal(dir)
	if err != nil {
		t.Fatalf("loadGlobal() error = %v", err)
	}

	home, _ := os.UserHomeDir()
	wantProjectsDir := filepath.Join(home, "Projects")

	if cfg.ProjectsDir != wantProjectsDir {
		t.Errorf("ProjectsDir = %q, want %q", cfg.ProjectsDir, wantProjectsDir)
	}
	if len(cfg.PHPVersions) != 0 {
		t.Errorf("PHPVersions = %v, want empty", cfg.PHPVersions)
	}
	if cfg.Versions.MySQL != DefaultMySQLVersion {
		t.Errorf("Versions.MySQL = %q, want %q", cfg.Versions.MySQL, DefaultMySQLVersion)
	}
	if cfg.Versions.Redis != DefaultRedisVersion {
		t.Errorf("Versions.Redis = %q, want %q", cfg.Versions.Redis, DefaultRedisVersion)
	}
	if cfg.Versions.Postgres != "" {
		t.Errorf("Versions.Postgres = %q, want empty (set by db_driver)", cfg.Versions.Postgres)
	}
	if cfg.Versions.Mailpit != DefaultMailpitVersion {
		t.Errorf("Versions.Mailpit = %q, want %q", cfg.Versions.Mailpit, DefaultMailpitVersion)
	}
}

func TestLoadGlobal_ParsesConfigFile(t *testing.T) {
	dir := t.TempDir()
	content := `
projects_dir: /srv/projects
php_versions:
  - "8.1"
  - "8.2"
  - "8.3"
`
	if err := os.WriteFile(filepath.Join(dir, GlobalConfigFile), []byte(content), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	cfg, err := loadGlobal(dir)
	if err != nil {
		t.Fatalf("loadGlobal() error = %v", err)
	}

	if cfg.ProjectsDir != "/srv/projects" {
		t.Errorf("ProjectsDir = %q, want %q", cfg.ProjectsDir, "/srv/projects")
	}
	if len(cfg.PHPVersions) != 3 {
		t.Fatalf("PHPVersions length = %d, want 3", len(cfg.PHPVersions))
	}
	if cfg.PHPVersions[0] != "8.1" {
		t.Errorf("PHPVersions[0] = %q, want %q", cfg.PHPVersions[0], "8.1")
	}
	if cfg.PHPVersions[1] != "8.2" {
		t.Errorf("PHPVersions[1] = %q, want %q", cfg.PHPVersions[1], "8.2")
	}
	if cfg.PHPVersions[2] != "8.3" {
		t.Errorf("PHPVersions[2] = %q, want %q", cfg.PHPVersions[2], "8.3")
	}
}

func TestLoadGlobal_ParsesServiceVersions(t *testing.T) {
	dir := t.TempDir()
	content := `
projects_dir: /srv/projects
versions:
  mysql: "9.0"
  redis: "7"
  postgres: "16"
  mailpit: "v1.21"
`
	if err := os.WriteFile(filepath.Join(dir, GlobalConfigFile), []byte(content), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	cfg, err := loadGlobal(dir)
	if err != nil {
		t.Fatalf("loadGlobal() error = %v", err)
	}

	if cfg.Versions.MySQL != "9.0" {
		t.Errorf("Versions.MySQL = %q, want %q", cfg.Versions.MySQL, "9.0")
	}
	if cfg.Versions.Redis != "7" {
		t.Errorf("Versions.Redis = %q, want %q", cfg.Versions.Redis, "7")
	}
	if cfg.Versions.Postgres != "16" {
		t.Errorf("Versions.Postgres = %q, want %q", cfg.Versions.Postgres, "16")
	}
	if cfg.Versions.Mailpit != "v1.21" {
		t.Errorf("Versions.Mailpit = %q, want %q", cfg.Versions.Mailpit, "v1.21")
	}
}

func TestLoadGlobal_ExpandsTilde(t *testing.T) {
	dir := t.TempDir()
	content := "projects_dir: ~/MyProjects\n"
	if err := os.WriteFile(filepath.Join(dir, GlobalConfigFile), []byte(content), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	cfg, err := loadGlobal(dir)
	if err != nil {
		t.Fatalf("loadGlobal() error = %v", err)
	}

	home, _ := os.UserHomeDir()
	wantProjectsDir := filepath.Join(home, "MyProjects")

	if strings.HasPrefix(cfg.ProjectsDir, "~/") {
		t.Errorf("ProjectsDir = %q, tilde was not expanded", cfg.ProjectsDir)
	}
	if cfg.ProjectsDir != wantProjectsDir {
		t.Errorf("ProjectsDir = %q, want %q", cfg.ProjectsDir, wantProjectsDir)
	}
}

func TestLoadGlobal_ParsesPhpIni(t *testing.T) {
	dir := t.TempDir()
	content := `
php_ini:
  memory_limit: 1G
  display_errors: "Off"
`
	os.WriteFile(filepath.Join(dir, GlobalConfigFile), []byte(content), 0644)

	cfg, err := loadGlobal(dir)
	if err != nil {
		t.Fatalf("loadGlobal() error = %v", err)
	}

	if cfg.PhpIni["memory_limit"] != "1G" {
		t.Errorf("PhpIni[memory_limit] = %q, want %q", cfg.PhpIni["memory_limit"], "1G")
	}
	if cfg.PhpIni["display_errors"] != "Off" {
		t.Errorf("PhpIni[display_errors] = %q, want %q", cfg.PhpIni["display_errors"], "Off")
	}
}

func TestLoadGlobal_ParsesMysqlCnf(t *testing.T) {
	dir := t.TempDir()
	content := `
mysql_cnf:
  innodb_buffer_pool_size: 256M
`
	os.WriteFile(filepath.Join(dir, GlobalConfigFile), []byte(content), 0644)

	cfg, err := loadGlobal(dir)
	if err != nil {
		t.Fatalf("loadGlobal() error = %v", err)
	}

	if cfg.MysqlCnf["innodb_buffer_pool_size"] != "256M" {
		t.Errorf("MysqlCnf[innodb_buffer_pool_size] = %q, want %q", cfg.MysqlCnf["innodb_buffer_pool_size"], "256M")
	}
}
