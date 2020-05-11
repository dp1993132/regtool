package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"fmt"
	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/ecc"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/viper"
	register_api "github.com/yottachain/YTDataNode/cmd/register-api"
	"github.com/yottachain/YTDataNode/commander"
	"github.com/dp1993132/RegisterWarp/configgo"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"strings"
	"time"
)

var maxSpace int64 = 4096
var key1 string // 抵押私钥
var key2 string // 矿机管理员私钥
var key3 string // 矿池私钥

var depAN string        //抵押账号名
var adminAN string      //管理员账号名
var poolAdminAN string  //矿池名
var beneficialAN string //收益账号
var envs []string

var configPath string
var ytfsNodePath string
var ytfsSignerPath string
var updateUrl string

var minerId uint64

var snList = []string{
	"49.234.139.206",
	"129.211.72.15",
	"122.152.203.189",
	"212.129.153.253",
	"49.235.52.30",
}

var snmaddr []configgo.PeerInfo

var baseNodeUrl = "http://49.234.139.206:8888"
var api *register_api.API

var currsn string

var aesKey=[]byte("YottaChainNB@)!@")

func main() {

	configPath = os.Args[1]
	file, err := os.OpenFile(configPath, os.O_CREATE|os.O_RDONLY, 0644)
	if err != nil {
		fmt.Println(err)
	}
	defer file.Close()

	ext :=path.Ext(configPath)
	dir := path.Dir(configPath)

	if ext == ".yaml" {
		buf,err := ioutil.ReadAll(file)
		if err != nil {
			panic(err)
		}
		data:=encryptAES(buf,aesKey)

		fi,err:= os.OpenFile(path.Join(dir,"config"),os.O_CREATE|os.O_RDWR,0777)
		if err != nil {
			panic(err)
		}
		defer fi.Close()

		fi.Write(data)

		fmt.Println("已生成配置文件")
	} else {
		GetConfig(file)
		register()
		addPool()

	}
}

func init() {
}

func GetConfig(file *os.File){
	viper.SetConfigType("yaml")

	data:=bytes.NewBuffer([]byte{})

	buf,err:=ioutil.ReadAll(file)
	if err!=nil{
		panic(err)
	}

	data.Write(decryptAES(buf,aesKey))

	err= viper.ReadConfig(data)
	if err != nil {
		fmt.Println(err)
	}
	maxSpace = viper.GetInt64("maxSpace")
	if maxSpace <= 0 {
		maxSpace = 20
	}
	key1 = viper.GetString("dep.key")
	key2 = viper.GetString("miner.key")
	key3 = viper.GetString("pool.key")

	depAN = viper.GetString("dep.name")
	adminAN = viper.GetString("miner.name")
	poolAdminAN = viper.GetString("pool.name")
	beneficialAN = viper.GetString("beneficial.name")

	ytfsNodePath = viper.GetString("ytfsNodePath")
	ytfsSignerPath = viper.GetString("signerPath")

	envs = viper.GetStringSlice("env")
	for _, v := range envs {
		str := strings.Split(v, "=")
		os.Setenv(str[0], str[1])
	}

	snList = viper.GetStringSlice("snips")
	fmt.Println(snList)
	if maddrs:=viper.GetStringSlice("snmaddrs");len(maddrs)>0{
		snmaddr = make([]configgo.PeerInfo,len(maddrs))
		for k,v:=range maddrs{
			ma,err:=multiaddr.NewMultiaddr(v)
			if err != nil {
				continue
			}
			pi,err:=peer.AddrInfoFromP2pAddr(ma)
			if err != nil {
				continue
			}
			snmaddr[k]= configgo.PeerInfo{pi.ID.Pretty(),[]string{pi.Addrs[0].String()}}
		}
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	snindex := r.Int() % len(snList)
	currsn = snList[snindex]
	fmt.Println("当前sn",currsn)

	if bu := viper.GetString("baseNodeUrl"); bu != "" {
		baseNodeUrl = bu
	}
	api = &register_api.API{
		eos.New(baseNodeUrl),
		currsn,
	}

	updateUrl = viper.GetString("updateUrl")

	fmt.Println("最大空间:", maxSpace)
	fmt.Println("抵押账号:", depAN)
	fmt.Println("矿机管理员:", adminAN)
	fmt.Println("矿池管理员:", poolAdminAN)
	fmt.Println("收益账号:", beneficialAN)
	fmt.Println("ytfs-node 路径:", ytfsNodePath)
	fmt.Println("ytfs-signer 路径:", ytfsSignerPath)
	fmt.Println("开始注册...")
}

func register() {

	err := commander.InitBySignleStorage(uint64(maxSpace*(1<<30)), 1<<14)
	if err != nil {
		fmt.Println("初始化失败", err)
	}

	cfg, err := configgo.ReadConfig()
	if err != nil {
		fmt.Println(err)
	}

	id, err := api.GetNewMinerID()
	fmt.Println(id, err)
	minerId = id
	md := register_api.MinerData{
		id,
		eos.AN(adminAN),
		eos.AN(depAN),
		register_api.NewYTAAssect(maxSpace),
		cfg.PubKey,
	}
	action := &eos.Action{
		Account: eos.AN("hddpool12345"),
		Name:    eos.ActN("newminer"),
		Authorization: []eos.PermissionLevel{
			{Actor: eos.AN(depAN), Permission: eos.PN("active")},
		},
		ActionData: eos.NewActionData(md),
	}
	txOpts := &eos.TxOptions{}
	txOpts.FillFromChain(api.API)
	tx := eos.NewSignedTransaction(eos.NewTransaction([]*eos.Action{action}, txOpts))
	tx2 := signer(tx, key1)
	err = api.PushTransactionToSN(tx2, ":8082/preregnode")
	if err != nil {
		fmt.Println(err)
	}
	cfg.Adminacc = adminAN
	cfg.IndexID = uint32(md.MinerID)

	if len(os.Args) == 3 && os.Args[2] != "" {
		cfg.Storages[0].StorageName = os.Args[2]
		cfg.Storages[0].StorageType = 1
	}

	cfg.UpdateURL = updateUrl
	cfg.BPList = snmaddr
	cfg.Save()
}
func addPool() {
	cfg, err := configgo.ReadConfig()
	if err != nil {
		fmt.Println(err)
	}
	pi, err := getPoolInfo(poolAdminAN)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	apd := register_api.ADDPoolData{
		minerId,
		eos.AN(pi[0].PoolID),
		eos.AN(adminAN),
		uint64(maxSpace * (1 << 30) / (1 << 14)),
	}
	action := eos.Action{
		Account: eos.AN("hddpool12345"),
		Name:    eos.ActN("addm2pool"),
		Authorization: []eos.PermissionLevel{
			{Actor: eos.AN(adminAN), Permission: eos.PN("active")},
			{Actor: eos.AN(pi[0].PoolOwner), Permission: eos.PN("active")},
		},
		ActionData: eos.NewActionData(apd),
	}

	txOpts := &eos.TxOptions{}
	txOpts.FillFromChain(api.API)
	tx := eos.NewSignedTransaction(eos.NewTransaction([]*eos.Action{&action}, txOpts))
	tx2 := signer(tx, key2, key3)


	// 如果两个账号一样，使用一个签名就可以了
	if adminAN == pi[0].PoolOwner {
		action.Authorization = []eos.PermissionLevel{
			{Actor: eos.AN(adminAN), Permission: eos.PN("active")},
		}
		tx2 = signer(tx, key2)
	}

	err = api.PushTransactionToSN(tx2, ":8082/changeminerpool")
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("初始化成功")
	}
	cfg.PoolID = poolAdminAN
	cfg.Save()
}

