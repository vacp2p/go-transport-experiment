package main

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/transport"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

type CustomListener struct {
	conns        chan net.Conn
	closed       bool
	myCustomNode *CustomProtocol
	mu           sync.Mutex
}

func NewCustomListener(myCustomNode *CustomProtocol) *CustomListener {

	l := &CustomListener{
		conns:        make(chan net.Conn),
		myCustomNode: myCustomNode,
	}

	go func() {
		l.conns <- myCustomNode.Conn()
	}()

	return l
}

// Implement manet.Listener interface

type capableConn struct {
	transport.CapableConn
}

func (c capableConn) ConnState() network.ConnectionState {
	cs := c.CapableConn.ConnState()
	cs.Transport = "custom"
	return cs
}

func (l *CustomListener) Accept() (manet.Conn, error) {
	conn, ok := <-l.conns
	if !ok {
		return nil, fmt.Errorf("listener closed")
	}
	return manet.WrapNetConn(conn)
}

func (l *CustomListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return fmt.Errorf("listener already closed")
	}
	l.closed = true
	close(l.conns)
	return nil
}

func (l *CustomListener) Addr() net.Addr {
	myAddress := l.myCustomNode.host.Addrs()[0]
	tcpPort, err := myAddress.ValueForProtocol(ma.P_TCP)
	if err != nil {
		panic("invalid tcp component in multiaddress")
	}

	// TODO: support all ips

	ip4, err := myAddress.ValueForProtocol(ma.P_IP4)
	if err != nil {
		panic("invalid ip4 component in multiaddress")
	}

	portInt, err := strconv.Atoi(tcpPort)
	if err != nil {
		panic("could not convert tcp port in int")
	}

	return &net.TCPAddr{IP: net.ParseIP(ip4), Port: portInt}
}

func (l *CustomListener) Multiaddr() ma.Multiaddr {
	addr := l.Addr().(*net.TCPAddr)
	ma, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d/custom/%s", "0.0.0.0", addr.Port, l.myCustomNode.host.ID()))
	if err != nil {
		panic("could not obtain multiaddr" + err.Error())
	}
	return ma
}
