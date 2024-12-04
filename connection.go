package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

// CustomConn is a simple implementation of net.Conn
type CustomConn struct {
	ctx    context.Context
	cancel context.CancelFunc

	myCustomNode *CustomProtocol
	remotePeer   multiaddr.Multiaddr

	closed bool
	mu     sync.Mutex
}

// Implement the net.Conn interface for CustomConn

func (c *CustomConn) Read(b []byte) (n int, err error) {
	fmt.Println("Calling Read in", c.myCustomNode.host.ID())
	data, ok := <-c.myCustomNode.C
	if !ok {
		return 0, fmt.Errorf("connection closed")
	}
	fmt.Println("Done Read in", c.myCustomNode.host.ID(), string(data.Payload))
	n = copy(b, data.Payload)
	return n, nil
}

func (c *CustomConn) Write(b []byte) (n int, err error) {
	fmt.Println("Calling Write in", c.myCustomNode.host.ID(), "with", string(b))
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, fmt.Errorf("connection closed")
	}

	path := c.myCustomNode.GetRandomPath(c.remotePeer)
	originAddr, _ := multiaddr.NewMultiaddr(c.myCustomNode.host.Addrs()[0].String() + "/custom/" + c.myCustomNode.host.ID().String() + "/p2p/" + c.myCustomNode.parentID.String())
	message := &Message{
		Payload: &Payload{
			Payload: b,
			Origin:  originAddr.String(),
		},
		Path: path[1:],
	}

	err = c.myCustomNode.Send(c.ctx, path[0], message)
	if err != nil {
		return 0, err
	}

	return len(b), nil
}

func (c *CustomConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cancel()

	if c.closed {
		return fmt.Errorf("connection already closed")
	}
	c.closed = true
	return nil
}

func (c *CustomConn) LocalAddr() net.Addr {
	addr, err := manet.ToNetAddr(c.myCustomNode.host.Addrs()[0])
	if err != nil {
		panic("cannot obtain local addr")
	}
	return addr
}

func (c *CustomConn) LocalMultiaddr() multiaddr.Multiaddr {
	return c.myCustomNode.host.Addrs()[0]
}

func (c *CustomConn) RemoteMultiaddr() multiaddr.Multiaddr {
	return c.remotePeer
}

func (c *CustomConn) RemoteAddr() net.Addr {
	remoteAddrParts := strings.Split(c.remotePeer.String(), "/custom")
	remoteAddr, _ := multiaddr.NewMultiaddr(remoteAddrParts[0])

	addr, err := manet.ToNetAddr(remoteAddr)
	if err != nil {
		panic("cannot obtain local addr" + err.Error())
	}
	return addr
}

func (c *CustomConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *CustomConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *CustomConn) SetWriteDeadline(t time.Time) error {
	return nil
}
