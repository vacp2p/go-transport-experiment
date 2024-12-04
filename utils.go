package main

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

func GetPeerID(m multiaddr.Multiaddr) (peer.ID, error) {
	peerIDStr, err := m.ValueForProtocol(multiaddr.P_P2P)
	if err != nil {
		return "", err
	}

	peerID, err := peer.Decode(peerIDStr)
	if err != nil {
		return "", err
	}

	return peerID, nil
}

func EncapsulatePeerID(peerID peer.ID, addr multiaddr.Multiaddr) multiaddr.Multiaddr {
	hostInfo, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", peerID.String()))
	return addr.Encapsulate(hostInfo)
}