func signer(tx *eos.SignedTransaction, keys ...string) *eos.SignedTransaction {
	var kb = eos.NewKeyBag()
	for _, v := range keys {
		if err := kb.ImportPrivateKey(v); err != nil {
			fmt.Println(err)
		}
	}
	txopts := &eos.TxOptions{}
	txopts.FillFromChain(api.API)
	res, err := kb.Sign(tx, txopts.ChainID, getPubkey(kb)...)
	if err != nil {
		fmt.Println("签名失败:", err)
	} else {
		buf, _ := json.Marshal(res)
		fmt.Println(string(buf))
	}

	return res
}

func getPubkey(kb *eos.KeyBag) []ecc.PublicKey {
	var pkeys = make([]ecc.PublicKey, len(kb.Keys))
	for k, v := range kb.Keys {
		pkeys[k] = v.PublicKey()
	}
	return pkeys
}

type PoolInfo []struct {
	PoolID    string `json:"pool_id"`
	PoolOwner string `json:"pool_owner"`
	MaxSpace  uint64 `json:"max_space"`
}

func getPoolInfo(poolID string) (PoolInfo, error) {
	out, err := api.GetTableRows(eos.GetTableRowsRequest{
		Code:       "hddpool12345",
		Scope:      "hddpool12345",
		Table:      "storepool",
		Index:      "1",
		Limit:      1,
		LowerBound: poolID,
		UpperBound: poolID,
		JSON:       true,
		KeyType:    "name",
	})
	if err != nil {
		return nil, err
	}
	var res PoolInfo
	json.Unmarshal(out.Rows, &res)
	return res, nil
}

func padding(src []byte,blocksize int) []byte {
	padnum:=blocksize-len(src)%blocksize
	pad:=bytes.Repeat([]byte{byte(padnum)},padnum)
	return append(src,pad...)
}

func unpadding(src []byte) []byte {
	n:=len(src)
	unpadnum:=int(src[n-1])
	return src[:n-unpadnum]
}

func encryptAES(src []byte,key []byte) []byte {
	block,_:=aes.NewCipher(key)
	src=padding(src,block.BlockSize())
	blockmode:=cipher.NewCBCEncrypter(block,key)
	blockmode.CryptBlocks(src,src)
	return src
}

func decryptAES(src []byte,key []byte) []byte {
	block,_:=aes.NewCipher(key)
	blockmode:=cipher.NewCBCDecrypter(block,key)
	blockmode.CryptBlocks(src,src)
	src=unpadding(src)
	return src
}
