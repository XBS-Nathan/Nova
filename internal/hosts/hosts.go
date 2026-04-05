package hosts

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// hasEntry checks if the hosts file at path already contains the entry.
func hasEntry(path, domain string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read hosts file %s: %w", path, err)
	}
	entry := fmt.Sprintf("127.0.0.1 %s", domain)
	return strings.Contains(string(data), entry), nil
}

// ensureEntry appends "127.0.0.1 <domain>" if not already present.
// Testable core — writes directly without sudo.
func ensureEntry(path, domain string) error {
	exists, err := hasEntry(path, domain)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open hosts file for append: %w", err)
	}
	defer f.Close()

	entry := fmt.Sprintf("127.0.0.1 %s\n", domain)
	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("append to hosts file: %w", err)
	}

	return nil
}

// ensureWithSudo appends via sudo tee if entry is missing.
func ensureWithSudo(path, domain, description string) error {
	exists, err := hasEntry(path, domain)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	fmt.Printf("    Adding %s to %s (requires sudo)\n", domain, description)
	entry := fmt.Sprintf("127.0.0.1 %s\n", domain)
	cmd := exec.Command("sudo", "tee", "-a", path)
	cmd.Stdin = strings.NewReader(entry)
	cmd.Stderr = os.Stderr
	cmd.Stdout = nil // suppress tee's stdout echo
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("adding %s to %s: %w", domain, description, err)
	}

	return nil
}

// IsWSL2 reports whether the current environment is WSL2.
func IsWSL2() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

// Ensure ensures that "127.0.0.1 <domain>" exists in /etc/hosts.
// On WSL2 it also writes to the Windows hosts file.
func Ensure(domain string) error {
	const linuxHosts = "/etc/hosts"

	if err := ensureWithSudo(linuxHosts, domain, "Linux hosts"); err != nil {
		return err
	}

	if IsWSL2() {
		const windowsHosts = "/mnt/c/Windows/System32/drivers/etc/hosts"
		if err := ensureWindowsHosts(windowsHosts, domain); err != nil {
			// Non-fatal — Windows hosts is nice-to-have
			fmt.Printf("    ! Could not update Windows hosts: %s\n", err)
			fmt.Printf("    Add manually: 127.0.0.1 %s\n", domain)
		}
	}

	return nil
}

// ensureWindowsHosts writes to the Windows hosts file by invoking
// an elevated PowerShell process via cmd.exe. This triggers a Windows
// UAC prompt since WSL2 sudo can't write to Windows-protected files.
func ensureWindowsHosts(wslPath, domain string) error {
	exists, err := hasEntry(wslPath, domain)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	fmt.Printf("    Adding %s to Windows hosts (UAC prompt may appear)\n", domain)

	// Use cmd.exe to launch an elevated PowerShell that appends the entry.
	// The Windows hosts path must be in Windows format.
	winPath := `C:\Windows\System32\drivers\etc\hosts`
	entry := fmt.Sprintf("127.0.0.1 %s", domain)
	psCommand := fmt.Sprintf(
		"Add-Content -Path '%s' -Value '%s'",
		winPath, entry,
	)

	cmd := exec.Command("powershell.exe",
		"-NoProfile", "-NonInteractive", "-Command",
		fmt.Sprintf(
			"Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-Command','%s'",
			strings.ReplaceAll(psCommand, "'", "''"),
		),
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("elevated powershell: %w", err)
	}
	return nil
}

// Service is an adapter that satisfies the lifecycle.HostsService interface.
type Service struct{}

// Ensure delegates to the package-level Ensure function.
func (Service) Ensure(domain string) error { return Ensure(domain) }
