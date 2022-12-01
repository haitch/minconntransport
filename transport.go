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

func NewFromHttpTransport(minConnPerHost int, inner *http.Transport) http.RoundTripper {
	innerTP := inner.Clone()

	// disable connection pooling from http package
	innerTP.MaxConnsPerHost = 0

	// use connectionManager
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
		return nil, err
	}
	return resp, nil
}
