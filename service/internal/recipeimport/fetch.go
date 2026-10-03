// Package recipeimport turns a recipe page or pasted recipe text into a
// draft recipe.Input. It fetches through a client that refuses the host's
// own network, so a member cannot use it to look around behind the service.
package recipeimport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"syscall"
	"time"
)

// ErrUnreachable is every fetch failure as the caller sees it: refused
// address, bad scheme, timeout, status, size. One error on purpose, so the
// answer never tells whether something lives at a private address.
var ErrUnreachable = errors.New("page unreachable")

var errRefusedAddr = errors.New("address not allowed")

// refused are ranges that pass IsGlobalUnicast but are not plain public
// hosts: CGNAT, IETF/benchmark/reserved IPv4, and the IPv6 forms that embed
// or tunnel to an IPv4 address (NAT64, 6to4, Teredo, IPv4-compatible) or
// are deprecated site-local.
var refused = func() []netip.Prefix {
	var ps []netip.Prefix
	for _, s := range []string{
		"100.64.0.0/10", "192.0.0.0/24", "198.18.0.0/15", "240.0.0.0/4",
		"64:ff9b::/96", "64:ff9b:1::/48", "2002::/16", "2001::/32", "::/96", "fec0::/10",
	} {
		ps = append(ps, netip.MustParsePrefix(s))
	}
	return ps
}()

// PublicOnly allows an address only if it is reachable on the public
// internet: no loopback, private, link-local (cloud metadata), CGNAT,
// multicast, unspecified or reserved address, also not as an IPv4-mapped,
// NAT64, 6to4, Teredo or IPv4-compatible IPv6.
func PublicOnly(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || (a.Is4() && a.As4()[0] == 0) {
		return false
	}
	for _, p := range refused {
		if p.Contains(a) {
			return false
		}
	}
	return true
}

// Fetched is a successful response.
type Fetched struct {
	Body        []byte
	ContentType string
	URL         *url.URL // after redirects
}

// Fetcher gets pages and photos for an import.
type Fetcher struct {
	client    *http.Client
	userAgent string
}

// NewFetcher returns a Fetcher whose every connection must satisfy allow.
// Production passes PublicOnly; tests pass a func that admits httptest.
func NewFetcher(version string, allow func(netip.Addr) bool) *Fetcher {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
		// Control runs for every connection, after DNS and per redirect, so
		// neither a redirect nor DNS rebinding gets past it.
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			a, err := netip.ParseAddr(host)
			if err != nil || !allow(a) {
				return errRefusedAddr
			}
			return nil
		},
	}
	transport := &http.Transport{
		Proxy:                 nil, // a proxy would make the dialer check the proxy, not the target
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
	}
	return &Fetcher{
		client: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) > 5 { // at most 5 redirects
					return errors.New("too many redirects")
				}
				if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
					return errors.New("redirect to another scheme")
				}
				return nil
			},
		},
		userAgent: "Rezepte/" + version + " (recipe import; +https://github.com/s-frei/rezepte)",
	}
}

// Get fetches rawURL, reading at most limit bytes of body. Any failure is
// ErrUnreachable, wrapping the cause for the log.
func (f *Fetcher) Get(ctx context.Context, rawURL, accept string, limit int64) (Fetched, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Fetched{}, fmt.Errorf("%w: bad url %q", ErrUnreachable, rawURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Fetched{}, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	req.Header.Set("User-Agent", f.userAgent)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return Fetched{}, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Fetched{}, fmt.Errorf("%w: status %d", ErrUnreachable, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return Fetched{}, fmt.Errorf("%w: read: %w", ErrUnreachable, err)
	}
	if int64(len(body)) > limit {
		return Fetched{}, fmt.Errorf("%w: larger than %d bytes", ErrUnreachable, limit)
	}
	return Fetched{Body: body, ContentType: resp.Header.Get("Content-Type"), URL: resp.Request.URL}, nil
}
