package main

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-msgio/pbio"
	"github.com/multiformats/go-multiaddr"
)

const customProtocolID = "/custom/1.0.0"

type CustomProtocol struct {
	host host.Host

	mutex   sync.Mutex
	hasConn bool
	origin  multiaddr.Multiaddr

	parentID          peer.ID
	C                 chan *Payload
	listOfCustomNodes []multiaddr.Multiaddr
}

func NewCustomProtocol(host host.Host, listOfCustomNodes []multiaddr.Multiaddr) (*CustomProtocol, error) {
	for _, x := range listOfCustomNodes {
		peerID, err := GetPeerID(x)
		if err != nil {
			return nil, err
		}

		host.Peerstore().AddAddrs(peerID, []multiaddr.Multiaddr{x}, peerstore.PermanentAddrTTL)
	}
	return &CustomProtocol{
		host:              host,
		C:                 make(chan *Payload),
		listOfCustomNodes: listOfCustomNodes,
	}, nil
}

func (m *CustomProtocol) Conn() *CustomConn {
	for {
		if m.hasConn {
			ctx, cancel := context.WithCancel(context.Background())
			return &CustomConn{
				ctx:          ctx,
				cancel:       cancel,
				myCustomNode: m,
				remotePeer:   m.origin,
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (m *CustomProtocol) setParentID(p peer.ID) {
	m.parentID = p
}

func (m *CustomProtocol) GetRandomPath(destination multiaddr.Multiaddr) []string {
	if m.parentID == "" {
		panic("parentID not set")
	}

	rand.Shuffle(len(m.listOfCustomNodes), func(i, j int) {
		m.listOfCustomNodes[i], m.listOfCustomNodes[j] = m.listOfCustomNodes[j], m.listOfCustomNodes[i]
	})

	fmt.Println("DESTINATION", destination)

	// Remove both the origin and destination
	var filteredSlice []multiaddr.Multiaddr
	destinationPeerIdStr, _ := destination.ValueForProtocol(P_CUSTOM)
	destinationPeerId, _ := peer.Decode(destinationPeerIdStr)
	for _, v := range m.listOfCustomNodes {
		vpeerID, _ := GetPeerID(v)
		if vpeerID == m.host.ID() || vpeerID == destinationPeerId {
			continue
		}
		filteredSlice = append(filteredSlice, v)
	}

	// Return random 3 nodes
	n := 1
	if len(filteredSlice) < n {
		panic("More nodes required!!!")
	}

	destinationStrParts := strings.Split(destination.String(), "/p2p")
	destinationStr := strings.Replace(destinationStrParts[0], "/custom", "/p2p", -1)
	destinationMa, _ := multiaddr.NewMultiaddr(destinationStr)

	resultMa := append(filteredSlice[:n], destinationMa)
	result := make([]string, len(resultMa))
	for i, addr := range resultMa {
		result[i] = addr.String()
	}

	fmt.Println("THE RANDOM PATH: ", result)

	return result
}

func (m *CustomProtocol) Start(ctx context.Context) {

	m.host.SetStreamHandler(customProtocolID, func(s network.Stream) {
		fmt.Println("I am ", m.host.ID(), "and received request from ", s.Conn().RemotePeer())
		reader := pbio.NewDelimitedReader(s, math.MaxInt32)

		message := &Message{}
		reader.ReadMsg(message)

		if len(message.Path) > 0 {
			nextNode := message.Path[0]
			nextNodeMA, err := multiaddr.NewMultiaddr(nextNode)
			if err != nil {
				fmt.Println("ERROR building peerID!!!", err)
				s.Reset()
				return
			}

			peerID, err := GetPeerID(nextNodeMA)
			if err != nil {
				fmt.Println("ERROR obtaining peerID!!!", err)
				s.Reset()
				return
			}

			m.host.Peerstore().AddAddr(peerID, nextNodeMA, peerstore.PermanentAddrTTL)

			message := &Message{
				Path:    message.Path[1:], // Remove current node from the path
				Payload: message.Payload,
			}

			err = m.Send(ctx, nextNodeMA.String(), message)
			if err != nil {
				fmt.Println("ERROR sending message", err)
				s.Reset()
				return
			}
		} else {
			fmt.Println("PUSHING TO CHANNEL IN", m.host.ID(), string(message.Payload.Payload))

			m.mutex.Lock()
			if !m.hasConn {
				m.origin, _ = multiaddr.NewMultiaddr(message.Payload.Origin)
				m.hasConn = true
			}
			m.mutex.Unlock()

			m.C <- message.Payload

			fmt.Println("PUSHED TO CHANNEL  IN", m.host.ID())
		}

		s.Close()
	})
}

func (m *CustomProtocol) Write(stream network.Stream, message *Message) error {
	writer := pbio.NewDelimitedWriter(stream)
	err := writer.WriteMsg(message)
	if err != nil {
		stream.Reset()
		return err
	}
	return nil
}

func (m *CustomProtocol) Send(ctx context.Context, nextHopStr string, message *Message) error {
	nextHop, err := multiaddr.NewMultiaddr(nextHopStr)
	if err != nil {
		return err
	}

	peerID, err := GetPeerID(nextHop)
	if err != nil {
		return err
	}

	fmt.Println("Send: I am", m.host.ID())
	fmt.Println("Send: the next hop:", nextHop)

	stream, err := m.host.NewStream(ctx, peerID, customProtocolID)
	if err != nil {
		return nil
	}

	writer := pbio.NewDelimitedWriter(stream)

	message = &Message{
		Path:    message.Path, // Remove current node from the path
		Payload: message.Payload,
	}

	fmt.Println("Send: Now writing msg to", peerID)
	err = writer.WriteMsg(message)
	if err != nil {
		stream.Reset()
		return err
	}

	fmt.Println("Closing in ", m.host.ID())
	return stream.Close()
}
