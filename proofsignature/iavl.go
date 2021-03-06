package proofsignature

import (
	"encoding/json"
	"fmt"
	"sync"

	ibc "github.com/cosmos/ibc-go/v2/modules/core/23-commitment/types"

	"github.com/anconprotocol/sdk"
	ics23 "github.com/confio/ics23/go"
	"github.com/cosmos/iavl"
	pb "github.com/cosmos/iavl/proto"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/ipfs/go-graphsync"
	"github.com/pkg/errors"
	dbm "github.com/tendermint/tm-db"
)

type IavlProofAPI struct {
	Namespace string
	Version   string
	Service   *IavlProofService
	Public    bool
}

type IavlProofService struct {
	rwLock      sync.RWMutex
	tree        *iavl.MutableTree
	dagStore    sdk.Storage
	dagExchange graphsync.GraphExchange
}

func NewIavlAPI(dagStore sdk.Storage, dagExchange graphsync.GraphExchange, db dbm.DB, cacheSize, version int64) (*IavlProofAPI, error) {
	tree, err := iavl.NewMutableTree(db, int(cacheSize))
	if err != nil {
		return nil, errors.Wrap(err, "unable to create iavl tree")
	}

	if _, err := tree.LoadVersion(version); err != nil {
		return nil, errors.Wrapf(err, "unable to load version %d", version)
	}

	return &IavlProofAPI{
		Namespace: "proofs",
		Version:   "1.0",
		Service: &IavlProofService{
			rwLock:      sync.RWMutex{},
			tree:        tree,
			dagStore:    dagStore,
			dagExchange: dagExchange,
		},
		Public: false,
	}, nil

	// return &DurinAPI{
	// 	Namespace: "durin",
	// 	Version:   "1.0",
	// 	Service: &DurinService{
	// 		Adapter:   &evm,
	// 		GqlClient: gqlClient,
	// 	},
	// 	Public: true,
	// }
}

// func GetArguments(req) (map[string]interface{}, error) {
// 	var values map[string]interface{}
// 	dec, err := hexutil.Decode(req.String())
// 	if err != nil {
// 		return nil, err
// 	}
// 	err = json.Unmarshal(dec, values)
// 	return values, err
// }

// func ToHex(v interface{}) (hexutil.Bytes, error) {
// 	jsonval, err := json.Marshal(v)
// 	if err != nil {
// 		return (hexutil.Encode([]byte(fmt.Errorf("reverted, json marshal").Error()))), err
// 	}
// 	valenc := hexutil.Encode(jsonval)
// 	return (valenc), err
// }

func (s *IavlProofService) GetCurrentVersion() ([]byte, int8) {

	s.rwLock.RLock()
	defer s.rwLock.RUnlock()

	return s.tree.Hash(), s.tree.Height()
}

// HasVersioned returns a result containing a boolean on whether or not the IAVL tree
// has a given key at a specific tree version.
func (s *IavlProofService) HasVersioned(version int64) (bool, error) {

	s.rwLock.RLock()
	defer s.rwLock.RUnlock()

	if !s.tree.VersionExists(version) {
		return false, iavl.ErrVersionDoesNotExist
	}

	_, err := s.tree.GetImmutable(version)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Has returns a result containing a boolean on whether or not the IAVL tree
// has a given key in the current version
func (s *IavlProofService) Has(key []byte) (bool, error) {

	s.rwLock.RLock()
	defer s.rwLock.RUnlock()

	return s.tree.Has(key), nil
}

// Get returns a result containing the index and value for a given
// key based on the current state (version) of the tree.
// If the key does not exist, Get returns the index of the next value.
func (s *IavlProofService) Get(key []byte) (json.RawMessage, error) {

	s.rwLock.RLock()
	defer s.rwLock.RUnlock()

	res := make(map[string]interface{})
	res["index"], res["value"] = s.tree.Get(key)
	if res["index"] == nil {
		e := fmt.Errorf("The index requested does not exist")
		return nil, e
	}

	hexres, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}

	return hexres, nil
}

