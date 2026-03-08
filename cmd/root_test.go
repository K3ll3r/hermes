package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/TsekNet/hermes/internal/config"
)

func TestResolveConfig_NoInput(t *testing.T) {
	t.Parallel()
	cfg, err := resolveConfig("", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config for no input, got heading=%q", cfg.Heading)
	}
}

func TestResolveConfig_NoArgs_TerminalStdin(t *testing.T) {
	t.Parallel()
	// Simulate: no --config, no positional args, stdin is a terminal (char device).
	// On CI and in tests, stdin is typically /dev/null or NUL, not a char device,
	// so resolveConfig should still return (nil, nil).
	cfg, err := resolveConfig("", []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config, got heading=%q", cfg.Heading)
	}
}

func TestResolveConfig_InlineJSON(t *testing.T) {
	t.Parallel()
	cfg, err := resolveConfig("", []string{`{"heading":"Test","message":"Body"}`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config from inline JSON")
	}
	if cfg.Heading != "Test" {
		t.Errorf("heading = %q, want %q", cfg.Heading, "Test")
	}
}

func TestResolveConfig_FilePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "test.json")
	os.WriteFile(f, []byte(`{"heading":"FromFile","message":"M"}`), 0644)

	cfg, err := resolveConfig(f, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil || cfg.Heading != "FromFile" {
		t.Errorf("heading = %v, want FromFile", cfg)
	}
}

func TestDemoConfig_DND(t *testing.T) {
	t.Parallel()
	cfg := demoConfig()
	if cfg.DND != config.DNDIgnore {
		t.Errorf("dnd = %q, want %q: demo must ignore DND to prevent silent hang", cfg.DND, config.DNDIgnore)
	}
}

func TestDemoConfig_Valid(t *testing.T) {
	t.Parallel()
	cfg := demoConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("demoConfig is invalid: %v", err)
	}
	if cfg.Heading == "" {
		t.Error("heading is empty")
	}
	if len(cfg.Buttons) == 0 {
		t.Error("no buttons")
	}
	if cfg.TimeoutSeconds <= 0 {
		t.Error("timeout must be positive")
	}
}

func TestDemoConfig_DefaultsApplied(t *testing.T) {
	t.Parallel()
	cfg := demoConfig()
	if cfg.Title == "" {
		t.Error("title should be set by ApplyDefaults")
	}
}

func TestWaitForDND_IgnoreReturnsImmediately(t *testing.T) {
	t.Parallel()
	cfg := &config.NotificationConfig{DND: config.DNDIgnore}
	if err := waitForDND(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunUI_WebView2DirCreated(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("WebView2 data path only applies on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	expected := filepath.Join(dir, "hermes", "webview2-local")
	wv2Path := webview2DataPath()
	if wv2Path != expected {
		t.Fatalf("wv2Path = %q, want %q", wv2Path, expected)
	}

	info, err := os.Stat(wv2Path)
	if err != nil {
		t.Fatalf("WebView2 data dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("WebView2 data path is not a directory")
	}
}

func TestPrepareConfig_SetsDefaults(t *testing.T) {
	t.Parallel()
	cfg := &config.NotificationConfig{Heading: "H", Message: "M"}
	prepareConfig(cfg)
	if cfg.Title == "" {
		t.Error("title should be set by ApplyDefaults")
	}
	if cfg.DND == "" {
		t.Error("dnd should be set by ApplyDefaults")
	}
}

func TestDoubleClickPath_ReachesDemo(t *testing.T) {
	t.Parallel()

	// Simulate the exact double-click scenario: no flags, no args, no stdin pipe.
	// resolveConfig must return (nil, nil), causing runRoot to call runDemo.
	cfg, err := resolveConfig("", nil)
	if err != nil {
		t.Fatalf("resolveConfig error: %v", err)
	}
	if cfg != nil {
		t.Fatal("resolveConfig should return nil for double-click (no input)")
	}

	// demoConfig must produce a valid, DND-ignoring config.
	demo := demoConfig()
	if demo.DND != config.DNDIgnore {
		t.Errorf("demo dnd = %q, want %q", demo.DND, config.DNDIgnore)
	}
	if err := demo.Validate(); err != nil {
		t.Fatalf("demo config invalid: %v", err)
	}

	// waitForDND must return immediately for ignore mode.
	if err := waitForDND(demo); err != nil {
		t.Fatalf("waitForDND error: %v", err)
	}
}

func TestLoadFromArg_InvalidPath(t *testing.T) {
	t.Parallel()
	badPath := filepath.Join(t.TempDir(), "nonexistent", "config.json")
	_, err := loadFromArg(badPath)
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
	if !strings.Contains(err.Error(), "not a file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWebView2DataPath_NonWindowsReturnsEmpty(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test only applies on non-Windows")
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if p := webview2DataPath(); p != "" {
		t.Errorf("expected empty string on %s, got %q", runtime.GOOS, p)
	}
}
