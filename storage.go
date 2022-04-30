package sdk

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync/ipldutil"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/linking"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/multiformats/go-multihash"
)

const (
	LINK_PROTO_VERSION = 1
)

type Storage struct {
	dataStore  *pebble.DB
	LinkSystem linking.LinkSystem
	RootHash   cidlink.Link
}

func (s *Storage) InitGenesis(moniker []byte) {
	key, _ := crypto.GenerateKey()
	digest := crypto.Keccak256(moniker)
	var buf bytes.Buffer
	buf.WriteString(time.Now().GoString())
	root, err := key.Sign(&buf, digest, nil)
	if err != nil {
		panic(err)
	}
	n, err := ipldutil.DecodeNode(root)

	if err != nil {
		panic(err)
	}
	link := s.Store(ipld.LinkContext{}, n)
	fmt.Printf("root genesis is %v\n", link)
}

func (s *Storage) LoadGenesis(cid string) {
	r, err := ParseCidLink(cid)
	if err != nil {
		panic(err)
	}
	s.RootHash = r
}

func NewStorage(folder string) Storage {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	DefaultNodeHome := filepath.Join(userHomeDir, folder)
	db, err := pebble.Open(DefaultNodeHome, &pebble.Options{})

	if err != nil {
		log.Fatal(err)
	}

	// store.InitDefaults(DefaultNodeHome)
	lsys := cidlink.DefaultLinkSystem()
	//   you just need a function that conforms to the ipld.BlockWriteOpener interface.
	lsys.StorageWriteOpener = func(lnkCtx ipld.LinkContext) (io.Writer, ipld.BlockWriteCommitter, error) {
		// change prefix
		buf := bytes.Buffer{}
		return &buf, func(lnk ipld.Link) error {
			key := []byte(lnkCtx.LinkPath.String() + lnk.String())
			return db.Set(key, buf.Bytes(), pebble.Sync)
		}, nil
	}
	lsys.StorageReadOpener = func(lnkCtx ipld.LinkContext, lnk ipld.Link) (io.Reader, error) {
		key := []byte(lnkCtx.LinkPath.String() + lnk.String())

		value, closer, err := db.Get(key)
		defer closer.Close()

		return bytes.NewReader(value), err
	}

	defer db.Close()
	lsys.TrustedStorage = true
	return Storage{
		dataStore:  db,
		LinkSystem: lsys,
	}
}
func (s *Storage) Get(path, id string) ([]byte, error) {

	key := []byte(path + id)

	value, closer, err := s.dataStore.Get(key)
	defer closer.Close()

	return value, err

}
func (s *Storage) Remove(path, id string) error {
	key := []byte(path + id)

	err := s.dataStore.Delete(key, pebble.Sync)

	return err
}

func (s *Storage) Put(path, id string, data []byte) (err error) {

	key := []byte(path + id)

	err = s.dataStore.Set(key, data, pebble.Sync)

	return err

}

// func (s *Storage) Filter(path string, limit int, reverse bool) (ac [][]byte, err error) {

// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	key := []byte(path)
// 	tup := tuple.Tuple{key}
// 	ss := s.directory.Sub(tup)
// 	tr, err := s.dataStore.CreateTransaction()

// 	var items [][]byte
// 	ri := tr.GetRange(ss, fdb.RangeOptions{
// 		Reverse: reverse,
// 		Limit:   limit,
// 	}).Iterator()
// 	for ri.Advance() {
// 		kv := ri.MustGet()
// 		t, err := ss.Unpack(kv.Key)
// 		if err != nil {
// 			return nil, err
// 		}
// 		items = append(items, t[0].([]byte))
// 	}
// 	return items, nil
// }

// eth-block	ipld	0x90	permanent	Ethereum Header (RLP)
// eth-block-list	ipld	0x91	permanent	Ethereum Header List (RLP)
// eth-tx-trie	ipld	0x92	permanent	Ethereum Transaction Trie (Eth-Trie)
// eth-tx	ipld	0x93	permanent	Ethereum Transaction (MarshalBinary)
// eth-tx-receipt-trie	ipld	0x94	permanent	Ethereum Transaction Receipt Trie (Eth-Trie)
// eth-tx-receipt	ipld	0x95	permanent	Ethereum Transaction Receipt (MarshalBinary)
// eth-state-trie	ipld	0x96	permanent	Ethereum State Trie (Eth-Secure-Trie)
// eth-account-snapshot	ipld	0x97	permanent	Eth	ereum Account Snapshot (RLP)
// eth-storage-trie	ipld	0x98	permanent	Ethereum Contract Storage Trie (Eth-Secure-Trie)
// eth-receipt-log-trie	ipld	0x99	draft	Ethereum Transaction Receipt Log Trie (Eth-Trie)
// eth-reciept-log	ipld	0x9a	draft	Ethereum Transaction Receipt Log (RLP)
var (
	DagEthCodecs map[string]uint64 = make(map[string]uint64)
)

