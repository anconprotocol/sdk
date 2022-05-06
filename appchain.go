package sdk

import (
	"encoding/json"

	abcitypes "github.com/tendermint/tendermint/abci/types"
)

type AnconAppChain struct {
	storage *Storage
}

// var _ abcitypes.Application = (*AnconAppChain)(nil)

func NewAnconAppChain(storage *Storage) *AnconAppChain {
	return &AnconAppChain{
		storage: storage,
	}
}

func (app *CosmosAnconAppChain) SetOption(req abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
	return abcitypes.ResponseSetOption{}
}

func (app *CosmosAnconAppChain) Info(req abcitypes.RequestInfo) abcitypes.ResponseInfo {
	return abcitypes.ResponseInfo{}
}

func (app *CosmosAnconAppChain) isValid(data []byte) uint32 {
	return 0
}

func (app *CosmosAnconAppChain) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	code := app.isValid(req.Tx)
	if code != 0 {
		return abcitypes.ResponseDeliverTx{Code: code}
	}

	// node := basicnode.NewBytes(req.Tx)
	// issuer, _ := node.LookupByString("issuer")
	// cid, _ := node.LookupByString("contentHash")
	// sig, _ := node.LookupByString("signature")
	// path, _ := node.LookupByString("path")

	// events := []abcitypes.Event{
	// 	{
	// 		Type: "dagblock",
	// 		Attributes: []abcitypes.EventAttribute{
	// 			{Key: []byte("issuer"), Value: []byte(must.String(issuer)), Index: true},
	// 			{Key: []byte("contentHash"), Value: []byte(must.String(cid)), Index: true},
	// 			{Key: []byte("signature"), Value: []byte(must.String(sig)), Index: true},
	// 			{Key: []byte("path"), Value: []byte(must.String(path)), Index: true},
	// 		},
	// 	},
	// }
	return abcitypes.ResponseDeliverTx{Code: abcitypes.CodeTypeOK}
}

func (app *CosmosAnconAppChain) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	// Validate block exists and is signed
	return abcitypes.ResponseCheckTx{Code: 0}
}

func (app *CosmosAnconAppChain) Commit() abcitypes.ResponseCommit {
	res, _ := app.AnconAppChain.storage.Commit()

	return abcitypes.ResponseCommit{Data: res.RootHash, RetainHeight: res.Version}
}

func (app *CosmosAnconAppChain) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	resp := abcitypes.ResponseQuery{Key: req.Data}

	var err error
	var item json.RawMessage
	if req.Height == 0 {
		item, err = app.AnconAppChain.storage.GetWithProof(resp.Key)
		resp.Value = item

	} else if req.Height > 0 {
		item, err = app.AnconAppChain.storage.GetCommitmentProof(req.Data, req.Height)
		resp.Value = item
	}
	if err != nil {
		resp.Log = "key does not exist"
	} else {
		resp.Log = "exists"
	}
	return resp
}

func (CosmosAnconAppChain) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
	return abcitypes.ResponseInitChain{}
}

func (app *CosmosAnconAppChain) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	// no op
	return abcitypes.ResponseBeginBlock{}
}

func (app *CosmosAnconAppChain) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	return abcitypes.ResponseEndBlock{}
}

func (app *CosmosAnconAppChain) ListSnapshots(abcitypes.RequestListSnapshots) abcitypes.ResponseListSnapshots {
	return abcitypes.ResponseListSnapshots{}
}

func (CosmosAnconAppChain) OfferSnapshot(abcitypes.RequestOfferSnapshot) abcitypes.ResponseOfferSnapshot {
	return abcitypes.ResponseOfferSnapshot{}
}

func (CosmosAnconAppChain) LoadSnapshotChunk(abcitypes.RequestLoadSnapshotChunk) abcitypes.ResponseLoadSnapshotChunk {
	return abcitypes.ResponseLoadSnapshotChunk{}
}

func (CosmosAnconAppChain) ApplySnapshotChunk(abcitypes.RequestApplySnapshotChunk) abcitypes.ResponseApplySnapshotChunk {
	return abcitypes.ResponseApplySnapshotChunk{}
}
