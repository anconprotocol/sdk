package sdk

import (
	"fmt"
	"io"

	cosmossdk "github.com/cosmos/cosmos-sdk/types"
	abci "github.com/tendermint/tendermint/abci/types"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"

	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/snapshots"
	"github.com/cosmos/cosmos-sdk/store"
)

// CosmosAnconAppChain reflects the ABCI application implementation.
type CosmosAnconAppChain struct { // nolint: maligned
	// initialized on creation
	logger            log.Logger
	name              string                     // application name from abci.Info
	db                dbm.DB                     // common DB backend
	cms               cosmossdk.CommitMultiStore // Main (uncached) state
	storeLoader       StoreLoader                // function to handle store loading, may be overridden with SetStoreLoader()
	router            cosmossdk.Router           // handle any kind of message
	queryRouter       cosmossdk.QueryRouter      // router for redirecting query calls
	grpcQueryRouter   *baseapp.GRPCQueryRouter   // router for redirecting gRPC query calls
	msgServiceRouter  *baseapp.MsgServiceRouter  // router for redirecting Msg service messages
	interfaceRegistry types.InterfaceRegistry
	txDecoder         cosmossdk.TxDecoder // unmarshal []byte into cosmossdk.Tx

	anteHandler    cosmossdk.AnteHandler  // ante handler for fee and auth
	initChainer    cosmossdk.InitChainer  // initialize state with validators and state blob
	beginBlocker   cosmossdk.BeginBlocker // logic to run before any txs
	endBlocker     cosmossdk.EndBlocker   // logic to run after all txs, and to determine valset changes
	addrPeerFilter cosmossdk.PeerFilter   // filter peers by address and port
	idPeerFilter   cosmossdk.PeerFilter   // filter peers by node ID
	fauxMerkleMode bool                   // if true, IAVL MountStores uses MountStoresDB for simulation speed.

	// manages snapshots, i.e. dumps of app state at certain intervals
	snapshotManager    *snapshots.Manager
	snapshotInterval   uint64 // block interval between state sync snapshots
	snapshotKeepRecent uint32 // recent state sync snapshots to keep

	// volatile states:
	//
	// checkState is set on InitChain and reset on Commit
	// deliverState is set on InitChain and BeginBlock and set to nil on Commit
	checkState   *state // for CheckTx
	deliverState *state // for DeliverTx

	// an inter-block write-through cache provided to the context during deliverState
	interBlockCache cosmossdk.MultiStorePersistentCache

	// absent validators from begin block
	voteInfos []abci.VoteInfo

	// paramStore is used to query for ABCI consensus parameters from an
	// application parameter store.
	paramStore baseapp.ParamStore

	// The minimum gas prices a validator is willing to accept for processing a
	// transaction. This is mainly used for DoS and spam prevention.
	minGasPrices cosmossdk.DecCoins

	// initialHeight is the initial height at which we start the baseapp
	initialHeight int64

	// flag for sealing options and parameters to a BaseApp
	sealed bool

	// block height at which to halt the chain and gracefully shutdown
	haltHeight uint64

	// minimum block time (in Unix seconds) at which to halt the chain and gracefully shutdown
	haltTime uint64

	// minRetainBlocks defines the minimum block height offset from the current
	// block being committed, such that all blocks past this offset are pruned
	// from Tendermint. It is used as part of the process of determining the
	// ResponseCommit.RetainHeight value during ABCI Commit. A value of 0 indicates
	// that no blocks should be pruned.
	//
	// Note: Tendermint block pruning is dependant on this parameter in conunction
	// with the unbonding (safety threshold) period, state pruning and state sync
	// snapshot parameters to determine the correct minimum value of
	// ResponseCommit.RetainHeight.
	minRetainBlocks uint64

	// application's version string
	version string

	// application's protocol version that increments on every upgrade
	// if BaseApp is passed to the upgrade keeper's NewKeeper method.
	appVersion uint64

	// recovery handler for app.runTx method
	// runTxRecoveryMiddleware recoveryMiddleware

	// trace set will return full stack traces for errors in ABCI Log field
	trace bool

	storage *Storage
	// indexEvents defines the set of events in the form {eventType}.{attributeKey},
	// which informs Tendermint what to index. If empty, all events will be indexed.
	indexEvents map[string]struct{}
}

// File for storing in-package CosmosAnconAppChain optional functions,
// for options that need access to non-exported fields of the CosmosAnconAppChain

