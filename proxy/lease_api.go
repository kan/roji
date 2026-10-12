package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/kan/roji/config"
)

// LeasePath is the API endpoint `roji run` registers and gives back routes on.
const LeasePath = "/_api/leases"

// LeaseRequest asks for a hostname to be routed to a port on this machine.
type LeaseRequest struct {
	// Host is the name as the user gave it, resolved like a site in the config
	// file: "web" becomes "web.{domain}".
	Host string `json:"host"`

	// Port is where the dev server listens. A port and not a URL: the target is
	// always this machine, so the endpoint cannot be made to point a hostname
	// at anything else.
	Port int `json:"port"`

	// Owner identifies the `roji run` asking, so that only it renews or gives
	// back what it registered.
	Owner string `json:"owner"`
}

// LeaseResponse describes a route that was leased or renewed.
type LeaseResponse struct {
	Hostname string `json:"hostname"`
	URL      string `json:"url"`

	// CertCovered is false when roji's certificate is not valid for the
	// hostname, which a browser will refuse.
	CertCovered bool `json:"certCovered"`

	// Hides names the Docker service that was answering on the hostname and no
	// longer does, empty when there is none.
	Hides string `json:"hides,omitempty"`
}

// Lease error codes, for a client to choose its own wording by.
const (
	LeaseErrorInvalid  = "invalid"  // The request cannot be honored as written
	LeaseErrorDeclared = "declared" // The config file declares the hostname
	LeaseErrorHeld     = "held"     // Another `roji run` holds the hostname
)

// LeaseError is the body of a refused lease request.
type LeaseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// validHostname matches a DNS name of lowercase labels. Sites in the config
// file are taken as written, since whoever edits it owns the machine. A name
// arriving through the API gets checked.
var validHostname = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)

// serveLeases registers, renews and releases the routes `roji run` asks for.
//
//	POST   /_api/leases                      register or renew
//	DELETE /_api/leases?hostname=…&owner=…   release
func (h *Handler) serveLeases(w http.ResponseWriter, r *http.Request) {
	// This endpoint is for the CLI on this machine, and gives a hostname to any
	// port here, including ones whose only protection is listening on loopback:
	// a debugger, an admin UI. The rest of the API leaves "who can reach roji"
	// to `bind`. This checks for itself, since `bind` can be widened and what
	// this would then hand to the network is not roji's to hand over.
	if !fromThisMachine(r) {
		writeLeaseError(w, http.StatusForbidden, LeaseErrorInvalid, "this endpoint accepts requests from this machine only")
		return
	}

	// That does not keep out a web page open in a browser here, which can send
	// a POST wherever it likes. Browsers attach Origin to those and the CLI
	// does not, so its presence is refused.
	if r.Header.Get("Origin") != "" {
		writeLeaseError(w, http.StatusForbidden, LeaseErrorInvalid, "this endpoint does not accept requests from browsers")
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.serveLeaseAcquire(w, r)
	case http.MethodDelete:
		h.router.Release(r.URL.Query().Get("hostname"), r.URL.Query().Get("owner"))
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "POST, DELETE")
		writeLeaseError(w, http.StatusMethodNotAllowed, LeaseErrorInvalid, "method not allowed")
	}
}

func (h *Handler) serveLeaseAcquire(w http.ResponseWriter, r *http.Request) {
	var req LeaseRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeLeaseError(w, http.StatusBadRequest, LeaseErrorInvalid, "request body is not valid JSON")
		return
	}
	if req.Owner == "" {
		writeLeaseError(w, http.StatusBadRequest, LeaseErrorInvalid, "owner is required")
		return
	}

	// The same rules a site in the config file is held to, so that a name
	// refused there is refused here.
	site := config.ResolveSiteName(req.Host, false, h.baseDomain, h.dashboardHost)

	switch {
	case !validHostname.MatchString(site.Hostname):
		writeLeaseError(w, http.StatusBadRequest, LeaseErrorInvalid, fmt.Sprintf("%q is not a usable hostname", req.Host))
		return
	case site.Err != nil:
		writeLeaseError(w, http.StatusBadRequest, LeaseErrorInvalid, site.Err.Error())
		return
	case req.Port < 1 || req.Port > 65535:
		writeLeaseError(w, http.StatusBadRequest, LeaseErrorInvalid, fmt.Sprintf("port %d is not between 1 and 65535", req.Port))
		return
	}

	// "localhost" and not 127.0.0.1: which of the two loopback addresses a dev
	// server binds depends on the tool, and dialing the name tries both.
	target := &url.URL{Scheme: "http", Host: net.JoinHostPort("localhost", strconv.Itoa(req.Port))}

	hidden, err := h.router.Lease(site.Hostname, target, req.Owner)
	if err != nil {
		// Lease refuses for one of two reasons, both about who has the name.
		code := LeaseErrorHeld
		if errors.Is(err, ErrLeaseDeclared) {
			code = LeaseErrorDeclared
		}
		writeLeaseError(w, http.StatusConflict, code, err.Error())
		return
	}

	resp := LeaseResponse{
		Hostname:    site.Hostname,
		URL:         h.routeURL(site.Hostname),
		CertCovered: site.CertCovered,
	}
	if hidden != nil {
		resp.Hides = hidden.Backend.ServiceName
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode lease response", "error", err)
	}
}

// routeURL returns the address a browser on this machine reaches hostname at.
func (h *Handler) routeURL(hostname string) string {
	port := 443
	if h.statusConfig != nil && h.statusConfig.HTTPSPort != 0 {
		port = h.statusConfig.HTTPSPort
	}
	return httpsURL(hostname, port)
}

// fromThisMachine reports whether a request was sent by a process on the
// machine roji runs on.
//
// Loopback is the usual answer. The other is a connection whose two ends have
// the same address: with `bind` set to an address of this machine and no
// loopback, that address is where the CLI reaches roji, and the kernel hands
// it as the source too.
func fromThisMachine(r *http.Request) bool {
	remote := net.ParseIP(clientIP(r.RemoteAddr))
	if remote == nil {
		return false
	}
	if remote.IsLoopback() {
		return true
	}

	local, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok {
		return false
	}
	localIP := net.ParseIP(clientIP(local.String()))
	return localIP != nil && localIP.Equal(remote)
}

func writeLeaseError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(LeaseError{Code: code, Message: message})
}
