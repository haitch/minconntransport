package minconntransport

import (
	"net/http"
)

type roundTripperWithConnectionManager struct {
	inner *http.Transport
	cm    *connectionManager
}

func New() http.RoundTripper {
	return NewFromHttpTransport(5, http.DefaultTransport.(*http.Transport))
}

func NewFromHttpTransport(minConnPerHost int32, inner *http.Transport) http.RoundTripper {
	innerTP := inner.Clone()

	// disable connection pooling from http package
	innerTP.MaxConnsPerHost = 0

	// use connectionManager to manage the connection
	// key different is:
	//   defaultTransport prefer [idle connection, new connection]
	//   minConnTransport prefer [new connection, pooled connection] til it reach to limit, then it would do round-robin on all pooled connection
	cm := newConnectionManager(minConnPerHost, innerTP.TLSClientConfig)
	innerTP.DialTLSContext = cm.DialTLSContext
	return &roundTripperWithConnectionManager{
		inner: innerTP,
		cm:    cm,
	}
}

func (rt *roundTripperWithConnectionManager) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := rt.inner.RoundTrip(req)
	if err != nil {
		// TODO: if error is connection closed/ server go away, we need to replace the connection.
		// how to find the connection is a issue, here in transport we don't know which connection is broken.
		// default library only give DialTLSContext for us to override dail, ideally there should be a callback on close.
		return nil, err
	}
	return resp, nil
}
