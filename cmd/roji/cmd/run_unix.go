//go:build !windows

package cmd

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// relayedSignals are the signals roji run passes on to the command instead of
// dying of them.
var relayedSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}

// signalRelay passes signals roji run receives on to the command it started,
// and to whatever the command started in turn.
//
// The second part is the point. A dev server is rarely the direct child:
// `npx vite` is npm running a shell running node, and a signal sent to npm
// alone leaves node holding the port. How the rest are reached depends on
// whether roji run has the terminal.
type signalRelay struct {
	child *exec.Cmd

	// foreground is true when roji run is in the foreground process group of a
	// terminal. The command then stays in that group: moved out of it, it
	// could no longer read the terminal, and dev servers do (wrangler's
	// hotkeys, for one). Otherwise it gets a process group of its own.
	foreground bool
}

func newSignalRelay(child *exec.Cmd) *signalRelay {
	r := &signalRelay{child: child, foreground: inForegroundGroup()}
	if !r.foreground {
		child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	return r
}

// inForegroundGroup reports whether stdin is a terminal whose foreground
// process group is this process's.
//
// Asking the terminal, rather than asking whether stdin is a character device:
// /dev/null is one too, and a roji run started under a supervisor with its
// input there would be treated as if a terminal were delivering its signals.
func inForegroundGroup() bool {
	var pgrp int32
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, os.Stdin.Fd(), uintptr(syscall.TIOCGPGRP), uintptr(unsafe.Pointer(&pgrp)))
	return errno == 0 && int(pgrp) == syscall.Getpgrp()
}

// forward passes sig on. It is called for every signal roji run receives.
func (r *signalRelay) forward(sig os.Signal) {
	s, ok := sig.(syscall.Signal)
	if !ok {
		return
	}

	// The command has a process group to itself, so the group is exactly the
	// command and what it started.
	if !r.foreground {
		_ = syscall.Kill(-r.child.Process.Pid, s)
		return
	}

	// In the foreground the command shares roji run's process group.
	//
	// What the terminal sends, it sends to the whole group: Ctrl-C, and the
	// hangup when the terminal goes away. The command has those already, and
	// delivering an interrupt twice is how many dev servers are told to skip
	// their cleanup. The cost is that an interrupt sent to roji run alone,
	// with `kill -INT`, is not passed on; nothing tells the two apart. SIGTERM
	// is the signal for that.
	if sig == os.Interrupt || sig == syscall.SIGHUP {
		return
	}

	// The group is not the command's alone here. Started from make, an npm
	// script or a shell script, roji run shares it with whatever started it
	// and whatever that is running beside it, and a signal to the group would
	// take those down too. So the command's processes are found by descent
	// and signaled one by one.
	for _, pid := range descendants(r.child.Process.Pid) {
		_ = syscall.Kill(pid, s)
	}
}

// descendants returns root and every process descended from it.
//
// It reads the process table through ps, which Linux and macOS both have and
// agree on the flags of. When that fails, root alone is returned: signaling
// the direct child is what there is left to do.
func descendants(root int) []int {
	out, err := exec.Command("ps", "-A", "-o", "pid=,ppid=").Output()
	if err != nil {
		return []int{root}
	}

	children := make(map[int][]int)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		pid, errPid := strconv.Atoi(fields[0])
		ppid, errPpid := strconv.Atoi(fields[1])
		if errPid != nil || errPpid != nil {
			continue
		}
		children[ppid] = append(children[ppid], pid)
	}

	pids := []int{root}
	for i := 0; i < len(pids); i++ {
		pids = append(pids, children[pids[i]]...)
	}
	return pids
}

// exitCode returns what roji run exits with for a command that ended with err,
// which is nil for one that succeeded.
//
// A command killed by a signal is reported the way a shell does, as 128 plus
// the signal number, so that whatever started roji run can tell Ctrl-C from a
// crash.
func exitCode(err error) int {
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return 0
	case !errors.As(err, &exitErr):
		return 1
	}

	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal())
	}
	return exitErr.ExitCode()
}