// SetPruning sets a pruning option on the multistore associated with the app
func SetPruning(opts cosmossdk.PruningOptions) func(*CosmosAnconAppChain) {
	return func(bap *CosmosAnconAppChain) { bap.cms.SetPruning(opts) }
}

// SetMinGasPrices returns an option that sets the minimum gas prices on the app.
func SetMinGasPrices(gasPricesStr string) func(*CosmosAnconAppChain) {
	gasPrices, err := cosmossdk.ParseDecCoins(gasPricesStr)
	if err != nil {
		panic(fmt.Sprintf("invalid minimum gas prices: %v", err))
	}

	return func(bap *CosmosAnconAppChain) { bap.setMinGasPrices(gasPrices) }
}

// SetHaltHeight returns a CosmosAnconAppChain option function that sets the halt block height.
func SetHaltHeight(blockHeight uint64) func(*CosmosAnconAppChain) {
	return func(bap *CosmosAnconAppChain) { bap.setHaltHeight(blockHeight) }
}

// SetHaltTime returns a CosmosAnconAppChain option function that sets the halt block time.
func SetHaltTime(haltTime uint64) func(*CosmosAnconAppChain) {
	return func(bap *CosmosAnconAppChain) { bap.setHaltTime(haltTime) }
}

// SetMinRetainBlocks returns a CosmosAnconAppChain option function that sets the minimum
// block retention height value when determining which heights to prune during
// ABCI Commit.
func SetMinRetainBlocks(minRetainBlocks uint64) func(*CosmosAnconAppChain) {
	return func(bapp *CosmosAnconAppChain) { bapp.setMinRetainBlocks(minRetainBlocks) }
}

// SetTrace will turn on or off trace flag
func SetTrace(trace bool) func(*CosmosAnconAppChain) {
	return func(app *CosmosAnconAppChain) { app.setTrace(trace) }
}

// SetIndexEvents provides a CosmosAnconAppChain option function that sets the events to index.
func SetIndexEvents(ie []string) func(*CosmosAnconAppChain) {
	return func(app *CosmosAnconAppChain) { app.setIndexEvents(ie) }
}

// SetInterBlockCache provides a CosmosAnconAppChain option function that sets the
// inter-block cache.
func SetInterBlockCache(cache cosmossdk.MultiStorePersistentCache) func(*CosmosAnconAppChain) {
	return func(app *CosmosAnconAppChain) { app.setInterBlockCache(cache) }
}

// SetSnapshotInterval sets the snapshot interval.
func SetSnapshotInterval(interval uint64) func(*CosmosAnconAppChain) {
	return func(app *CosmosAnconAppChain) { app.SetSnapshotInterval(interval) }
}

// SetSnapshotKeepRecent sets the recent snapshots to keep.
func SetSnapshotKeepRecent(keepRecent uint32) func(*CosmosAnconAppChain) {
	return func(app *CosmosAnconAppChain) { app.SetSnapshotKeepRecent(keepRecent) }
}

// SetSnapshotStore sets the snapshot store.
func SetSnapshotStore(snapshotStore *snapshots.Store) func(*CosmosAnconAppChain) {
	return func(app *CosmosAnconAppChain) { app.SetSnapshotStore(snapshotStore) }
}

func (app *CosmosAnconAppChain) SetName(name string) {
	if app.sealed {
		panic("SetName() on sealed CosmosAnconAppChain")
	}

	app.name = name
}

// SetParamStore sets a parameter store on the CosmosAnconAppChain.
func (app *CosmosAnconAppChain) SetParamStore(ps baseapp.ParamStore) {
	if app.sealed {
		panic("SetParamStore() on sealed CosmosAnconAppChain")
	}

	app.paramStore = ps
}

// SetVersion sets the application's version string.
func (app *CosmosAnconAppChain) SetVersion(v string) {
	if app.sealed {
		panic("SetVersion() on sealed CosmosAnconAppChain")
	}
	app.version = v
}

// SetProtocolVersion sets the application's protocol version
func (app *CosmosAnconAppChain) SetProtocolVersion(v uint64) {
	app.appVersion = v
}

func (app *CosmosAnconAppChain) SetDB(db dbm.DB) {
	if app.sealed {
		panic("SetDB() on sealed CosmosAnconAppChain")
	}

	app.db = db
}

