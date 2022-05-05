package sdk

import (
	"github.com/ipld/go-ipld-prime/must"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

type AnconAppChain struct {
	storage *Storage
}

var _ abcitypes.Application = (*AnconAppChain)(nil)

func NewAnconAppChain(storage *Storage) *AnconAppChain {
	return &AnconAppChain{
		storage: storage,
	}
}

func (app *AnconAppChain) SetOption(req abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
	return abcitypes.ResponseSetOption{}
}

func (app *AnconAppChain) Info(req abcitypes.RequestInfo) abcitypes.ResponseInfo {
	return abcitypes.ResponseInfo{}
}

func (app *AnconAppChain) isValid(data []byte) uint32 {
	return 0
}

func (app *AnconAppChain) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	code := app.isValid(req.Tx)
	if code != 0 {
		return abcitypes.ResponseDeliverTx{Code: code}
	}

	node := basicnode.NewBytes(req.Tx)
	issuer, _ := node.LookupByString("issuer")
	cid, _ := node.LookupByString("contentHash")
	sig, _ := node.LookupByString("signature")
	path, _ := node.LookupByString("path")

	events := []abcitypes.Event{
		{
			Type: "dagblock",
			Attributes: []abcitypes.EventAttribute{
				{Key: []byte("issuer"), Value: []byte(must.String(issuer)), Index: true},
				{Key: []byte("contentHash"), Value: []byte(must.String(cid)), Index: true},
				{Key: []byte("signature"), Value: []byte(must.String(sig)), Index: true},
				{Key: []byte("path"), Value: []byte(must.String(path)), Index: true},
			},
		},
	}
	return abcitypes.ResponseDeliverTx{Code: abcitypes.CodeTypeOK, Events: events}
}

func (app *AnconAppChain) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	// Validate block exists and is signed
	return abcitypes.ResponseCheckTx{Code: 0}
}

func (app *AnconAppChain) Commit() abcitypes.ResponseCommit {
	res, _ := app.storage.Commit()

	return abcitypes.ResponseCommit{Data: res.RootHash, RetainHeight: res.Version}
}

func (app *AnconAppChain) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {

	return abcitypes.ResponseQuery{Code: 0}
}

func (AnconAppChain) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
	return abcitypes.ResponseInitChain{}
}

func (app *AnconAppChain) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	// no op
	return abcitypes.ResponseBeginBlock{}
}

func (app *AnconAppChain) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	return abcitypes.ResponseEndBlock{}
}

func (app *AnconAppChain) ListSnapshots(abcitypes.RequestListSnapshots) abcitypes.ResponseListSnapshots {
	return abcitypes.ResponseListSnapshots{}
}

func (AnconAppChain) OfferSnapshot(abcitypes.RequestOfferSnapshot) abcitypes.ResponseOfferSnapshot {
	return abcitypes.ResponseOfferSnapshot{}
}

func (AnconAppChain) LoadSnapshotChunk(abcitypes.RequestLoadSnapshotChunk) abcitypes.ResponseLoadSnapshotChunk {
	return abcitypes.ResponseLoadSnapshotChunk{}
}

func (AnconAppChain) ApplySnapshotChunk(abcitypes.RequestApplySnapshotChunk) abcitypes.ResponseApplySnapshotChunk {
	return abcitypes.ResponseApplySnapshotChunk{}
}
