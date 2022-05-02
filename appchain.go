package sdk

import (
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

	// TODO: Validate CID
	return abcitypes.ResponseDeliverTx{Code: 0}
}

func (app *AnconAppChain) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	// Validate block exists and is signed
	return abcitypes.ResponseCheckTx{Code: 0}
}

func (app *AnconAppChain) Commit() abcitypes.ResponseCommit {
	app.storage.Commit()
	return abcitypes.ResponseCommit{Data: []byte{}}
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
