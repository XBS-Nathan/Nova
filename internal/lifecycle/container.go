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
