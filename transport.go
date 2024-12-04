package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	tpt "github.com/libp2p/go-libp2p/core/transport"

	ma "github.com/multiformats/go-multiaddr"

	mafmt "github.com/multiformats/go-multiaddr-fmt"

	"github.com/joomcode/errorx"
)

type CustomTransport struct {
	// Used to upgrade unsecure TCP connections to secure multiplexed and
	// authenticate connections.
	upgrader tpt.Upgrader

	myCustomNode *CustomProtocol
}

type Option func(*CustomTransport) error

func WithCustomNode(h *CustomProtocol) Option {
	return func(t *CustomTransport) error {
		t.myCustomNode = h
		return nil
	}
}

func NewCustomTransport(u tpt.Upgrader, rcmgr network.ResourceManager, opts ...Option) (tpt.Transport, error) {
	t := &CustomTransport{
		upgrader: u,
	}

	for _, opt := range opts {
		if err := opt(t); err != nil {
			return nil, err
		}
	}

	if t.myCustomNode == nil {
		return nil, errors.New("custom node required")
	}

	return t, nil
}

func (*CustomTransport) Proxy() bool {
	return false
}

var supportedProtos = []int{P_CUSTOM}

func (t *CustomTransport) Protocols() []int {
	return supportedProtos
}

var matcher = mafmt.And(
	mafmt.IP,
	mafmt.Base(ma.P_TCP),
	mafmt.Base(P_CUSTOM),
)

func (t *CustomTransport) CanDial(maddr ma.Multiaddr) bool {
	return matcher.Matches(maddr)
}

func (t *CustomTransport) Close() {
	// TODO
}

type transportListener struct {
	tpt.Listener
}

func (l *transportListener) Accept() (tpt.CapableConn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &capableConn{CapableConn: conn}, nil
}

func (t *CustomTransport) Listen(laddr ma.Multiaddr) (tpt.Listener, error) {
	malist := NewCustomListener(t.myCustomNode)
	return &transportListener{Listener: t.upgrader.UpgradeListener(t, malist)}, nil
}

func (t *CustomTransport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (tpt.CapableConn, error) {
	if !t.CanDial(raddr) {
		return nil, errorx.IllegalArgument.New(fmt.Sprintf("Can't dial \"%s\".", raddr))
	}

	conn, err := t.dialAndUpgrade(ctx, raddr, p)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (c *CustomTransport) dialAndUpgrade(ctx context.Context, a ma.Multiaddr, p peer.ID) (tpt.CapableConn, error) {
	ctx, cancel := context.WithCancel(ctx)

	conn := &CustomConn{
		myCustomNode: c.myCustomNode,
		remotePeer:   EncapsulatePeerID(p, a),
		ctx:          ctx,
		cancel:       cancel,
	}
	cc, err := c.upgrader.Upgrade(ctx, c, conn, network.DirOutbound, p, &network.NullScope{})
	if err != nil {
		return nil, err
	}
	fmt.Println("> UPGRADE DONE")
	return capableConn{cc}, nil
}
