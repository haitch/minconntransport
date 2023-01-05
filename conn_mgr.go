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

type connectionMeta struct {
	conn   *tls.Conn
	broken bool
}

type perRemoteAddrConnMgr struct {
	connections []*connectionMeta
	corsor      int32
	count       int32
}

func newRemoteAddrConnMgr(count int32) *perRemoteAddrConnMgr {
	return &perRemoteAddrConnMgr{
		connections: make([]*connectionMeta, count),
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
	if conn != nil && !conn.broken {
		// TODO: check conn.ConnectionState()
		// move the corsor to next slot
		atomic.AddInt32(&hm.corsor, 1)
		return conn.conn, nil
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

	fmt.Printf("store new connection: %s --> %s \n", conn.LocalAddr(), conn.RemoteAddr())

	hm.connections[index] = &connectionMeta{conn: conn, broken: false}
	// move the corsor to next slot
	atomic.AddInt32(&hm.corsor, 1)
}

func (hm *perRemoteAddrConnMgr) markBrokenConnection(err *net.OpError) {
	// TODO: direct locate with assist of a map?
	for _, conn := range hm.connections {
		if conn.conn.RemoteAddr() == err.Addr && conn.conn.LocalAddr() == err.Source {
			fmt.Printf("found broken connection: %s  --> %s \n", conn.conn.LocalAddr(), conn.conn.RemoteAddr())
			conn.broken = true
		}
	}
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

func (cm *connectionManager) markBrokenConnection(err *net.OpError) {
	// TODO: direct locate instead of loop through all
	for _, hm := range cm.connections {
		hm.markBrokenConnection(err)
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
