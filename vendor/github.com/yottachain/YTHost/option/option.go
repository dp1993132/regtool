package option

import (
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/yottachain/YTHost/config"
)

type Option func(cfg *config.Config)

func ListenAddr(ma multiaddr.Multiaddr) Option {
	return func(cfg *config.Config) {
		cfg.ListenAddr = ma
	}
}

func Identity(key crypto.PrivKey) Option {
	return func(cfg *config.Config) {
		cfg.Privkey = key
		id, _ := peer.IDFromPrivateKey(cfg.Privkey)
		cfg.ID = id
	}
}

func OpenPProf(addr string) Option {
	return func(cfg *config.Config) {
		cfg.PProf = addr
	}
}

func OpenDebug() Option {
	return func(cfg *config.Config) {
		cfg.Debug = true
	}
}
