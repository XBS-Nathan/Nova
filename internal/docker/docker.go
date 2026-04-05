package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/XBS-Nathan/apex-flow-dev-cli/internal/config"
	"github.com/XBS-Nathan/apex-flow-dev-cli/internal/phpimage"
)

// ComposeFile returns the path to the shared docker-compose.yml.
func ComposeFile() string {
	return filepath.Join(config.GlobalDir(), "docker-compose.yml")
}

// PHPServiceName converts a PHP version like "8.2" to a service name like "php82".
func PHPServiceName(version string) string {
	return "php" + strings.ReplaceAll(version, ".", "")
}

// Up generates the compose file for the given PHP versions and starts services.
func Up(projectsDir string, phpVersions []string) error {
	content := generateCompose(projectsDir, phpVersions)
	if err := os.WriteFile(ComposeFile(), []byte(content), 0644); err != nil {
		return fmt.Errorf("writing compose file: %w", err)
	}
	return compose("up", "-d")
}

// Down stops shared Docker services.
func Down() error {
	return compose("down")
}

// Exec runs a command in a running service container.
func Exec(service, workdir string, args ...string) error {
	execArgs := append([]string{"exec", "-w", workdir, service}, args...)
	return compose(execArgs...)
}

// IsUp checks if shared services are running.
func IsUp() bool {
	cmd := exec.Command("docker", "compose", "-f", ComposeFile(), "ps", "--status", "running", "-q")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}

func compose(args ...string) error {
	fullArgs := append([]string{"compose", "-f", ComposeFile()}, args...)
	cmd := exec.Command("docker", fullArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose %s: %w", args[0], err)
	}
	return nil
}

func generateCompose(projectsDir string, phpVersions []string) string {
	globalDir := config.GlobalDir()
	var b strings.Builder

	b.WriteString("services:\n")

	// caddy service
	b.WriteString("  caddy:\n")
	b.WriteString("    image: caddy:2-alpine\n")
	b.WriteString("    restart: unless-stopped\n")
	b.WriteString("    ports:\n")
	b.WriteString("      - \"80:80\"\n")
	b.WriteString("      - \"443:443\"\n")
	b.WriteString("    volumes:\n")
	fmt.Fprintf(&b, "      - %s/caddy:/etc/caddy\n", globalDir)
	fmt.Fprintf(&b, "      - %s/caddy/data:/data\n", globalDir)
	fmt.Fprintf(&b, "      - %s:/srv\n", projectsDir)
	b.WriteString("    networks:\n")
	b.WriteString("      - dev\n")
	b.WriteString("\n")

	// PHP services
	for _, ver := range phpVersions {
		name := PHPServiceName(ver)
		image := phpimage.ImageTag(ver, nil)
		fmt.Fprintf(&b, "  %s:\n", name)
		fmt.Fprintf(&b, "    image: %s\n", image)
		b.WriteString("    restart: unless-stopped\n")
		b.WriteString("    volumes:\n")
		fmt.Fprintf(&b, "      - %s:/srv\n", projectsDir)
		fmt.Fprintf(&b, "      - %s/php/%s/conf.d:/usr/local/etc/php/conf.d\n", globalDir, ver)
		b.WriteString("    networks:\n")
		b.WriteString("      - dev\n")
		b.WriteString("\n")
	}

	// mysql service
	b.WriteString("  mysql:\n")
	b.WriteString("    image: mysql:8.0\n")
	b.WriteString("    restart: unless-stopped\n")
	b.WriteString("    ports:\n")
	b.WriteString("      - \"3306:3306\"\n")
	b.WriteString("    environment:\n")
	b.WriteString("      MYSQL_ROOT_PASSWORD: root\n")
	b.WriteString("    volumes:\n")
	b.WriteString("      - mysql_data:/var/lib/mysql\n")
	b.WriteString("    networks:\n")
	b.WriteString("      - dev\n")
	b.WriteString("\n")

	// redis service
	b.WriteString("  redis:\n")
	b.WriteString("    image: redis:8\n")
	b.WriteString("    restart: unless-stopped\n")
	b.WriteString("    ports:\n")
	b.WriteString("      - \"6379:6379\"\n")
	b.WriteString("    command: redis-server --appendonly yes\n")
	b.WriteString("    volumes:\n")
	b.WriteString("      - redis_data:/data\n")
	b.WriteString("    networks:\n")
	b.WriteString("      - dev\n")
	b.WriteString("\n")

	// typesense service
	b.WriteString("  typesense:\n")
	b.WriteString("    image: typesense/typesense:26.0\n")
	b.WriteString("    restart: unless-stopped\n")
	b.WriteString("    ports:\n")
	b.WriteString("      - \"8108:8108\"\n")
	b.WriteString("    environment:\n")
	b.WriteString("      TYPESENSE_API_KEY: dev\n")
	b.WriteString("    volumes:\n")
	b.WriteString("      - typesense_data:/data\n")
	b.WriteString("    networks:\n")
	b.WriteString("      - dev\n")
	b.WriteString("\n")

	// postgres service
	b.WriteString("  postgres:\n")
	b.WriteString("    image: postgres:15\n")
	b.WriteString("    restart: unless-stopped\n")
	b.WriteString("    ports:\n")
	b.WriteString("      - \"5432:5432\"\n")
	b.WriteString("    environment:\n")
	b.WriteString("      POSTGRES_USER: postgres\n")
	b.WriteString("      POSTGRES_PASSWORD: postgres\n")
	b.WriteString("      POSTGRES_DB: postgres\n")
	b.WriteString("    volumes:\n")
	b.WriteString("      - postgres_data:/var/lib/postgresql/data\n")
	b.WriteString("    networks:\n")
	b.WriteString("      - dev\n")
	b.WriteString("\n")

	// volumes
	b.WriteString("volumes:\n")
	b.WriteString("  mysql_data:\n")
	b.WriteString("  redis_data:\n")
	b.WriteString("  typesense_data:\n")
	b.WriteString("  postgres_data:\n")
	b.WriteString("\n")

	// networks
	b.WriteString("networks:\n")
	b.WriteString("  dev:\n")

	return b.String()
}
