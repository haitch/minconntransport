package minconntransport

import (
	"errors"
	"net"
	"net/http"
	"strings"
)

type roundTripperWithConnectionManager struct {
	managed      *http.Transport
	original     *http.Transport
	cm           *connectionManager
	perHostLimit map[string]int32
}

func NewFromHttpTransport(inner *http.Transport, hostLimit map[string]int32) http.RoundTripper {
	managedTP := inner.Clone()

	// disable connection pooling from http package
	managedTP.MaxConnsPerHost = 0

	managedTP.ForceAttemptHTTP2 = false
	managedTP.TLSClientConfig.NextProtos = []string{"http/1.1"}

	// use connectionManager to manage the connection
	// key different is:
	//   defaultTransport prefer [idle connection, new connection]
	//   minConnTransport prefer [new connection, pooled connection] til it reach to limit, then it would do round-robin on all pooled connection
	cm := newConnectionManager(managedTP.TLSClientConfig, sanitizeHostLimit(hostLimit))
	managedTP.DialTLSContext = cm.DialTLSContext
	return &roundTripperWithConnectionManager{
		managed:  managedTP,
		original: inner,
		cm:       cm,
	}
}

func (rt *roundTripperWithConnectionManager) RoundTrip(req *http.Request) (*http.Response, error) {
	// if host is not in the hostLimit, use default transport
	if _, ok := rt.cm.hostLimit[sanitizeHostName(req.Host)]; !ok {
		return rt.original.RoundTrip(req)
	}

	resp, err := rt.managed.RoundTrip(req)
	if err != nil {
		// if it's a network error, mark the connection as broken
		// not handling retry here, this pkg is only for connection management, at least for now.
		netOpErr := &net.OpError{}
		if errors.As(err, &netOpErr) {
			rt.cm.markBrokenConnection(netOpErr)
		}
		return nil, err
	}
	return resp, nil
}

func sanitizeHostLimit(hostLimit map[string]int32) map[string]int32 {
	hostLimitCopy := make(map[string]int32)
	for k, v := range hostLimit {
		if v >= 0 {
			hostLimitCopy[sanitizeHostName(k)] = v
		}
	}
	return hostLimitCopy
}

func sanitizeHostName(host string) string {
	if strings.Contains(host, ":") {
		return host
	}
	return strings.ToLower(host) + ":443"
}