func (app *CosmosAnconAppChain) SetCMS(cms store.CommitMultiStore) {
	if app.sealed {
		panic("SetEndBlocker() on sealed CosmosAnconAppChain")
	}

	app.cms = cms
}

func (app *CosmosAnconAppChain) SetInitChainer(initChainer cosmossdk.InitChainer) {
	if app.sealed {
		panic("SetInitChainer() on sealed CosmosAnconAppChain")
	}

	app.initChainer = initChainer
}

func (app *CosmosAnconAppChain) SetBeginBlocker(beginBlocker cosmossdk.BeginBlocker) {
	if app.sealed {
		panic("SetBeginBlocker() on sealed CosmosAnconAppChain")
	}

	app.beginBlocker = beginBlocker
}

func (app *CosmosAnconAppChain) SetEndBlocker(endBlocker cosmossdk.EndBlocker) {
	if app.sealed {
		panic("SetEndBlocker() on sealed CosmosAnconAppChain")
	}

	app.endBlocker = endBlocker
}

func (app *CosmosAnconAppChain) SetAnteHandler(ah cosmossdk.AnteHandler) {
	if app.sealed {
		panic("SetAnteHandler() on sealed CosmosAnconAppChain")
	}

	app.anteHandler = ah
}

func (app *CosmosAnconAppChain) SetAddrPeerFilter(pf cosmossdk.PeerFilter) {
	if app.sealed {
		panic("SetAddrPeerFilter() on sealed CosmosAnconAppChain")
	}

	app.addrPeerFilter = pf
}

func (app *CosmosAnconAppChain) SetIDPeerFilter(pf cosmossdk.PeerFilter) {
	if app.sealed {
		panic("SetIDPeerFilter() on sealed CosmosAnconAppChain")
	}

	app.idPeerFilter = pf
}

func (app *CosmosAnconAppChain) SetFauxMerkleMode() {
	if app.sealed {
		panic("SetFauxMerkleMode() on sealed CosmosAnconAppChain")
	}

	app.fauxMerkleMode = true
}

// SetCommitMultiStoreTracer sets the store tracer on the CosmosAnconAppChain's underlying
// CommitMultiStore.
func (app *CosmosAnconAppChain) SetCommitMultiStoreTracer(w io.Writer) {
	app.cms.SetTracer(w)
}

// SetStoreLoader allows us to customize the rootMultiStore initialization.
func (app *CosmosAnconAppChain) SetStoreLoader(loader StoreLoader) {
	if app.sealed {
		panic("SetStoreLoader() on sealed CosmosAnconAppChain")
	}

	app.storeLoader = loader
}

// SetRouter allows us to customize the router.
func (app *CosmosAnconAppChain) SetRouter(router cosmossdk.Router) {
	if app.sealed {
		panic("SetRouter() on sealed CosmosAnconAppChain")
	}
	app.router = router
}

// SetSnapshotStore sets the snapshot store.
func (app *CosmosAnconAppChain) SetSnapshotStore(snapshotStore *snapshots.Store) {
	if app.sealed {
		panic("SetSnapshotStore() on sealed CosmosAnconAppChain")
	}
	if snapshotStore == nil {
		app.snapshotManager = nil
		return
	}
	app.snapshotManager = snapshots.NewManager(snapshotStore, app.cms)
}

// SetSnapshotInterval sets the snapshot interval.
func (app *CosmosAnconAppChain) SetSnapshotInterval(snapshotInterval uint64) {
	if app.sealed {
		panic("SetSnapshotInterval() on sealed CosmosAnconAppChain")
	}
	app.snapshotInterval = snapshotInterval
}

// SetSnapshotKeepRecent sets the number of recent snapshots to keep.
func (app *CosmosAnconAppChain) SetSnapshotKeepRecent(snapshotKeepRecent uint32) {
	if app.sealed {
		panic("SetSnapshotKeepRecent() on sealed CosmosAnconAppChain")
	}
	app.snapshotKeepRecent = snapshotKeepRecent
}

// SetInterfaceRegistry sets the InterfaceRegistry.
func (app *CosmosAnconAppChain) SetInterfaceRegistry(registry types.InterfaceRegistry) {
	app.interfaceRegistry = registry
	app.grpcQueryRouter.SetInterfaceRegistry(registry)
	app.msgServiceRouter.SetInterfaceRegistry(registry)
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
