//go:build !windows

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"syscall"

	"github.com/google/deck"
)

func isPrivileged() bool {
	return os.Getuid() == 0
}

func launchInUserSessions(exe string, args []string) {
	sessions, err := activeUserSessions()
	if err != nil {
		deck.Warningf("session launch: %v", err)
		return
	}

	for _, s := range sessions {
		if err := launchAsUser(s, exe, args); err != nil {
			deck.Warningf("session launch: uid %d (%s): %v", s.uid, s.username, err)
			continue
		}
		deck.Infof("session launch: started hermes serve as %s (uid %d)", s.username, s.uid)
	}
}

type userSession struct {
	uid      uint32
	gid      uint32
	username string
	home     string
}

func activeUserSessions() ([]userSession, error) {
	switch runtime.GOOS {
	case "linux":
		return activeSessionsLinux()
	case "darwin":
		return activeSessionsDarwin()
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// activeSessionsLinux finds users with active logind sessions by scanning
// /run/user/<uid> directories, which systemd-logind creates per active user.
// Returns empty (not error) on non-systemd systems where /run/user is absent.
func activeSessionsLinux() ([]userSession, error) {
	entries, err := os.ReadDir("/run/user")
	if os.IsNotExist(err) {
		deck.Infof("session launch: /run/user not found (non-systemd?), skipping")
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read /run/user: %w", err)
	}

	var sessions []userSession
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		uid, err := strconv.ParseUint(e.Name(), 10, 32)
		if err != nil || uid < 1000 {
			continue
		}
		s, err := resolveUser(uint32(uid))
		if err != nil {
			deck.Infof("session launch: skip uid %d: %v", uid, err)
			continue
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// activeSessionsDarwin finds users with home directories in /Users.
// This is a heuristic: unlike Linux /run/user, macOS has no single indicator
// of active GUI sessions accessible from a postinstall script. Users with
// uid < 500 and system accounts (Shared, dot-prefixed) are excluded.
func activeSessionsDarwin() ([]userSession, error) {
	entries, err := os.ReadDir("/Users")
	if err != nil {
		return nil, fmt.Errorf("read /Users: %w", err)
	}

	var sessions []userSession
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || name == "Shared" || name[0] == '.' {
			continue
		}
		u, err := user.Lookup(name)
		if err != nil {
			continue
		}
		uid, _ := strconv.ParseUint(u.Uid, 10, 32)
		gid, _ := strconv.ParseUint(u.Gid, 10, 32)
		if uid < 500 {
			continue
		}
		sessions = append(sessions, userSession{
			uid:      uint32(uid),
			gid:      uint32(gid),
			username: u.Username,
			home:     u.HomeDir,
		})
	}
	return sessions, nil
}

func resolveUser(uid uint32) (userSession, error) {
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return userSession{}, err
	}
	gid, _ := strconv.ParseUint(u.Gid, 10, 32)
	return userSession{
		uid:      uid,
		gid:      uint32(gid),
		username: u.Username,
		home:     u.HomeDir,
	}, nil
}

func launchAsUser(s userSession, exe string, args []string) error {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: s.uid,
			Gid: s.gid,
		},
		Setsid: true,
	}
	cmd.Dir = s.home
	cmd.Env = buildUserEnv(s)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	return cmd.Start()
}

func buildUserEnv(s userSession) []string {
	env := []string{
		"HOME=" + s.home,
		"USER=" + s.username,
		"LOGNAME=" + s.username,
		"SHELL=/bin/sh",
		"PATH=/usr/local/bin:/usr/bin:/bin",
	}

	switch runtime.GOOS {
	case "linux":
		env = append(env, fmt.Sprintf("XDG_RUNTIME_DIR=/run/user/%d", s.uid))
		env = append(env, fmt.Sprintf("DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/%d/bus", s.uid))
	}

	return env
}
