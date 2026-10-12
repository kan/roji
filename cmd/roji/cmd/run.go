package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kan/roji/apiclient"
	"github.com/kan/roji/config"
	"github.com/kan/roji/i18n"
	"github.com/kan/roji/proxy"
	"github.com/spf13/cobra"
)

const (
	// portPlaceholder is replaced in the command's arguments by the port
	// roji run chose.
	portPlaceholder = "{port}"

	// leaseRequestTimeout bounds one call to the lease endpoint, which is on
	// this machine.
	leaseRequestTimeout = 3 * time.Second

	// leaseRenewInterval leaves room for two renewals to fail before the lease
	// lapses.
	leaseRenewInterval = proxy.LeaseTTL / 3

	// listenGracePeriod is how long the command gets to open the port before
	// roji run suggests it is listening somewhere else.
	listenGracePeriod = 20 * time.Second
)

var runCmd = &cobra.Command{
	Use:   "run <name> -- <command> [args...]",
	Short: "Start a dev server on a free port and give it a hostname",
	Long: `Pick a free port, route https://<name>.<domain> to it, and run the command.

The port is passed to the command as the PORT environment variable and replaces
every {port} in its arguments. The route is removed when the command exits.`,
	Example: `  roji run app -- npx vite --port {port}
  roji run api -- npx wrangler dev --port {port}
  roji run web -- bin/rails server`,
	// Everything after "--" belongs to the command being run. Requiring the
	// separator is what keeps its flags from being read as roji's.
	Args: func(cmd *cobra.Command, args []string) error {
		if cmd.ArgsLenAtDash() != 1 || len(args) < 2 {
			return errors.New(i18n.T("cmd.run.usage"))
		}
		return nil
	},
	RunE: runRun,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
	// The arguments were fine, so a failure from here on is not about how the
	// command was typed and the usage text would only bury the reason. main
	// prints the error.
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	settings, err := config.Load(configFile, collectCLIOverrides(cmd))
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	port, err := pickFreePort()
	if err != nil {
		return fmt.Errorf("failed to find a free port: %w", err)
	}

	l := newLeaseHolder(settings, args[0], port)

	// The route comes first. Without it the command would start and its URL
	// would not open, which looks like the command's fault.
	lease, err := l.acquire(cmd.Context())
	if err != nil {
		return err
	}
	l.hostname = lease.Hostname

	fmt.Fprintln(os.Stderr, i18n.Tf("cmd.run.started", lease.URL, port))
	if !lease.CertCovered {
		fmt.Fprintln(os.Stderr, i18n.Tf("cmd.run.cert_warning", lease.Hostname, settings.Domain))
	}
	if lease.Hides != "" {
		fmt.Fprintln(os.Stderr, i18n.Tf("cmd.run.hides_warning", lease.Hostname, lease.Hides))
	}

	code := runWithLease(l, args[1:])
	l.release()

	// The command's exit code is roji run's. Exiting here skips deferred
	// calls, which is why the release above is not one.
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

// runWithLease runs the command, keeping the lease alive for as long as it
// runs, and returns the exit code roji run should finish with.
func runWithLease(l *leaseHolder, command []string) int {
	port := l.request.Port
	portValue := strconv.Itoa(port)
	for i, arg := range command {
		command[i] = strings.ReplaceAll(arg, portPlaceholder, portValue)
	}

	child := exec.Command(command[0], command[1:]...)
	child.Stdin, child.Stdout, child.Stderr = os.Stdin, os.Stdout, os.Stderr
	child.Env = append(os.Environ(), "PORT="+portValue)

	relay := newSignalRelay(child)

	// Before Start, so that a signal arriving while the command starts does
	// not kill roji run and leave the command behind without its route.
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, relayedSignals...)
	defer signal.Stop(signals)

	if err := child.Start(); err != nil {
		fmt.Fprintln(os.Stderr, i18n.Tf("cmd.run.start_failed", command[0], err))
		return 127
	}

	ctx, stop := context.WithCancel(context.Background())
	renewing := make(chan struct{})
	go func() {
		defer close(renewing)
		l.keepAlive(ctx)
	}()
	// Renewing has to be over before this returns and the caller gives the
	// route back. A renewal still on its way would register the route again
	// after the release, and it would then sit there until it lapsed, refusing
	// the same name to the next roji run.
	defer func() {
		stop()
		<-renewing
	}()

	go warnIfNotListening(ctx, port)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case sig := <-signals:
				relay.forward(sig)
			}
		}
	}()

	return exitCode(child.Wait())
}

// warnIfNotListening says so once when the command has not opened the port
// after listenGracePeriod.
//
// The usual cause is a tool that ignores PORT and was not given {port}: it
// starts on its own default port, everything looks fine in its output, and the
// URL roji run printed shows an error page.
func warnIfNotListening(ctx context.Context, port int) {
	addr := net.JoinHostPort("localhost", strconv.Itoa(port))
	deadline := time.NewTimer(listenGracePeriod)
	defer deadline.Stop()
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline.C:
			fmt.Fprintln(os.Stderr, i18n.Tf("cmd.run.not_listening", port, portPlaceholder))
			return
		case <-tick.C:
			conn, err := (&net.Dialer{Timeout: 400 * time.Millisecond}).DialContext(ctx, "tcp", addr)
			if err == nil {
				conn.Close()
				return
			}
		}
	}
}

