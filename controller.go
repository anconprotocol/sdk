package sdk

import (
	"crypto/ecdsa"

	"github.com/ipfs/go-graphsync"
	"github.com/libp2p/go-libp2p-core/peer"
)

type AnconSyncContext struct {
	Store      Storage
	Exchange   graphsync.GraphExchange
	IPFSPeer   *peer.AddrInfo
	PrivateKey *ecdsa.PrivateKey
}

func NewAnconSyncContext(s Storage, exchange graphsync.GraphExchange, ipfspeer *peer.AddrInfo) *AnconSyncContext {
	return &AnconSyncContext{
		Store:    s,
		Exchange: exchange,
		IPFSPeer: ipfspeer,
	}
}
