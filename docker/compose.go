package docker

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// buildComposeArgs builds the argument list for a docker compose command.
// If configFiles is non-empty, it splits on comma and prepends -f for each file.
func buildComposeArgs(configFiles string, args ...string) []string {
	var result []string
	result = append(result, "compose")

	if configFiles != "" {
		for f := range strings.SplitSeq(configFiles, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				result = append(result, "-f", f)
			}
		}
	}

	result = append(result, args...)
	return result
}

// composeLogReader wraps an io.ReadCloser and the underlying exec.Cmd,
// ensuring the subprocess is killed when Close is called.
type composeLogReader struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (r *composeLogReader) Close() error {
	err := r.ReadCloser.Close()
	if r.cmd.Process != nil {
		_ = r.cmd.Process.Kill()
	}
	_ = r.cmd.Wait()
	return err
}

// ComposeUp runs "docker compose up -d" for the given project.
func (c *Client) ComposeUp(ctx context.Context, workingDir, configFiles string) (string, error) {
	args := buildComposeArgs(configFiles, "up", "-d")
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workingDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("compose up failed: %w: %s", err, string(output))
	}
	return string(output), nil
}

// ComposeDown runs "docker compose down" for the given project.
func (c *Client) ComposeDown(ctx context.Context, workingDir, configFiles string) (string, error) {
	args := buildComposeArgs(configFiles, "down")
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workingDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("compose down failed: %w: %s", err, string(output))
	}
	return string(output), nil
}

// ComposeRestart runs "docker compose restart" for the given project.
func (c *Client) ComposeRestart(ctx context.Context, workingDir, configFiles string) (string, error) {
	args := buildComposeArgs(configFiles, "restart")
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workingDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("compose restart failed: %w: %s", err, string(output))
	}
	return string(output), nil
}

// ComposeLogs runs "docker compose logs -f" and returns a reader for the log stream.
func (c *Client) ComposeLogs(ctx context.Context, workingDir, configFiles string) (io.ReadCloser, error) {
	args := buildComposeArgs(configFiles, "logs", "-f", "--no-color")
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workingDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout // Merge stderr into stdout

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start compose logs: %w", err)
	}

	return &composeLogReader{
		ReadCloser: stdout,
		cmd:        cmd,
	}, nil
}