// GetByIndex returns a result containing the key and value for a given
// index based on the current state (version) of the tree.
func (s *IavlProofService) GetByIndex(index int64) (json.RawMessage, error) {

	s.rwLock.RLock()
	defer s.rwLock.RUnlock()

	res := make(map[string]interface{})

	res["key"], res["value"] = s.tree.GetByIndex(index)
	if res["key"] == nil {
		e := fmt.Errorf("The key requested does not exist")
		return nil, e
	}

	hexres, err := json.Marshal(res)

	if err != nil {
		return nil, err
	}

	return hexres, nil
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
func (s *IavlProofService) GetWithProof(key []byte) (json.RawMessage, error) {

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
func (s *IavlProofService) GetCommitmentProof(key []byte, version int64) (json.RawMessage, error) {

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

// GetVersioned returns a result containing the IAVL tree version and value
// for a given key at a specific tree version.
func (s *IavlProofService) GetVersioned(version int64, key []byte) (json.RawMessage, error) {

	s.rwLock.RLock()
	defer s.rwLock.RUnlock()

	if !s.tree.VersionExists(version) {
		return nil, iavl.ErrVersionDoesNotExist
	}

	_, err := s.tree.GetImmutable(version)
	if err != nil {
		return nil, err
	}

	res := make(map[string]interface{})
	res["index"], res["value"] = s.tree.Get(key)
	res["version"] = version

	hexres, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}

	return hexres, nil
}

// GetVersionedWithProof returns a result containing the IAVL tree version and
// value for a given key at a specific tree version including a verifiable Merkle
// proof.
// func (s *IavlProofService) GetVersionedWithProof(req *pb.GetVersionedRequest) (*pb.GetWithProofResponse, error) {

// 	s.rwLock.RLock()
// 	defer s.rwLock.RUnlock()

// 	value, proof, err := s.tree.GetVersionedWithProof(req.Key, req.Version)
// 	if err != nil {
// 		return nil, err
// 	}

// 	if value == nil {
// 		s := status.New(codes.NotFound, "the key requested does not exist")
// 		return nil, s.Err()
// 	}

// 	proofPb := proof.ToProto()

// 	return &pb.GetWithProofResponse{Value: value, Proof: proofPb}, nil
// }

// Set returns a result after inserting a key/value pair into the IAVL tree
// based on the current state (version) of the tree.
func (s *IavlProofService) Set(key []byte, value []byte) (json.RawMessage, error) {

	s.rwLock.Lock()
	defer s.rwLock.Unlock()

	if key == nil {
		return nil, errors.New("key cannot be nil")
	}

	if value == nil {
		return nil, errors.New("value cannot be nil")
	}

	res := make(map[string]interface{})
	res["updated"] = s.tree.Set(key, value)
	//TODO
	//emits a graphsync event kv commited
	//the message propagates through the graphsync network & gets stored
	//Get proof with graphsync, verify if the proof is replicated elsewhere
	//that proof wil be validated with
	//will be necessary to make 2 or 3 extension data & 2 agents

	hexres, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}

	return hexres, nil
}

// SaveVersion saves a new IAVL tree version to the DB based on the current
// state (version) of the tree. It returns a result containing the hash and
// new version number.
func (s *IavlProofService) SaveVersion(_ *empty.Empty) ([]byte, error) {

	s.rwLock.Lock()
	defer s.rwLock.Unlock()

	root, version, err := s.tree.SaveVersion()
	if err != nil {
		return nil, err
	}
	res := &pb.SaveVersionResponse{RootHash: root, Version: version}
	resJson, err := json.Marshal(res)

	return resJson, nil
}

// DeleteVersion deletes an IAVL tree version from the DB. The version can then
// no longer be accessed. It returns a result containing the version and root
// hash of the versioned tree that was deleted.
func (s *IavlProofService) DeleteVersion(version int64) (json.RawMessage, error) {

	s.rwLock.Lock()
	defer s.rwLock.Unlock()

	iTree, err := s.tree.GetImmutable(version)
	if err != nil {
		return nil, err
	}

	if err := s.tree.DeleteVersion(version); err != nil {
		return nil, err
	}

	res := make(map[string]interface{})

	res["hash"] = iTree.Hash()
	res["version"] = version

	hexres, err := json.Marshal(res)

	return hexres, nil
}

// Version returns the IAVL tree version based on the current state.
// func (s *IavlProofService) Version(_ *empty.Empty) (*pb.VersionResponse, error) {

// 	s.rwLock.RLock()
// 	defer s.rwLock.RUnlock()

// 	return &pb.VersionResponse{Version: s.tree.Version()}, nil
// }

// Hash returns the IAVL tree root hash based on the current state.
func (s *IavlProofService) Hash(_ *empty.Empty) (json.RawMessage, error) {

	res := make(map[string]interface{})
	res["hash"] = s.tree.Hash()
	res["version"] = s.tree.Version()

	hexres, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}

	return hexres, nil
}