func init() {
	DagEthCodecs["eth-block"] = 0x90
}

func GetDagEthereumLinkPrototype(codec string) ipld.LinkPrototype {
	return cidlink.LinkPrototype{cid.Prefix{
		Version:  LINK_PROTO_VERSION,
		Codec:    DagEthCodecs[codec],
		MhType:   multihash.SHA2_256, // sha2-256
		MhLength: 32,                 // sha2-256 hash has a 32-byte sum.
	}}
}

func GetDagCBORLinkPrototype() ipld.LinkPrototype {
	return cidlink.LinkPrototype{cid.Prefix{
		Version:  LINK_PROTO_VERSION,
		Codec:    cid.DagCBOR,        // dag-cbor
		MhType:   multihash.SHA2_256, // sha2-256
		MhLength: 32,                 // sha2-256 hash has a 32-byte sum.
	}}
}

func GetDagJSONLinkPrototype() ipld.LinkPrototype {
	return cidlink.LinkPrototype{cid.Prefix{
		Version:  LINK_PROTO_VERSION,
		Codec:    0x0129,             // dag-json
		MhType:   multihash.SHA2_256, // sha2-256
		MhLength: 32,                 // sha2-256 hash has a 32-byte sum.
	}}
}

func GetDagJOSELinkPrototype() ipld.LinkPrototype {
	return cidlink.LinkPrototype{cid.Prefix{
		Version:  LINK_PROTO_VERSION,
		Codec:    cid.DagJOSE,        // dag-json
		MhType:   multihash.SHA2_256, // sha2-256
		MhLength: 32,                 // sha2-256 hash has a 32-byte sum.
	}}
}

func GetRawLinkPrototype() ipld.LinkPrototype {
	return cidlink.LinkPrototype{cid.Prefix{
		Version:  LINK_PROTO_VERSION,
		Codec:    0x55,               // dag-json
		MhType:   multihash.SHA2_256, // sha2-256
		MhLength: 32,                 // sha2-256 hash has a 32-byte sum.
	}}
}

// Store node as  dag-json
func (k *Storage) Store(linkCtx ipld.LinkContext, node datamodel.Node) datamodel.Link {
	return k.LinkSystem.MustStore(linkCtx, GetDagJSONLinkPrototype(), node)
}

// Load node from  dag-json
func (k *Storage) Load(linkCtx ipld.LinkContext, link datamodel.Link) (datamodel.Node, error) {
	np := basicnode.Prototype.Any
	node, err := k.LinkSystem.Load(linkCtx, link, np)
	if err != nil {
		return nil, err
	}

	return node, nil
}

// Store node as  dag-cbor
func (k *Storage) StoreDagCBOR(linkCtx ipld.LinkContext, node datamodel.Node) datamodel.Link {
	return k.LinkSystem.MustStore(linkCtx, GetDagCBORLinkPrototype(), node)
}

// Store node as  raw
func (k *Storage) StoreRaw(linkCtx ipld.LinkContext, node datamodel.Node) datamodel.Link {
	return k.LinkSystem.MustStore(linkCtx, GetRawLinkPrototype(), node)
}

// Store node as  dag-eth
func (k *Storage) StoreDagEth(linkCtx ipld.LinkContext, node datamodel.Node, codecFormat string) datamodel.Link {
	return k.LinkSystem.MustStore(linkCtx, GetDagEthereumLinkPrototype(codecFormat), node)
}

func Encode(n datamodel.Node) (string, error) {
	var sb strings.Builder
	err := dagjson.Encode(n, &sb)
	return sb.String(), err
}

func Decode(proto datamodel.NodePrototype, src string) (datamodel.Node, error) {
	nb := proto.NewBuilder()
	err := dagjson.Decode(nb, strings.NewReader(src))
	return nb.Build(), err
}

func EncodeCBOR(n datamodel.Node) ([]byte, error) {
	var sb bytes.Buffer
	err := dagcbor.Encode(n, &sb)
	return sb.Bytes(), err
}

func DecodeCBOR(proto datamodel.NodePrototype, src []byte) (datamodel.Node, error) {
	nb := proto.NewBuilder()
	err := dagcbor.Decode(nb, bytes.NewReader(src))
	return nb.Build(), err
}
