/*
Copyright 2019 The Cloud-Barista Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package managedset

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"syscall"
	"time"
)

// metadataAddrs are the cloud metadata addresses that sit outside the link-local range.
var metadataAddrs = []netip.Addr{
	netip.MustParseAddr("100.100.100.200"), // Alibaba Cloud
	netip.MustParseAddr("fd00:ec2::254"),   // AWS over IPv6
	netip.MustParseAddr("fd20:ce::254"),    // GCP over IPv6
}

// errBodyMax is how much of a server's error response is kept in an error message.
const errBodyMax = 512

// blockedAddr reports whether ip is a link-local, unspecified or cloud metadata address.
func blockedAddr(ip netip.Addr) bool {
	ip = ip.Unmap().WithZone("")
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	return slices.Contains(metadataAddrs, ip)
}

// checkHost rejects a literal blocked address and the metadata host names.
func checkHost(host string) error {
	if ip, err := netip.ParseAddr(host); err == nil && blockedAddr(ip) {
		return errors.New("link-local and metadata addresses are not allowed")
	}
	if strings.EqualFold(host, "metadata.google.internal") || strings.EqualFold(host, "metadata") {
		return errors.New("metadata hosts are not allowed")
	}
	return nil
}

// guardDial refuses a connection to a blocked address. It runs after name resolution, so a host name or a
// redirect that leads to such an address is refused as well.
func guardDial(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("unexpected dial address %q: %w", address, err)
	}
	if blockedAddr(ap.Addr()) {
		return fmt.Errorf("connections to link-local and metadata addresses are not allowed (%s)", ap.Addr())
	}
	return nil
}

// checkRedirect keeps the default hop limit and refuses a redirect to a blocked host. The dialer cannot see
// the final destination when a proxy is in use.
func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	return checkHost(req.URL.Hostname())
}

// ServerClient sends requests to a server URL registered through the API. Every such request goes through
// Do, so the destination check lives in one place.
type ServerClient struct {
	hc *http.Client
}

// NewServerClient returns a client for a registered server URL. tlsConfig may be nil.
func NewServerClient(tlsConfig *tls.Config, timeout time.Duration) *ServerClient {
	transport := &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		DialContext:     (&net.Dialer{Control: guardDial}).DialContext,
		TLSClientConfig: tlsConfig,
		// A custom dialer turns HTTP/2 off; keep it on where it was on before (no custom TLS config).
		ForceAttemptHTTP2: tlsConfig == nil,
	}
	return &ServerClient{hc: &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: checkRedirect}}
}

// Do sends one request. prepare, when set, adds headers or credentials before it is sent.
func (c *ServerClient) Do(ctx context.Context, method, rawUrl string, body io.Reader, prepare func(*http.Request)) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawUrl, body)
	if err != nil {
		return nil, err
	}
	if prepare != nil {
		prepare(req)
	}
	return c.hc.Do(req)
}

// ErrorBody returns the start of a server's error response for use in an error message.
func ErrorBody(raw []byte) string {
	if len(raw) > errBodyMax {
		raw = raw[:errBodyMax]
	}
	return strings.TrimSpace(string(raw))
}
