module github.com/anconprotocol/sdk

go 1.16

require (
	github.com/ipfs/go-block-format v0.0.3
	github.com/ipfs/go-cid v0.1.0
	github.com/ipfs/go-graphsync v0.9.3
	github.com/ipld/go-car/v2 v2.0.2
	github.com/ipld/go-ipld-prime v0.14.0
	github.com/libp2p/go-libp2p v0.14.4
	github.com/libp2p/go-libp2p-connmgr v0.2.4
	github.com/libp2p/go-libp2p-core v0.8.6
	github.com/libp2p/go-libp2p-kad-dht v0.13.1
	github.com/libp2p/go-libp2p-noise v0.2.0
	github.com/multiformats/go-multiaddr v0.3.3
)

require (
	github.com/0xPolygon/polygon-sdk v0.0.0-20211207172349-a9ee5ed12815
	github.com/confio/ics23/go v0.6.6
	github.com/cosmos/iavl v0.17.3
	github.com/ethereum/go-ethereum v1.10.13
	github.com/gogo/protobuf v1.3.3 // indirect
	github.com/golang/glog v1.0.0
	github.com/golang/protobuf v1.5.2
	github.com/google/cel-go v0.9.0
	github.com/gopherjs/gopherjs v0.0.0-20200217142428-fce0ec30dd00 // indirect
	github.com/ipfs/go-ipfs-blockstore v1.0.4 // indirect
	github.com/multiformats/go-multihash v0.1.0
	github.com/pkg/errors v0.9.1
	github.com/smartystreets/assertions v1.1.1 // indirect
	github.com/spf13/cast v1.4.1
	github.com/tendermint/tendermint v0.35.0 // indirect
	github.com/tendermint/tm-db v0.6.6
	golang.org/x/net v0.0.0-20211123203042-d83791d6bcd9 // indirect
	google.golang.org/grpc v1.42.0
	google.golang.org/protobuf v1.27.1
)

replace github.com/libp2p/go-libp2p => github.com/libp2p/go-libp2p v0.14.1

replace github.com/libp2p/go-libp2p-core v0.10.0 => github.com/libp2p/go-libp2p-core v0.9.0

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
