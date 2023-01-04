package minconntransport

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync/atomic"
)

type connMgrErrCode string

func (ce connMgrErrCode) Error() string { return string(ce) }

const (
	connNotAvailable connMgrErrCode = "connNotAvailable"
)

type perRemoteAddrConnMgr struct {
	connections []*tls.Conn
	corsor      int32
	count       int32
}

func newRemoteAddrConnMgr(count int32) *perRemoteAddrConnMgr {
	return &perRemoteAddrConnMgr{
		connections: make([]*tls.Conn, count),
		count:       count,
		corsor:      0,
	}
}

func (hm *perRemoteAddrConnMgr) Get() (*tls.Conn, error) {
	var index int32
	if index = atomic.LoadInt32(&hm.corsor); index >= hm.count {
		atomic.SwapInt32(&hm.corsor, 0)
		index = 0
	}

	conn := hm.connections[index]
	if conn != nil {
		// TODO: check conn.ConnectionState()
		// move the corsor to next slot
		atomic.AddInt32(&hm.corsor, 1)
		return conn, nil
	} else {
		// not moving the corsor, so later we can set the connection to the same slot
		return nil, connNotAvailable
	}
}

func (hm *perRemoteAddrConnMgr) Set(conn *tls.Conn) {
	var index int32
	if index = atomic.LoadInt32(&hm.corsor); index >= hm.count {
		atomic.SwapInt32(&hm.corsor, 0)
		index = 0
	}

	fmt.Println(conn.LocalAddr(), conn.RemoteAddr())

	hm.connections[index] = conn
	// move the corsor to next slot
	atomic.AddInt32(&hm.corsor, 1)
}

type connectionManager struct {
	connections map[string]*perRemoteAddrConnMgr // map of host to connections
	connPerHost int32
	tlsConfig   *tls.Config
}

func newConnectionManager(connPerHost int32, tlsCfg *tls.Config) *connectionManager {
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
