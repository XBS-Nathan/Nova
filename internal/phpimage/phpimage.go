package phpimage

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/XBS-Nathan/apex-flow-dev-cli/internal/config"
)

const baseExtensions = "pdo_mysql pdo_pgsql opcache pcntl bcmath"

// EnsureBuilt builds the PHP-FPM image for a version if the extension set has changed.
func EnsureBuilt(version string, extensions []string) error {
	tag := ImageTag(version, extensions)

	cmd := exec.Command("docker", "image", "inspect", tag)
	if cmd.Run() == nil {
		return nil // already built
	}

	dir, err := writeDockerfile(version, extensions)
	if err != nil {
		return err
	}

	fmt.Printf("  → Building PHP %s image...\n", version)
	build := exec.Command("docker", "build", "-t", tag, dir)
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("building php %s image: %w", version, err)
	}

	return nil
}

// ImageTag returns the Docker image tag for a PHP version + extensions.
func ImageTag(version string, extensions []string) string {
	hash := extensionHash(extensions)
	return fmt.Sprintf("dev-php:%s-%s", version, hash)
}

func writeDockerfile(version string, extensions []string) (string, error) {
	dir := filepath.Join(config.GlobalDir(), "dockerfiles", "php", version)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating dockerfile dir: %w", err)
	}

	content := generateDockerfile(version, extensions)
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(content), 0644); err != nil {
		return "", fmt.Errorf("writing Dockerfile: %w", err)
	}

	phpIni := "[PHP]\nmemory_limit = 512M\nupload_max_filesize = 100M\npost_max_size = 100M\n"
	if err := os.WriteFile(filepath.Join(dir, "php.ini"), []byte(phpIni), 0644); err != nil {
		return "", fmt.Errorf("writing php.ini: %w", err)
	}

	return dir, nil
}

func generateDockerfile(version string, extensions []string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "FROM php:%s-fpm-alpine\n\n", version)
	fmt.Fprintf(&b, "RUN apk add --no-cache linux-headers $PHPIZE_DEPS \\\n")
	fmt.Fprintf(&b, "    && docker-php-ext-install %s \\\n", baseExtensions)
	fmt.Fprintf(&b, "    && pecl install redis xdebug \\\n")
	fmt.Fprintf(&b, "    && docker-php-ext-enable redis \\\n")

	if len(extensions) > 0 {
		fmt.Fprintf(&b, "    && pecl install %s \\\n", strings.Join(extensions, " "))
		fmt.Fprintf(&b, "    && docker-php-ext-enable %s \\\n", strings.Join(extensions, " "))
	}

	fmt.Fprintf(&b, "    && apk del $PHPIZE_DEPS\n\n")
	fmt.Fprintf(&b, "COPY php.ini /usr/local/etc/php/php.ini\n\n")
	fmt.Fprintf(&b, "WORKDIR /srv\n")

	return b.String()
}

func unionExtensions(lists ...[]string) []string {
	seen := make(map[string]bool)
	for _, list := range lists {
		for _, ext := range list {
			seen[ext] = true
		}
	}

	result := make([]string, 0, len(seen))
	for ext := range seen {
		result = append(result, ext)
	}
	sort.Strings(result)
	return result
}

func extensionHash(extensions []string) string {
	sorted := make([]string, len(extensions))
	copy(sorted, extensions)
	sort.Strings(sorted)

	h := sha256.Sum256([]byte(strings.Join(sorted, ",")))
	return fmt.Sprintf("%x", h[:4])
}
