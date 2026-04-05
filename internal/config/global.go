package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const GlobalConfigFile = "config.yaml"

// GlobalConfig holds user-level configuration from ~/.dev/config.yaml.
type GlobalConfig struct {
	ProjectsDir string   `yaml:"projects_dir"`
	PHPVersions []string `yaml:"php_versions"`
}

// loadGlobal reads devDir/config.yaml and returns a GlobalConfig with defaults applied.
// It is intentionally unexported so tests can inject a temp directory.
func loadGlobal(devDir string) (*GlobalConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("determining home directory: %w", err)
	}

	cfg := &GlobalConfig{}

	path := filepath.Join(devDir, GlobalConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg.ProjectsDir = filepath.Join(home, "Projects")
			return cfg, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	// Expand ~/... in projects_dir.
	if strings.HasPrefix(cfg.ProjectsDir, "~/") {
		cfg.ProjectsDir = filepath.Join(home, cfg.ProjectsDir[2:])
	}

	// Default projects_dir when empty.
	if cfg.ProjectsDir == "" {
		cfg.ProjectsDir = filepath.Join(home, "Projects")
	}

	return cfg, nil
}

// LoadGlobal reads ~/.dev/config.yaml and returns a GlobalConfig with defaults applied.
func LoadGlobal() (*GlobalConfig, error) {
	return loadGlobal(GlobalDir())
}
