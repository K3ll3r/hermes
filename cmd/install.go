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

const motdMarker = "# Hermes-MOTD"

func installCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Configure MOTD hook for SSH login banners",
		Long: `Writes a one-liner to the system shell profile so that 'hermes motd'
runs on SSH login. Called by platform installers (MSI, .deb, .pkg)
after files are placed on disk. Safe to run multiple times.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runInstall()
		},
	}
	return cmd
}

func runInstall() error {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = installWindows()
	case "darwin":
		err = installDarwin()
	default:
		err = installLinux()
	}
	if err != nil {
		return err
	}

	if isPrivileged() {
		exe, exeErr := os.Executable()
		if exeErr != nil {
			deck.Warningf("install: cannot resolve executable path: %v", exeErr)
			return nil
		}
		args := []string{"serve"}
		if runtime.GOOS == "windows" {
			args = append(args, "--startup-delay", "5")
		}
		deck.Infof("install: launching hermes serve in active user sessions")
		launchInUserSessions(exe, args)
	}

	return nil
}

func installWindows() error {
	profileDir := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0")
	profilePath := filepath.Join(profileDir, "profile.ps1")

	hook := fmt.Sprintf(
		`if (Test-Path "$env:ProgramFiles\Hermes\hermes.exe") { & "$env:ProgramFiles\Hermes\hermes.exe" motd } %s`,
		motdMarker,
	)

	return appendLineIfMissing(profilePath, hook, motdMarker)
}

func installDarwin() error {
	zprofile := "/etc/zprofile"
	profiledMarker := "# hermes-profile.d"
	snippet := fmt.Sprintf(
		`if [ -d /etc/profile.d ]; then for f in /etc/profile.d/*.sh; do [ -r "$f" ] && . "$f"; done; fi %s`,
		profiledMarker,
	)

	if err := appendLineIfMissing(zprofile, snippet, profiledMarker); err != nil {
		return fmt.Errorf("zprofile: %w", err)
	}

	motdPath := "/etc/profile.d/hermes-motd.sh"
	motdLine := fmt.Sprintf("command -v hermes >/dev/null 2>&1 && hermes motd %s", motdMarker)
	return writeFileIfMissing(motdPath, motdLine+"\n", 0644)
}

func installLinux() error {
	motdPath := "/etc/profile.d/hermes-motd.sh"
	motdLine := fmt.Sprintf("command -v hermes >/dev/null 2>&1 && hermes motd %s", motdMarker)
	return writeFileIfMissing(motdPath, motdLine+"\n", 0644)
}

func appendLineIfMissing(path, line, marker string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if strings.Contains(string(data), marker) {
		deck.Infof("install: marker %q already present in %s", marker, path)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		fmt.Fprintln(f)
	}
	fmt.Fprintln(f, line)
	deck.Infof("install: wrote MOTD hook to %s", path)
	return nil
}

func writeFileIfMissing(path, content string, perm os.FileMode) error {
	data, err := os.ReadFile(path)
	if err == nil && strings.Contains(string(data), motdMarker) {
		deck.Infof("install: %s already configured", path)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	deck.Infof("install: wrote %s", path)
	return nil
}
