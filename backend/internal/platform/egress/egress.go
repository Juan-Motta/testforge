// Package egress guards outbound HTTP calls to user-configured URLs (provider
// endpoints, Langfuse hosts) against server-side request forgery.
//
// URLs must be https. Unless private networking is allowed (local
// development), connections to loopback, private, link-local, CGNAT or
// unspecified addresses are refused at dial time, after DNS resolution, so a
// public hostname that resolves to an internal address is blocked too.
package egress

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// ErrBlocked is returned (wrapped) when a destination is not allowed.
var ErrBlocked = errors.New("destination is a private network address")

var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// CheckURL validates and normalizes a base URL: absolute, https (http only
// when allowPrivate), a host, no credentials, query or fragment, no trailing slash.
func CheckURL(raw string, allowPrivate bool) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Hostname() == "" {
		return "", errors.New("Enter an absolute URL, like https://host/path.")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && allowPrivate) {
		return "", errors.New("Use an https URL.")
	}
	if u.User != nil {
		return "", errors.New("Do not put credentials in the URL.")
	}
	if u.RawQuery != "" || u.Fragment != "" || strings.Contains(raw, "#") {
		return "", errors.New("Remove the query string or fragment.")
	}
	return strings.TrimRight(u.String(), "/"), nil
}

// NewClient returns an HTTP client that enforces the private-address policy
// on every connection, including redirects.
func NewClient(allowPrivate bool, timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout}
	if !allowPrivate {
		dialer.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil || !isPublic(host) {
				return fmt.Errorf("%w: %s", ErrBlocked, host)
			}
			return nil
		}
	}
	transport := &http.Transport{
		Proxy:                 nil, // never route provider calls through an ambient proxy
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		MaxIdleConns:          20,
		IdleConnTimeout:       60 * time.Second,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "https" && !allowPrivate {
				return fmt.Errorf("%w: redirect to %s", ErrBlocked, req.URL.Scheme)
			}
			return nil
		},
	}
}

func isPublic(host string) bool {
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	return !(addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() || addr.IsUnspecified() || addr.IsInterfaceLocalMulticast() || cgnat.Contains(addr))
}
