package clientStore

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/mr-tron/base58"
	"github.com/multiformats/go-multiaddr"
	"github.com/yottachain/YTHost/client"
	"sync"
)

type ClientStore struct {
	connect func(ctx context.Context, id peer.ID, mas []multiaddr.Multiaddr) (*client.YTHostClient, error)
	q       chan struct{}
	sync.Map
	sync.Mutex
	MtxMap sync.Map
	IdLockMap map[peer.ID] sync.Mutex
}

// Get 获取一个客户端，如果没有，建立新的客户端连接
func (cs *ClientStore) Get(ctx context.Context, pid peer.ID, mas []multiaddr.Multiaddr) (*client.YTHostClient, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("ctx done")
	default:
		return cs.get(ctx, pid, mas)
	}
}

func (cs *ClientStore) get(ctx context.Context, pid peer.ID, mas []multiaddr.Multiaddr) (*client.YTHostClient, error) {
	cs.q <- struct {
	}{}
	defer func() {
		<-cs.q
	}()

	//actul, _ := cs.MtxMap.LoadOrStore(pid, &sync.Mutex{})
	//mux := actul.(*sync.Mutex)
	//mux.Lock()
	//defer mux.Unlock()

	// 尝试次数
	var tryCount int
	const max_try_count = 5

	cs.Lock()
	idLock, ok := cs.IdLockMap[pid]
	if !ok {
		cs.IdLockMap[pid] = sync.Mutex{}
		idLock, _ = cs.IdLockMap[pid]
	}
	cs.Unlock()

	idLock.Lock()
	defer idLock.Unlock()

	// 取已存在clt
start:
	// 如果达到最大尝试次数就返回错误
	if tryCount++; tryCount > max_try_count {
		return nil, fmt.Errorf("Maximum attempts %d ", max_try_count)
	}
	_c, ok := cs.Map.Load(pid)
	// 如果不存在创建新的clt
	if !ok {
		if clt, err := cs.connect(ctx, pid, mas); err != nil {
			return nil, err
		} else {
			cs.Map.Store(pid, clt)
			// 创建clt完成后返回到开始
			goto start
		}
	} else {
		// 如果已存在clt无法ping通,删除记录重新创建
		c := _c.(*client.YTHostClient)
		if c.IsClosed() || !c.Ping(ctx) {
		//if c.IsClosed() {
			c.Close()
			cs.Map.Delete(pid)
			goto start
		}

		return c, nil
	}
}

func (cs *ClientStore) GetByAddrString(ctx context.Context, id string, addrs []string) (*client.YTHostClient, error) {
	buf, _ := base58.Decode(id)
	pid, err := peer.IDFromBytes(buf)
	if err != nil {
		return nil, err
	}

	var mas = make([]multiaddr.Multiaddr, len(addrs))
	for k, v := range addrs {
		ma, err := multiaddr.NewMultiaddr(v)
		if err != nil {
			continue
		}
		mas[k] = ma
	}

	return cs.get(ctx, pid, mas)
}

// Close 关闭一个客户端
func (cs *ClientStore) Close(pid peer.ID) error {
	cs.Lock()
	defer cs.Unlock()

	_clt, ok := cs.Load(pid)
	if !ok {
		return fmt.Errorf("no find client ID is %s", pid.Pretty())
	}
	clt := _clt.(*client.YTHostClient)

	cs.Map.Delete(pid)
	return clt.Close()
}

func (cs *ClientStore) GetClient(pid peer.ID) (*client.YTHostClient, bool) {

	_clt, ok := cs.Map.Load(pid)
	if ok {
		clt := _clt.(*client.YTHostClient)
		return clt, ok
	}
	return nil, ok
}

// Len 返回当前连接数
//func (cs *ClientStore) Len() int {
//}

func NewClientStore(connFunc func(ctx context.Context, id peer.ID, mas []multiaddr.Multiaddr) (*client.YTHostClient, error)) *ClientStore {
	return &ClientStore{
		connFunc,
		make(chan struct{}, 1000),
		sync.Map{},
		sync.Mutex{},
		sync.Map{},
		make(map[peer.ID] sync.Mutex),
	}
}
