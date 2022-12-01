package minconntransport

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
)

type connMgrErrCode string

func (ce connMgrErrCode) Error() string { return string(ce) }

const (
	connNotAvailable connMgrErrCode = "connNotAvailable"
)

type perRemoteAddrConnMgr struct {
	connections []*tls.Conn
	corsor      int
	count       int
}

func newRemoteAddrConnMgr(count int) *perRemoteAddrConnMgr {
	return &perRemoteAddrConnMgr{
		connections: make([]*tls.Conn, count),
		count:       count,
	}
}

func (hm *perRemoteAddrConnMgr) Get() (*tls.Conn, error) {
	if hm.corsor >= hm.count {
		hm.corsor = 0
	}

	conn := hm.connections[hm.corsor]
	if conn != nil {
		cs := conn.ConnectionState()
		fmt.Println(cs)

		hm.corsor++
		return conn, nil
	} else {
		// not moving the corsor, so later we can set the connection to the same slot
		return nil, connNotAvailable
	}
}

func (hm *perRemoteAddrConnMgr) Set(conn *tls.Conn) {
	if hm.corsor >= hm.count {
		hm.corsor = 0
	}

	fmt.Println(conn.LocalAddr(), conn.RemoteAddr())

	hm.connections[hm.corsor] = conn
	hm.corsor++
}

type connectionManager struct {
	connections map[string]*perRemoteAddrConnMgr // map of host to connections
	connPerHost int
	tlsConfig   *tls.Config
}

func newConnectionManager(connPerHost int, tlsCfg *tls.Config) *connectionManager {
	if tlsCfg == nil {
		tlsCfg = &tls.Config{}
	}

	return &connectionManager{
		connections: make(map[string]*perRemoteAddrConnMgr),
		connPerHost: connPerHost,
		tlsConfig:   tlsCfg,
	}
}

func (cm *connectionManager) DialTLSContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var hm *perRemoteAddrConnMgr
	var ok bool
	if hm, ok = cm.connections[addr]; !ok {
		hm = newRemoteAddrConnMgr(cm.connPerHost)
		cm.connections[addr] = hm
	}

	tlsConn, err := hm.Get()
	if err == nil {
		return tlsConn, nil
	}

	tlsCfgCloned := cm.tlsConfig.Clone()
	dialer := &tls.Dialer{Config: tlsCfgCloned}
	tlsConnInterface, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	tlsConn = tlsConnInterface.(*tls.Conn)
	hm.Set(tlsConn)
	return tlsConn, nil
}
