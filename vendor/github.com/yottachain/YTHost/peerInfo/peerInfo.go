package peerInfo

import (
	"fmt"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"strings"
)

type PeerInfo struct {
	ID    peer.ID
	Addrs []multiaddr.Multiaddr
}

func (pi *PeerInfo) List() []multiaddr.Multiaddr {
	var mas = make([]multiaddr.Multiaddr, len(pi.Addrs))
	peerMa, err := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", pi.ID))
	if err != nil {
		return nil
	}
	for k, v := range pi.Addrs {
		mas[k] = v.Encapsulate(peerMa)
	}

	return mas
}

func (pi *PeerInfo) StringList() []string {
	var list = pi.List()
	var mastr = make([]string, len(list))
	for k, v := range list {
		mastr[k] = strings.Replace(v.String(), "ipfs", "p2p", 1)
	}
	return mastr
}
