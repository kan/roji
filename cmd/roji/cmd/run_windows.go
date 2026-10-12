//go:build windows

package cmd

import (
	"errors"
	"os"
	"os/exec"
)

// relayedSignals are the signals roji run passes on to the command instead of
// dying of them.
var relayedSignals = []os.Signal{os.Interrupt}

// signalRelay passes signals roji run receives on to the command it started.
type signalRelay struct {
	child *exec.Cmd
}

func newSignalRelay(child *exec.Cmd) *signalRelay {
	return &signalRelay{child: child}
}

// forward passes sig on. Ctrl-C reaches every process attached to the console,
// the command included, so with a console there is nothing to do. Without
// one, Windows has no signal to send another process, and ending the command
// is what was asked for.
func (r *signalRelay) forward(os.Signal) {
	if info, err := os.Stdin.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
		return
	}
	_ = r.child.Process.Kill()
}

// exitCode returns what roji run exits with for a command that ended with err,
// which is nil for one that succeeded.
func exitCode(err error) int {
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exitErr) && exitErr.ExitCode() >= 0:
		return exitErr.ExitCode()
	default:
		return 1
	}
}
