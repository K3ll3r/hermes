package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/deck"
	"github.com/spf13/cobra"
)

func uninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove MOTD hook from shell profiles",
		Long: `Removes the hermes MOTD one-liner from system shell profiles.
Called by platform installers (MSI, .deb, .pkg) during uninstall.
Safe to run multiple times.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runUninstall()
		},
	}
	return cmd
}

func runUninstall() error {
	switch runtime.GOOS {
	case "windows":
		return uninstallWindows()
	case "darwin":
		return uninstallDarwin()
	default:
		return uninstallLinux()
	}
}

func uninstallWindows() error {
	profileDir := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0")
	profilePath := filepath.Join(profileDir, "profile.ps1")
	return removeLinesWithMarker(profilePath, motdMarker)
}

func uninstallDarwin() error {
	if err := removeLinesWithMarker("/etc/zprofile", "hermes-profile.d"); err != nil {
		deck.Warningf("uninstall: zprofile: %v", err)
	}
	os.Remove("/etc/profile.d/hermes-motd.sh")
	deck.Infof("uninstall: removed /etc/profile.d/hermes-motd.sh")
	return nil
}

func uninstallLinux() error {
	os.Remove("/etc/profile.d/hermes-motd.sh")
	os.Remove("/etc/profile.d/hermes.sh")
	deck.Infof("uninstall: removed profile.d scripts")
	return nil
}

func removeLinesWithMarker(path, marker string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}

	lines := strings.Split(string(data), "\n")
	var kept []string
	removed := 0
	for _, line := range lines {
		if strings.Contains(line, marker) {
			removed++
			continue
		}
		kept = append(kept, line)
	}

	if removed == 0 {
		deck.Infof("uninstall: no marker %q found in %s", marker, path)
		return nil
	}

	result := strings.Join(kept, "\n")
	result = strings.TrimRight(result, "\n") + "\n"

	if err := os.WriteFile(path, []byte(result), 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	deck.Infof("uninstall: removed %d line(s) from %s", removed, path)
	return nil
}