// pickFreePort returns a port nothing is listening on, on either loopback
// address. Something else could take it before the command binds it, but there
// is no way to reserve a port for another process.
//
// Both addresses, because roji reaches the command through "localhost", which
// is both. A port free on 127.0.0.1 and taken on ::1 would send requests to
// whatever holds it there.
func pickFreePort() (int, error) {
	var lastErr error
	for range 20 {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return 0, err
		}
		port := ln.Addr().(*net.TCPAddr).Port

		ln6, err := net.Listen("tcp", net.JoinHostPort("::1", strconv.Itoa(port)))
		ln.Close()
		switch {
		case err == nil:
			ln6.Close()
			return port, nil
		case !errors.Is(err, syscall.EADDRINUSE):
			// No ::1 to listen on, so nothing else is listening there either.
			return port, nil
		}
		lastErr = err
	}
	return 0, lastErr
}

// newLeaseOwner returns a value no other roji run will pick.
func newLeaseOwner() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // Does not fail: see crypto/rand
	return hex.EncodeToString(b)
}

// leaseHolder registers a route with the running roji and keeps it registered.
type leaseHolder struct {
	settings *config.Settings
	client   *http.Client // One for every request, so renewals share a connection
	request  proxy.LeaseRequest

	// hostname is the name as roji resolved it, known once the route is
	// registered.
	hostname string
}

func newLeaseHolder(settings *config.Settings, host string, port int) *leaseHolder {
	return &leaseHolder{
		settings: settings,
		client:   apiclient.NewClient(leaseRequestTimeout),
		request:  proxy.LeaseRequest{Host: host, Port: port, Owner: newLeaseOwner()},
	}
}

// leaseFailure is why a route could not be registered or renewed.
//
// It carries the reason and leaves the advice to whoever reports it. The same
// refusal means "pick another name" before the command has started and
// something else once it is running.
type leaseFailure struct {
	// reason says what went wrong, in the user's language and without advice.
	reason string
	// advice is what to do about it when it stops roji run from starting.
	advice string
}

func (f *leaseFailure) Error() string {
	if f.advice == "" {
		return f.reason
	}
	return f.reason + "\n" + f.advice
}

// acquire registers the route, or renews it when it is already held. A failure
// is returned as a *leaseFailure.
func (l *leaseHolder) acquire(ctx context.Context) (*proxy.LeaseResponse, error) {
	resp, err := apiclient.SendJSON(ctx, l.client, l.settings, http.MethodPost, proxy.LeasePath, l.request)
	if err != nil {
		return nil, &leaseFailure{
			reason: i18n.Tf("cmd.run.unreachable", err),
			advice: i18n.T("cmd.run.unreachable_advice"),
		}
	}
	defer resp.Body.Close()

	// A roji from before roji run existed serves the dashboard page for a path
	// it does not know, so anything that is not this endpoint's JSON means the
	// running roji is too old.
	unsupported := &leaseFailure{reason: i18n.Tf("cmd.run.unsupported", resp.StatusCode)}

	if resp.StatusCode != http.StatusOK {
		var refused proxy.LeaseError
		if err := json.NewDecoder(resp.Body).Decode(&refused); err != nil {
			return nil, unsupported
		}
		switch refused.Code {
		case proxy.LeaseErrorDeclared:
			return nil, &leaseFailure{
				reason: i18n.Tf("cmd.run.declared", l.request.Host),
				advice: i18n.T("cmd.run.declared_advice"),
			}
		case proxy.LeaseErrorHeld:
			return nil, &leaseFailure{
				reason: i18n.Tf("cmd.run.held", l.request.Host),
				advice: i18n.T("cmd.run.held_advice"),
			}
		default:
			return nil, &leaseFailure{reason: i18n.Tf("cmd.run.refused", refused.Message)}
		}
	}

	var lease proxy.LeaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&lease); err != nil {
		return nil, unsupported
	}
	return &lease, nil
}

// keepAlive renews the lease until ctx is done.
//
// A renewal is the same request as the registration, so a roji that restarted
// and forgot the route gets it back on the next one. A failure is reported
// when it starts and when it ends, not on every attempt.
func (l *leaseHolder) keepAlive(ctx context.Context) {
	ticker := time.NewTicker(leaseRenewInterval)
	defer ticker.Stop()

	failing := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, err := l.acquire(ctx)
			switch {
			case ctx.Err() != nil:
				return
			case err != nil && !failing:
				// The reason alone. The advice that goes with it is about
				// starting the command, and the command is running.
				reason := err.Error()
				var failure *leaseFailure
				if errors.As(err, &failure) {
					reason = failure.reason
				}
				fmt.Fprintln(os.Stderr, i18n.Tf("cmd.run.renew_failed", l.hostname, reason))
			case err == nil && failing:
				fmt.Fprintln(os.Stderr, i18n.Tf("cmd.run.renew_recovered", l.hostname))
			}
			failing = err != nil
		}
	}
}

// release gives the route back. Failing to is not reported: the command has
// already exited, and a lease that is not given back lapses on its own.
func (l *leaseHolder) release() {
	query := url.Values{"hostname": {l.hostname}, "owner": {l.request.Owner}}
	resp, err := apiclient.SendJSON(context.Background(), l.client, l.settings, http.MethodDelete,
		proxy.LeasePath+"?"+query.Encode(), nil)
	if err == nil {
		resp.Body.Close()
	}
}
