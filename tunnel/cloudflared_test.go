package tunnel

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func testConfig(dir string) Config {
	return Config{
		Domain:     "example.com",
		Name:       "roji",
		Port:       8080,
		ConfigPath: filepath.Join(dir, "cloudflared.yml"),
	}
}

func TestRenderConfig(t *testing.T) {
	rendered := RenderConfig(testConfig(t.TempDir()))

	var parsed struct {
		Tunnel  string `yaml:"tunnel"`
		Ingress []struct {
			Hostname string `yaml:"hostname"`
			Service  string `yaml:"service"`
		} `yaml:"ingress"`
	}
	if err := yaml.Unmarshal([]byte(rendered), &parsed); err != nil {
		t.Fatalf("generated configuration is not valid YAML: %v\n%s", err, rendered)
	}

	if parsed.Tunnel != "roji" {
		t.Errorf("tunnel = %q, want roji", parsed.Tunnel)
	}

	// One catch-all rule plus the terminating one. A rule per route would mean
	// restarting cloudflared, which has no way to reload, every time a
	// container starts or stops.
	if len(parsed.Ingress) != 2 {
		t.Fatalf("ingress has %d rules, want 2:\n%s", len(parsed.Ingress), rendered)
	}
	if parsed.Ingress[0].Hostname != "*.example.com" {
		t.Errorf("ingress hostname = %q, want *.example.com", parsed.Ingress[0].Hostname)
	}
	if parsed.Ingress[0].Service != "http://127.0.0.1:8080" {
		t.Errorf("ingress service = %q, want http://127.0.0.1:8080", parsed.Ingress[0].Service)
	}
	// cloudflared refuses to start without a final catch-all rule.
	if parsed.Ingress[1].Hostname != "" || parsed.Ingress[1].Service != "http_status:404" {
		t.Errorf("last rule = %+v, want a bare http_status:404", parsed.Ingress[1])
	}
}

func TestRenderConfigPortVaries(t *testing.T) {
	cfg := testConfig(t.TempDir())
	cfg.Port = 9999

	if got := RenderConfig(cfg); !strings.Contains(got, "http://127.0.0.1:9999") {
		t.Errorf("generated configuration does not name the configured port:\n%s", got)
	}
}

func TestWriteConfig(t *testing.T) {
	// A directory that does not exist yet: the data directory is created on
	// demand elsewhere too, and a first run should not need it prepared.
	cfg := testConfig(filepath.Join(t.TempDir(), "data"))

	if err := WriteConfig(cfg); err != nil {
		t.Fatalf("WriteConfig() error: %v", err)
	}

	written, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		t.Fatalf("failed to read back the configuration: %v", err)
	}
	if string(written) != RenderConfig(cfg) {
		t.Error("the written file does not match RenderConfig")
	}
}

func TestWriteConfigOverwrites(t *testing.T) {
	cfg := testConfig(t.TempDir())

	if err := os.WriteFile(cfg.ConfigPath, []byte("stale: true\n"), 0644); err != nil {
		t.Fatalf("failed to seed a stale file: %v", err)
	}
	if err := WriteConfig(cfg); err != nil {
		t.Fatalf("WriteConfig() error: %v", err)
	}

	written, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		t.Fatalf("failed to read back the configuration: %v", err)
	}
	if strings.Contains(string(written), "stale") {
		t.Errorf("the stale configuration survived:\n%s", written)
	}
}

// fakeCloudflared puts a stand-in for cloudflared on PATH: a process that
// records the arguments it was given, then waits to be signalled.
func fakeCloudflared(t *testing.T) (argsFile string) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}

	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + argsFile + "\n" +
		"trap 'exit 0' INT TERM\n" +
		"while true; do sleep 0.05; done\n"

	if err := os.WriteFile(filepath.Join(dir, Binary), []byte(script), 0755); err != nil {
		t.Fatalf("failed to write the stand-in: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return argsFile
}

func TestRunnerStartStop(t *testing.T) {
	argsFile := fakeCloudflared(t)
	cfg := testConfig(t.TempDir())
	runner := NewRunner(cfg)

	if err := runner.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	t.Cleanup(runner.Stop)

	// Starting writes the configuration cloudflared is told to read.
	if _, err := os.Stat(cfg.ConfigPath); err != nil {
		t.Errorf("Start() did not write the configuration: %v", err)
	}

	// The stand-in writes its arguments before waiting; give it a moment.
	var args string
	for range 100 {
		if b, err := os.ReadFile(argsFile); err == nil && len(b) > 0 {
			args = string(b)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, want := range []string{"tunnel", "run", cfg.Name, cfg.ConfigPath, "--no-autoupdate"} {
		if !strings.Contains(args, want) {
			t.Errorf("cloudflared was not passed %q; got:\n%s", want, args)
		}
	}

	if err := runner.Start(); err == nil {
		t.Error("Start() on a running tunnel should report it is already running")
	}

	runner.Stop()

	// Stopping clears the process, so the tunnel can be started again.
	if err := runner.Start(); err != nil {
		t.Errorf("Start() after Stop() error: %v", err)
	}
}

func TestRunnerStopWithoutStart(t *testing.T) {
	// Nothing to stop is not an error: shutdown runs the same path whether or
	// not the tunnel came up.
	NewRunner(testConfig(t.TempDir())).Stop()
}

func TestRunnerStartWithoutBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := NewRunner(testConfig(t.TempDir())).Start()
	if err == nil {
		t.Fatal("Start() with no cloudflared on PATH should fail")
	}
	if !strings.Contains(err.Error(), Binary) {
		t.Errorf("error %q should name %s", err, Binary)
	}
}
