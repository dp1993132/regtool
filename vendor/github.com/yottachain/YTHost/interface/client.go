package YTinterface

import (
	"context"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
)

type YTClient interface {
	SendMsg(ctx context.Context, id int32, data []byte) ([]byte, error)
	RemotePeer() peer.AddrInfo
	RemotePeerPubkey() crypto.PubKey
	LocalPeer() peer.AddrInfo
}
