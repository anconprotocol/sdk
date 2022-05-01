package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	ibc "github.com/cosmos/ibc-go/v2/modules/core/23-commitment/types"

	ics23 "github.com/confio/ics23/go"
	cosmosiavl "github.com/cosmos/cosmos-sdk/store/iavl"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	"github.com/cosmos/iavl"
	pb "github.com/cosmos/iavl/proto"
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
	dbm "github.com/tendermint/tm-db"
)

const (
	LINK_PROTO_VERSION = 1
)

type Storage struct {
	dataStore  dbm.DB
	LinkSystem linking.LinkSystem
	RootHash   cidlink.Link
	iavlstore  *cosmosiavl.Store
	rwLock     sync.RWMutex
	tree       *iavl.MutableTree
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

func NewStorage(db dbm.DB, version int64, cacheSize int) Storage {
	tree, err := iavl.NewMutableTree(db, int(cacheSize))
	iavlStore := cosmosiavl.UnsafeNewStore(tree)

	if err != nil {
		panic(err)
	}

	if _, err := tree.LoadVersion(version); err != nil {
		panic(err)
	}

	lsys := cidlink.DefaultLinkSystem()
	s := Storage{
		dataStore:  db,
		rwLock:     sync.RWMutex{},
		tree:       tree,
		iavlstore:  iavlStore,
		LinkSystem: lsys,
	}
	//   you just need a function that conforms to the ipld.BlockWriteOpener interface.
	lsys.StorageWriteOpener = func(lnkCtx ipld.LinkContext) (io.Writer, ipld.BlockWriteCommitter, error) {

		// change prefix
		buf := bytes.Buffer{}
		return &buf, func(lnk ipld.Link) error {

			s.rwLock.Lock()
			defer s.rwLock.Unlock()

			path := []byte(lnkCtx.LinkPath.String())

			kvs := prefix.NewStore(iavlStore, path)

			kvs.Set([]byte(lnk.String()), buf.Bytes())

			return nil

		}, nil
	}
	lsys.StorageReadOpener = func(lnkCtx ipld.LinkContext, lnk ipld.Link) (io.Reader, error) {
		s.rwLock.Lock()
		defer s.rwLock.Unlock()

		path := []byte(lnkCtx.LinkPath.String())

		kvs := prefix.NewStore(iavlStore, path)
		value := kvs.Get([]byte(lnk.String()))
		return bytes.NewReader(value), err
	}

	lsys.TrustedStorage = true
	return s
}

func (s *Storage) Get(path []byte, id string) ([]byte, error) {
	s.rwLock.Lock()
	defer s.rwLock.Unlock()

	kvs := prefix.NewStore(s.iavlstore, path)
	value := kvs.Get([]byte(id))
	return value, nil
}

func (s *Storage) Remove(path []byte, id string) error {
	s.rwLock.Lock()
	defer s.rwLock.Unlock()

	kvs := prefix.NewStore(s.iavlstore, path)
	kvs.Delete([]byte(id))
	return nil
}

func (s *Storage) Put(path []byte, id string, data []byte) (err error) {
	s.rwLock.Lock()
	defer s.rwLock.Unlock()

	kvs := prefix.NewStore(s.iavlstore, path)

	kvs.Set([]byte(id), data)

	return nil
}

func (s *Storage) Iterate(path []byte, start, end []byte) (dbm.Iterator, error) {
	s.rwLock.Lock()
	defer s.rwLock.Unlock()

	kvs := prefix.NewStore(s.iavlstore, path)

	return kvs.Iterator(start, end), nil
}

func (s *Storage) Has(path []byte, id []byte) (bool, error) {
	s.rwLock.Lock()
	defer s.rwLock.Unlock()

	kvs := prefix.NewStore(s.iavlstore, path)

	return kvs.Has(id), nil
}

func (s *Storage) GetTreeHeight() int8 {
	return s.tree.Height()
}

func (s *Storage) GetTreeHash() []byte {
	return s.tree.Hash()
}

func (s *Storage) GetTreeVersion() int64 {
	return s.tree.Version()
}

// SaveVersion saves a new IAVL tree version to the DB based on the current
// state (version) of the tree. It returns a result containing the hash and
// new version number.
func (s *Storage) Commit() (*pb.SaveVersionResponse, error) {

	s.rwLock.Lock()
	defer s.rwLock.Unlock()

	root, version, err := s.tree.SaveVersion()
	if err != nil {
		return nil, err
	}

	res := &pb.SaveVersionResponse{RootHash: root, Version: version}

	return res, nil
}

/*
CreateMembershipProof will produce a CommitmentProof that the given key (and queries value) exists in the iavl tree.
If the key doesn't exist in the tree, this will return an error.
*/
func createMembershipProof(tree *iavl.MutableTree, key []byte, exist *ics23.ExistenceProof) (*ics23.CommitmentProof, error) {
	// exist, err := createExistenceProof(tree, key)
	proof := &ics23.CommitmentProof{
		Proof: &ics23.CommitmentProof_Exist{
			Exist: exist,
		},
	}
	return proof, nil
	// return ics23.CombineProofs([]*ics23.CommitmentProof{proof})
}

// GetWithProof returns a result containing the IAVL tree version and value for
// a given key based on the current state (version) of the tree including a
// verifiable Merkle proof.
func (s *Storage) GetWithProof(key []byte) (json.RawMessage, error) {

	s.rwLock.RLock()
	defer s.rwLock.RUnlock()

	res := make(map[string]interface{})
	var err error
	var proof *iavl.RangeProof

	value, proof, err := s.tree.GetWithProof(key)
	if err != nil {
		return nil, err
	}

	if value == nil {
		s := fmt.Errorf("The key requested does not exist")
		return nil, s
	}

	exp, err := convertExistenceProof(proof, key, value)
	if err != nil {
		return nil, err
	}

	memproof, err := createMembershipProof(s.tree, key, exp)
	if err != nil {
		return nil, err
	}

	memproofbyte, err := memproof.Marshal()
	if err != nil {
		return nil, err
	}
	exproof := &ics23.CommitmentProof{}
	err = exproof.Unmarshal(memproofbyte)

	if err != nil {
		return nil, err
	}

	mp := &ibc.MerkleProof{
		Proofs: []*ics23.CommitmentProof{exproof},
	}
	res["proof"] = mp
	res["value"] = value

	hexres, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}

	return hexres, nil
}

// GetCommitmentProof returns a result containing the IAVL tree version and value for
// a given key based on the current state (version) of the tree including a
// verifiable existing or not existing Commitment proof.
func (s *Storage) GetCommitmentProof(key []byte, version int64) (json.RawMessage, error) {

	s.rwLock.RLock()
	defer s.rwLock.RUnlock()

	if s.tree.VersionExists(version) {
		t, err := s.tree.GetImmutable(version)
		if err != nil {
			return nil, err
		}

		existenceProof, err := t.GetMembershipProof(key)
		if err != nil {
			return nil, err
		}

		if existenceProof == nil {
			s := fmt.Errorf("The key requested does not exist")
			return nil, s
		}

		nonMembershipProof, err := t.GetNonMembershipProof(key)

		mp := &ibc.MerkleProof{
			Proofs: []*ics23.CommitmentProof{existenceProof, nonMembershipProof},
		}

		hexres, err := json.Marshal(mp.Proofs)
		if err != nil {
			return nil, err
		}

		return hexres, nil
	}

	return nil, fmt.Errorf("invalid version")
}

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