// VersionExists returns a result containing a boolean on whether or not a given
// version exists in the IAVL tree.
// func (s *IavlProofService) VersionExists(req *pb.VersionExistsRequest) (*pb.VersionExistsResponse, error) {

// 	s.rwLock.RLock()
// 	defer s.rwLock.RUnlock()

// 	return &pb.VersionExistsResponse{Result: s.tree.VersionExists(req.Version)}, nil
// }

// Verify verifies an IAVL range proof returning an error if the proof is invalid.
// func (*IavlProofService) Verify(ctx context.Context, req *pb.VerifyRequest) (*empty.Empty, error) {

// 	proof, err := iavl.RangeProofFromProto(req.Proof)

// 	if err != nil {
// 		return nil, err
// 	}

// 	if err := proof.Verify(req.RootHash); err != nil {
// 		return nil, err
// 	}

// 	return &empty.Empty{}, nil
// }

// VerifyItem verifies if a given key/value pair in an IAVL range proof returning
// an error if the proof or key is invalid.
// func (*IavlProofService) VerifyItem(ctx context.Context, req *pb.VerifyItemRequest) (*empty.Empty, error) {

// 	proof, err := iavl.RangeProofFromProto(req.Proof)

// 	if err != nil {
// 		return nil, err
// 	}

// 	if err := proof.Verify(req.RootHash); err != nil {
// 		return nil, err
// 	}

// 	if err := proof.VerifyItem(req.Key, req.Value); err != nil {
// 		return nil, err
// 	}

// 	return &empty.Empty{}, nil
// }

// VerifyAbsence verifies the absence of a given key in an IAVL range proof
// returning an error if the proof or key is invalid.
// func (*IavlProofService) VerifyAbsence(ctx context.Context, req *pb.VerifyAbsenceRequest) (*empty.Empty, error) {

// 	proof, err := iavl.RangeProofFromProto(req.Proof)

// 	if err != nil {
// 		return nil, err
// 	}

// 	if err := proof.Verify(req.RootHash); err != nil {
// 		return nil, err
// 	}

// 	if err := proof.VerifyAbsence(req.Key); err != nil {
// 		return nil, err
// 	}

// 	return &empty.Empty{}, nil
// }

// Rollback resets the working tree to the latest saved version, discarding
// any unsaved modifications.
// func (s *IavlProofService) rollback(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {

// 	s.rwLock.Lock()
// 	defer s.rwLock.Unlock()

// 	s.tree.Rollback()
// 	return &empty.Empty{}, nil
// }

// func (s *IavlProofService) GetAvailableVersions(ctx context.Context, req *empty.Empty) (*pb.GetAvailableVersionsResponse, error) {

// 	s.rwLock.RLock()
// 	defer s.rwLock.RUnlock()

// 	versionsInts := s.tree.AvailableVersions()

// 	versions := make([]int64, len(versionsInts))

// 	for i, version := range versionsInts {
// 		versions[i] = int64(version)
// 	}

// 	return &pb.GetAvailableVersionsResponse{Versions: versions}, nil
// }

// func (s *IavlProofService) load(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {

// 	s.rwLock.Lock()
// 	defer s.rwLock.Unlock()

// 	_, err := s.tree.Load()

// 	return &empty.Empty{}, err

// }

// func (s *IavlProofService) loadVersion(ctx context.Context, req *pb.LoadVersionRequest) (*empty.Empty, error) {

// 	s.rwLock.Lock()
// 	defer s.rwLock.Unlock()

// 	_, err := s.tree.LoadVersion(req.Version)

// 	return &empty.Empty{}, err

// }

// func (s *IavlProofService) loadVersionForOverwriting(ctx context.Context, req *pb.LoadVersionForOverwritingRequest) (*empty.Empty, error) {

// 	s.rwLock.Lock()
// 	defer s.rwLock.Unlock()

// 	_, err := s.tree.LoadVersionForOverwriting(req.Version)

// 	return &empty.Empty{}, err

// }
