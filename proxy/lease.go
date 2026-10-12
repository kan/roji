package proxy

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

// LeaseTTL is how long a leased route lasts without being renewed.
//
// `roji run` renews well inside it. What it bounds is how long a route outlives
// a `roji run` that was killed before it could give the route back.
const LeaseTTL = 30 * time.Second

// Reasons a hostname cannot be leased.
var (
	// ErrLeaseDeclared: the config file declares the hostname. It was written
	// down deliberately, so a command started later does not take it over.
	ErrLeaseDeclared = errors.New("hostname is declared in the config file")
	// ErrLeaseHeld: another `roji run` holds the hostname and is still renewing it.
	ErrLeaseHeld = errors.New("hostname is held by another roji run")
)

// lease is a route registered through the API by `roji run`, on behalf of a
// dev server it started on a port of its choosing.
//
// It sits with the declared routes rather than in a tier of its own: Resolve
// treats it as one, so precedence stays the single rule "declared before
// discovered". It differs in where it comes from and how it goes away — it is
// not in the config file, so a reload leaves it alone, and it lapses unless its
// owner keeps renewing it.
type lease struct {
	route   *Route
	owner   string // Chosen by the `roji run` that registered it
	expires time.Time
}

// liveAt reports whether the lease is still in force at now.
func (l *lease) liveAt(now time.Time) bool {
	return now.Before(l.expires)
}

// liveLease returns the lease on hostname, or nil when there is none or it has
// lapsed. The caller holds r.mu.
//
// Expiry is decided here, at the moment of asking, so that a lapsed lease stops
// routing whether or not ExpireLeases has got round to removing it.
func (r *Router) liveLease(hostname string) *lease {
	if held, ok := r.leases[hostname]; ok && held.liveAt(r.now()) {
		return held
	}
	return nil
}

// declaredOn returns what the declared tier has on hostname: the site the
// config file declares or a live lease, else nil. The caller holds r.mu.
//
// At most one of the two exists. Lease refuses a hostname the config file
// declares, and ReplaceDeclared drops the lease on one it starts declaring.
func (r *Router) declaredOn(hostname string) *Route {
	if route, ok := r.declaredRoutes[hostname]; ok {
		return route
	}
	if held := r.liveLease(hostname); held != nil {
		return held.route
	}
	return nil
}

// Lease routes hostname to target on behalf of owner until LeaseTTL passes
// without another call. Calling again as the same owner renews the lease, and
// is how a lease held when roji restarted comes back.
//
// It returns the Docker route the lease hides, if any, for the caller to tell
// whoever asked.
func (r *Router) Lease(hostname string, target *url.URL, owner string) (hidden *Route, err error) {
	hostname = strings.ToLower(hostname)

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.declaredRoutes[hostname]; ok {
		return nil, ErrLeaseDeclared
	}

	held := r.liveLease(hostname)
	if held != nil && held.owner != owner {
		return nil, ErrLeaseHeld
	}

	expires := r.now().Add(LeaseTTL)
	hidden = r.dockerRouteOn(hostname)

	// A renewal changes nothing anyone is shown, so it is neither logged nor
	// announced to subscribers.
	if held != nil && held.route.LocalBackend.Target.String() == target.String() {
		held.expires = expires
		return hidden, nil
	}

	r.leases[hostname] = &lease{
		route: &Route{
			Hostname:     hostname,
			LocalBackend: &LocalBackend{Target: target, Temporary: true},
		},
		owner:   owner,
		expires: expires,
	}

	slog.Info("route leased", "hostname", hostname, "target", target.String())
	if hidden != nil {
		slog.Warn("leased route hides a Docker route on the same hostname",
			"hostname", hostname,
			"docker_service", hidden.Backend.ServiceName,
			"docker_path", hidden.PathPrefix)
	}

	go r.notifySubscribers()
	return hidden, nil
}

// Release gives up the lease owner holds on hostname. Anything else — no
// lease, or one that has since passed to another owner — is left alone, so a
// `roji run` exiting late cannot take down its successor's route.
func (r *Router) Release(hostname, owner string) {
	hostname = strings.ToLower(hostname)

	r.mu.Lock()
	defer r.mu.Unlock()

	held, ok := r.leases[hostname]
	if !ok || held.owner != owner {
		return
	}

	delete(r.leases, hostname)
	slog.Info("leased route released", "hostname", hostname)
	go r.notifySubscribers()
}

// ExpireLeases removes the leases that have lapsed and tells subscribers when
// it removed any. Routing does not depend on it — see liveLease — but the
// dashboard does, since nothing else announces that a route is gone.
func (r *Router) ExpireLeases() {
	r.mu.Lock()
	defer r.mu.Unlock()

	removed := false
	for hostname, held := range r.leases {
		if held.liveAt(r.now()) {
			continue
		}
		delete(r.leases, hostname)
		removed = true
		slog.Info("leased route expired", "hostname", hostname, "target", held.route.LocalBackend.Target.String())
	}

	if removed {
		go r.notifySubscribers()
	}
}

// RunLeaseExpiry calls ExpireLeases every interval until ctx is done.
func (r *Router) RunLeaseExpiry(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.ExpireLeases()
		}
	}
}
