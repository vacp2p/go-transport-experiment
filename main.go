package main

import (
	"context"
	"fmt"
	"net"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/muxer/yamux"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	golog "github.com/ipfs/go-log/v2"
)

func assert(err error) {
	if err != nil {
		panic("ERROR!!!: " + err.Error())
	}
}

const P_CUSTOM = 887

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	multiaddr.AddProtocol(multiaddr.Protocol{
		Name:       "custom",
		Code:       P_CUSTOM,
		VCode:      multiaddr.CodeToVarint(P_CUSTOM),
		Size:       multiaddr.LengthPrefixedVarSize,
		Transcoder: multiaddr.TranscoderP2P,
	})

	golog.SetAllLoggers(golog.LevelInfo)

	var customHosts []host.Host
	var customProtocols []*CustomProtocol
	for i := 0; i < 10; i++ {
		h, err := makeCustomHost()
		assert(err)
		customHosts = append(customHosts, h)
	}
	for i := 0; i < 10; i++ {
		p, err := makeCustomProto(ctx, customHosts[i], customHosts)
		assert(err)
		customProtocols = append(customProtocols, p)
	}

	// nodeA and B are not connected directly. Message will go thru via custom protocol
	nodeA, err := makeBasicHost(customProtocols[0])
	assert(err)

	nodeB, err := makeBasicHost(customProtocols[9])
	assert(err)

	fmt.Println("NODEA", nodeA.Addrs(), nodeA.ID())
	fmt.Println("Custom0", customHosts[0].Addrs(), customHosts[0].ID())

	fmt.Println("NODEB", nodeB.Addrs(), nodeB.ID())
	fmt.Println("Custom9", customHosts[9].Addrs(), customHosts[9].ID())

	nodeA.Peerstore().AddAddrs(nodeB.ID(), nodeB.Addrs(), peerstore.PermanentAddrTTL)

	pingService := ping.NewPingService(nodeA)

	fmt.Println("==================================================================")
	result := pingService.Ping(ctx, nodeB.ID())

	fmt.Println("Waiting for ping result...")
	fmt.Println(<-result)

	<-ctx.Done()
}

func makeCustomHost() (host.Host, error) {
	hostAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", "0.0.0.0", 0))
	if err != nil {
		return nil, err
	}

	tcpAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/tcp/%d", hostAddr.Port))
	if err != nil {
		return nil, err
	}

	hostAddrMA, err := manet.FromNetAddr(hostAddr)
	if err != nil {
		return nil, err
	}

	tcpAddrMa := hostAddrMA.Encapsulate(tcpAddr)

	h, err := libp2p.New(
		libp2p.ListenAddrs(tcpAddrMa),
		libp2p.Transport(tcp.NewTCPTransport),
	)
	if err != nil {
		return nil, err
	}

	return h, nil
}

func makeCustomProto(ctx context.Context, h host.Host, listOfNodes []host.Host) (*CustomProtocol, error) {
	var maddrs []multiaddr.Multiaddr
	for _, m := range listOfNodes {
		maddrs = append(maddrs, EncapsulatePeerID(m.ID(), m.Addrs()[0]))
	}

	customProto, err := NewCustomProtocol(h, maddrs)
	if err != nil {
		return nil, err
	}

	customProto.Start(ctx)

	return customProto, nil
}

func makeBasicHost(myCustomProtocol *CustomProtocol) (host.Host, error) {
	// custom multiaddress
	hostAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", "0.0.0.0", 0))
	if err != nil {
		return nil, err
	}

	hostAddrMA, err := manet.FromNetAddr(hostAddr)
	if err != nil {
		return nil, err
	}

	customAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/custom/%s", myCustomProtocol.host.ID()))
	if err != nil {
		return nil, err
	}

	customMA := hostAddrMA.Encapsulate(customAddr)

	// tcp multiaddress
	// hostAddr2, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", "0.0.0.0", 0))
	// if err != nil {
	// 	return nil, err
	// }

	// hostAddrMA2, err := manet.FromNetAddr(hostAddr2)
	// if err != nil {
	// 	return nil, err
	// }

	h, err := libp2p.New(
		libp2p.ListenAddrs(customMA), //, hostAddrMA2),
		libp2p.Transport(NewCustomTransport, WithCustomNode(myCustomProtocol)),
		libp2p.Muxer("/yamux/1.0.0", yamux.DefaultTransport),
	)
	if err != nil {
		return nil, err
	}

	myCustomProtocol.setParentID(h.ID())

	return h, nil
}
