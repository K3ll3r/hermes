package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/TsekNet/hermes/internal/client"
	"github.com/TsekNet/hermes/internal/server"
	"github.com/google/deck"
	"github.com/spf13/cobra"
)

func stopCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the running hermes service daemon",
		Long: `Sends a graceful Shutdown RPC to the hermes daemon. Falls back to
process-level signals if the gRPC call fails.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runStop(port)
		},
	}
	cmd.Flags().IntVar(&port, "port", server.DefaultPort, "gRPC port to find the daemon on")
	return cmd
}

func runStop(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port: %d (must be 1-65535)", port)
	}

	if stopped := tryGracefulShutdown(port); stopped {
		return nil
	}

	pid, err := findListenerPID(port)
	if err != nil {
		return fmt.Errorf("no hermes daemon found on port %d: %w", port, err)
	}

	deck.Infof("found hermes daemon pid %d on port %d", pid, port)

	if err := killProcess(pid); err != nil {
		return fmt.Errorf("stop pid %d: %w", pid, err)
	}

	if waitForExit(port, 5*time.Second) {
		fmt.Fprintf(os.Stderr, "hermes daemon (pid %d) stopped\n", pid)
		return nil
	}
	return fmt.Errorf("daemon pid %d did not exit within 5s", pid)
}

func tryGracefulShutdown(port int) bool {
	c, err := client.Dial(port)
	if err != nil {
		return false
	}
	defer c.Close()

	deck.Infof("sending Shutdown RPC to port %d", port)
	if err := c.Shutdown(context.Background()); err != nil {
		deck.Warningf("Shutdown RPC failed: %v, falling back to process kill", err)
		return false
	}
	if waitForExit(port, 5*time.Second) {
		fmt.Fprintf(os.Stderr, "hermes daemon stopped (graceful shutdown)\n")
		return true
	}
	deck.Warningf("daemon did not exit after Shutdown RPC, falling back to process kill")
	return false
}

func waitForExit(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err != nil {
			return true
		}
		conn.Close()
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

func findListenerPID(port int) (int, error) {
	switch runtime.GOOS {
	case "windows":
		return findListenerWindows(port)
	default:
		return findListenerUnix(port)
	}
}

func findListenerUnix(port int) (int, error) {
	// lsof is available on macOS and most Linux distros.
	out, err := exec.Command("lsof", "-ti", fmt.Sprintf("tcp:%d", port)).Output()
	if err != nil {
		// Fallback: try ss (Linux).
		out, err = exec.Command("ss", "-tlnp", fmt.Sprintf("sport = :%d", port)).Output()
		if err != nil {
			return 0, fmt.Errorf("cannot find listener: lsof and ss both failed")
		}
		return parseSS(string(out))
	}
	s := strings.TrimSpace(string(out))
	lines := strings.Split(s, "\n")
	return strconv.Atoi(strings.TrimSpace(lines[0]))
}

func parseSS(output string) (int, error) {
	for _, line := range strings.Split(output, "\n") {
		if idx := strings.Index(line, "pid="); idx >= 0 {
			rest := line[idx+4:]
			if comma := strings.IndexByte(rest, ','); comma > 0 {
				rest = rest[:comma]
			}
			return strconv.Atoi(strings.TrimSpace(rest))
		}
	}
	return 0, fmt.Errorf("no pid found in ss output")
}

func findListenerWindows(port int) (int, error) {
	out, err := exec.Command("netstat", "-ano", "-p", "TCP").Output()
	if err != nil {
		return 0, fmt.Errorf("netstat: %w", err)
	}
	target := fmt.Sprintf(":%d", port)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		if strings.HasSuffix(fields[1], target) && strings.EqualFold(fields[3], "LISTENING") {
			return strconv.Atoi(fields[4])
		}
	}
	return 0, fmt.Errorf("no listener on port %d", port)
}

func killProcess(pid int) error {
	if runtime.GOOS == "windows" {
		return exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid)).Run()
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(os.Interrupt)
}
