package host

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	_ "net/http/pprof"
	"net/rpc"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/mr-tron/base58"
	"github.com/multiformats/go-multiaddr"
	mnet "github.com/multiformats/go-multiaddr-net"
	"github.com/yottachain/YTHost/client"
	"github.com/yottachain/YTHost/clientStore"
	"github.com/yottachain/YTHost/config"
	"github.com/yottachain/YTHost/connAutoCloser"
	"github.com/yottachain/YTHost/option"
	"github.com/yottachain/YTHost/peerInfo"
	"github.com/yottachain/YTHost/service"
)

//type Host interface {
//	Accept()
//	Addrs() []multiaddr.Multiaddr
//	Server() *rpc.Server
//	Config() *config.Config
//	Connect(ctx context.Context, pid peer.ID, mas []multiaddr.Multiaddr) (*client.YTHostClient, error)
//	RegisterHandler(id service.MsgId, handlerFunc service.Handler)
//}

type host struct {
	cfg      *config.Config
	listener mnet.Listener
	srv      *rpc.Server
	service.HandlerMap
	clientStore *clientStore.ClientStore
	httpClient  *http.Client
}

func NewHost(options ...option.Option) (*host, error) {
	hst := new(host)
	hst.cfg = config.NewConfig()

	for _, bindOp := range options {
		bindOp(hst.cfg)
	}

	ls, err := mnet.Listen(hst.cfg.ListenAddr)

	if err != nil {
		return nil, err
	}

	hst.listener = ls

	srv := rpc.NewServer()
	hst.srv = srv

	hst.HandlerMap = make(service.HandlerMap)

	hst.clientStore = clientStore.NewClientStore(hst.Connect)

	if hst.cfg.PProf != "" {
		go func() {
			if err := http.ListenAndServe(hst.cfg.PProf, nil); err != nil {
				fmt.Println("PProf open fail:", err)
			} else {
				fmt.Println("PProf debug open:", hst.cfg.PProf)
			}
		}()
	}

	hst.httpClient = &http.Client{}

	return hst, nil
}

func (hst *host) Accept() {
	addrService := new(service.AddrService)
	addrService.Info.ID = hst.cfg.ID
	addrService.Info.Addrs = hst.Addrs()
	addrService.PubKey = hst.Config().Privkey.GetPublic()

	msgService := new(service.MsgService)
	msgService.Handler = hst.HandlerMap
	msgService.Pi = peerInfo.PeerInfo{hst.cfg.ID, hst.Addrs()}

	if err := hst.srv.RegisterName("as", addrService); err != nil {
		panic(err)
	}

	if err := hst.srv.RegisterName("ms", msgService); err != nil {
		panic(err)
	}

	//for {
	//	hst.srv.Accept(mnet.NetListener(hst.listener))
	//}

	lis := mnet.NetListener(hst.listener)
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Print("rpc.Serve: accept:", err.Error())
			continue
		}
		ac := connAutoCloser.New(conn)
		ac.SetOuttime(time.Minute * 5)
		go hst.srv.ServeConn(ac)
	}
}

func (h *host) SendHTTPMsg(ma multiaddr.Multiaddr, mid int32, msg []byte) ([]byte, error) {
	addr, err := ma.ValueForProtocol(multiaddr.P_DNS4)
	if err != nil {
		ip, err := ma.ValueForProtocol(multiaddr.P_IP4)
		if err != nil {
			return nil, err
		}
		addr = ip
	}
	port, err := ma.ValueForProtocol(multiaddr.P_TCP)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s:%s/msg/%d", addr, port, mid), bytes.NewBuffer(msg))
	if err != nil {
		return nil, err
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	respData, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	return respData, err
}

func (hst *host) Listenner() mnet.Listener {
	return hst.listener
}

func (hst *host) Server() *rpc.Server {
	return hst.srv
}

func (hst *host) Config() *config.Config {
	return hst.cfg
}

func (hst *host) ClientStore() *clientStore.ClientStore {
	return hst.clientStore
}

func (hst *host) Addrs() []multiaddr.Multiaddr {

	port, err := hst.listener.Multiaddr().ValueForProtocol(multiaddr.P_TCP)
	if err != nil {
		return nil
	}

	tcpMa, err := multiaddr.NewMultiaddr(fmt.Sprintf("/tcp/%s", port))

	if err != nil {
		return nil
	}

	var res []multiaddr.Multiaddr
	maddrs, err := mnet.InterfaceMultiaddrs()
	if err != nil {
		return nil
	}

	for _, ma := range maddrs {
		newMa := ma.Encapsulate(tcpMa)
		if mnet.IsIPLoopback(newMa) {
			continue
		}
		res = append(res, newMa)
	}
	return res
}

// Connect 连接远程节点
func (hst *host) Connect(ctx context.Context, pid peer.ID, mas []multiaddr.Multiaddr) (*client.YTHostClient, error) {
	conn, err := hst.connect(ctx, pid, mas)
	if err != nil {
		return nil, err
	}

	clt := rpc.NewClient(conn)
	ytclt, err := client.WarpClient(clt, &peer.AddrInfo{
		hst.cfg.ID,
		hst.Addrs(),
	}, hst.cfg.Privkey.GetPublic())
	if err != nil {
		return nil, err
	}
	return ytclt, nil
}

func (hst *host)SendMsgAuto(ctx context.Context, pid peer.ID,mid int32,  ma multiaddr.Multiaddr,msg []byte) ([]byte,error) {
	log.Printf("[YTHost]mid %x, buf len %d\n",mid,len(msg))
	if _,err:=ma.ValueForProtocol(multiaddr.P_HTTP);err ==nil {
		return hst.SendHTTPMsg(ma,mid,msg)
	} else {
		clt,err :=hst.clientStore.Get(ctx,pid,[]multiaddr.Multiaddr{ma})
		if err != nil {
			return nil,err
		}
		return clt.SendMsg(ctx,mid,msg)
	}
}

func (hst *host) connect(ctx context.Context, pid peer.ID, mas []multiaddr.Multiaddr) (mnet.Conn, error) {
	connChan := make(chan mnet.Conn)
	errChan := make(chan error)
	wg := sync.WaitGroup{}
	wg.Add(len(mas))

	go func() {
		wg.Wait()
		select {
		case errChan <- fmt.Errorf("dail all maddr fail"):
		case <-time.After(time.Millisecond * 500):
			return
		}
	}()

	for _, addr := range mas {
		go func(addr multiaddr.Multiaddr) {
			defer wg.Done()
			d := &mnet.Dialer{}
			if conn, err := d.DialContext(ctx, addr); err == nil {
				select {
				case connChan <- conn:
				case <-time.After(time.Second * 30):
				}
			} else {
				if hst.cfg.Debug {
					log.Println("conn error:", err)
				}
			}
		}(addr)
	}

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("ctx quit")
		case conn := <-connChan:
			return conn, nil
		case err := <-errChan:
			return nil, err
		}
	}
}

// ConnectAddrStrings 连接字符串地址
func (hst *host) ConnectAddrStrings(ctx context.Context, id string, addrs []string) (*client.YTHostClient, error) {

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

	return hst.Connect(ctx, pid, mas)
}

// SendMsg 发送消息
func (hst *host) SendMsg(ctx context.Context, pid peer.ID, mid int32, msg []byte) ([]byte, error) {
	clt, ok := hst.ClientStore().GetClient(pid)
	if !ok {
		return nil, fmt.Errorf("no client ID is:%s", pid.Pretty())
	}
	res, err := clt.SendMsg(ctx, mid, msg)
	return res, err
}
