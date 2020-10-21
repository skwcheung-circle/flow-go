// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package matching

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	realproto "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// 1. Matching engine should validate the incoming receipt (aka ExecutionReceipt):
//     1. it should stores it to the mempool if valid
//     2. it should ignore it when:
//         1. the origin is invalid
//         2. the role is invalid
//         3. the result (a receipt has one result, multiple receipts might have the same result) has been sealed already
//         4. the receipt has been received before
//         5. the result has been received before
// 2. Matching engine should validate the incoming approval (aka ResultApproval):
//     1. it should store it to the mempool if valid
//     2. it should ignore it when:
//         1. the origin is invalid
//         2. the role is invalid
//         3. the result has been sealed already
// 3. Matching engine should be able to find matched results:
//     1. It should find no matched result if there is no result and no approval
//     2. it should find 1 matched result if we received a receipt, and the block has no payload (impossible now, system every block will have at least one chunk to verify)
//     3. It should find no matched result if there is only result, but no approval (skip for now, because we seal results without approvals)
// 4. Matching engine should be able to seal a matched result:
//     1. It should not seal a matched result if:
//         1. the block is missing (consensus hasn’t received this executed block yet)
//         2. the approvals for a certain chunk are insufficient (skip for now, because we seal results without approvals)
//         3. there is some chunk didn’t receive enough approvals
//         4. the previous result is not known
//         5. the previous result references the wrong block
//     2. It should seal a matched result if the approvals are sufficient
// 5. Matching engine should request results from execution nodes:
//     1. If there are unsealed and finalized blocks, it should request the execution receipts from the execution nodes.
func TestMatchingEngine(t *testing.T) {
	suite.Run(t, new(MatchingSuite))
}

type MatchingSuite struct {
	suite.Suite

	conID flow.Identifier
	exeID flow.Identifier
	verID flow.Identifier

	identities map[flow.Identifier]*flow.Identity

	approvers flow.IdentityList

	state *protocol.State

	lastSealed     *flow.Header
	sealedSnapshot *protocol.Snapshot
	finalSnapshot  *protocol.Snapshot

	results map[flow.Identifier]*flow.ExecutionResult
	blocks  map[flow.Identifier]*flow.Block

	resultsDB *storage.ExecutionResults
	headersDB *storage.Headers
	indexDB   *storage.Index

	pendingResults map[flow.Identifier]*flow.IncorporatedResult
	pendingSeals   map[flow.Identifier]*flow.IncorporatedResultSeal

	resultsPL   *mempool.IncorporatedResults
	receiptsPL  *mempool.Receipts
	approvalsPL *mempool.Approvals
	sealsPL     *mempool.IncorporatedResultSeals

	requester *module.Requester

	assigner *module.ChunkAssigner

	matching *Engine
}

func (ms *MatchingSuite) SetupTest() {

	unit := engine.NewUnit()
	log := zerolog.New(os.Stderr)
	metrics := metrics.NewNoopCollector()

	con := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exe := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	ver := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	ms.conID = con.NodeID
	ms.exeID = exe.NodeID
	ms.verID = ver.NodeID

	ms.identities = make(map[flow.Identifier]*flow.Identity)
	ms.identities[ms.conID] = con
	ms.identities[ms.exeID] = exe
	ms.identities[ms.verID] = ver

	ms.approvers = unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleVerification))

	ms.state = &protocol.State{}
	ms.state.On("Sealed").Return(
		func() realproto.Snapshot {
			return ms.sealedSnapshot
		},
		nil,
	)
	ms.state.On("Final").Return(
		func() realproto.Snapshot {
			return ms.finalSnapshot
		},
		nil,
	)
	ms.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) realproto.Snapshot {
			return ms.finalSnapshot
		},
		nil,
	)

	ms.finalSnapshot = &protocol.Snapshot{}
	ms.finalSnapshot.On("Identity", mock.Anything).Return(
		func(nodeID flow.Identifier) *flow.Identity {
			identity := ms.identities[nodeID]
			return identity
		},
		func(nodeID flow.Identifier) error {
			_, found := ms.identities[nodeID]
			if !found {
				return fmt.Errorf("could not get identity (%x)", nodeID)
			}
			return nil
		},
	)
	ms.finalSnapshot.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return ms.approvers
		},
		func(selector flow.IdentityFilter) error {
			return nil
		},
	)
	ms.finalSnapshot.On("Head").Return(
		func() *flow.Header {
			return &flow.Header{} // we don't care
		},
		nil,
	)

	ms.lastSealed = &flow.Header{}

	ms.sealedSnapshot = &protocol.Snapshot{}

	ms.results = make(map[flow.Identifier]*flow.ExecutionResult)
	ms.blocks = make(map[flow.Identifier]*flow.Block)

	ms.resultsDB = &storage.ExecutionResults{}
	ms.resultsDB.On("ByID", mock.Anything).Return(
		func(resultID flow.Identifier) *flow.ExecutionResult {
			return ms.results[resultID]
		},
		func(resultID flow.Identifier) error {
			_, found := ms.results[resultID]
			if !found {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	ms.headersDB = &storage.Headers{}
	ms.headersDB.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Header {
			block, found := ms.blocks[blockID]
			if !found {
				return nil
			}
			return block.Header
		},
		func(blockID flow.Identifier) error {
			_, found := ms.blocks[blockID]
			if !found {
				return storerr.ErrNotFound
			}
			return nil
		},
	)
	ms.headersDB.On("ByHeight", mock.Anything).Return(
		func(blockHeight uint64) *flow.Header {
			for _, b := range ms.blocks {
				if b.Header.Height == blockHeight {
					return b.Header
				}
			}
			return nil
		},
		func(blockHeight uint64) error {
			for _, b := range ms.blocks {
				if b.Header.Height == blockHeight {
					return nil
				}
			}
			return storerr.ErrNotFound
		},
	)
	ms.headersDB.On("GetLastSealed").Return(
		func() *flow.Header {
			return ms.lastSealed
		},
		func() error {
			return nil
		},
	)

	ms.indexDB = &storage.Index{}
	ms.indexDB.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Index {
			block, found := ms.blocks[blockID]
			if !found {
				return nil
			}
			if block.Payload == nil {
				return nil
			}
			return block.Payload.Index()
		},
		func(blockID flow.Identifier) error {
			_, found := ms.blocks[blockID]
			if !found {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	ms.pendingResults = make(map[flow.Identifier]*flow.IncorporatedResult)
	ms.pendingSeals = make(map[flow.Identifier]*flow.IncorporatedResultSeal)

	ms.resultsPL = &mempool.IncorporatedResults{}
	ms.resultsPL.On("Size").Return(uint(0)) // only for metrics
	ms.resultsPL.On("All").Return(
		func() []*flow.IncorporatedResult {
			results := make([]*flow.IncorporatedResult, 0, len(ms.pendingResults))
			for _, result := range ms.pendingResults {
				results = append(results, result)
			}
			return results
		},
	)

	ms.receiptsPL = &mempool.Receipts{}
	ms.receiptsPL.On("Size").Return(uint(0)) // only for metrics

	ms.approvalsPL = &mempool.Approvals{}
	ms.approvalsPL.On("Size").Return(uint(0)) // only for metrics

	ms.sealsPL = &mempool.IncorporatedResultSeals{}
	ms.sealsPL.On("Size").Return(uint(0)) // only for metrics
	ms.sealsPL.On("ByID", mock.Anything).Return(
		func(sealID flow.Identifier) *flow.IncorporatedResultSeal {
			return ms.pendingSeals[sealID]
		},
		func(sealID flow.Identifier) bool {
			_, found := ms.pendingSeals[sealID]
			return found
		},
	)

	ms.requester = new(module.Requester)
	ms.assigner = &module.ChunkAssigner{}

	ms.matching = &Engine{
		unit:                    unit,
		log:                     log,
		metrics:                 metrics,
		mempool:                 metrics,
		state:                   ms.state,
		requester:               ms.requester,
		resultsDB:               ms.resultsDB,
		headersDB:               ms.headersDB,
		indexDB:                 ms.indexDB,
		incorporatedResults:     ms.resultsPL,
		receipts:                ms.receiptsPL,
		approvals:               ms.approvalsPL,
		seals:                   ms.sealsPL,
		checkingSealing:         atomic.NewBool(false),
		requestReceiptThreshold: 10,
		maxUnsealedResults:      200,
		assigner:                ms.assigner,
		requireApprovals:        true,
	}
}

func (ms *MatchingSuite) TestOnReceiptInvalidOrigin() {

	// try to submit a receipt with a random origin ID
	originID := ms.exeID
	receipt := unittest.ExecutionReceiptFixture()

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().Error(err, "should reject receipt with mismatching origin and executor")

	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnReceiptUnknownBlock() {
	// try ot submit a receipt from a consensus node
	originID := ms.conID
	receipt := unittest.ExecutionReceiptFixture()
	receipt.ExecutorID = originID

	// onReceipt should not throw an error
	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().NoError(err, "should ignore receipt for unknown block")

	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnReceiptSealedResult() {

	// try to submit a receipt for a sealed result
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	originID := ms.exeID
	receipt := unittest.ReceiptForBlockFixture(&block)
	receipt.ExecutorID = originID

	// pretend that the receipt's block is already sealed
	ms.lastSealed = block.Header

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().NoError(err, "should ignore receipt for sealed result")

	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnReceiptInvalidRole() {

	// try ot submit a receipt from a consensus node
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	originID := ms.conID
	receipt := unittest.ReceiptForBlockFixture(&block)
	receipt.ExecutorID = originID

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().Error(err, "should reject receipt from wrong node role")

	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnReceiptUnstakedExecutor() {

	// try ot submit a receipt from an unstaked node
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	originID := ms.exeID
	receipt := unittest.ReceiptForBlockFixture(&block)
	receipt.ExecutorID = originID
	ms.identities[originID].Stake = 0

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().Error(err, "should reject receipt from unstaked node")

	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnReceiptPendingReceipt() {

	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	originID := ms.exeID
	receipt := unittest.ReceiptForBlockFixture(&block)
	receipt.ExecutorID = originID

	// check that we attempted to add the receipt to the mempool and return
	// false as if it was already in the mempool
	ms.receiptsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			added := args.Get(0).(*flow.ExecutionReceipt)
			ms.Assert().Equal(receipt, added)
		},
	).Return(false)

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().NoError(err, "should ignore already pending receipt")

	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnReceiptPendingResult() {

	// try to submit a receipt for a result that is already in the mempool
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	originID := ms.exeID
	receipt := unittest.ReceiptForBlockFixture(&block)
	receipt.ExecutorID = originID

	// pretend the receipt is added to the mempool
	ms.receiptsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			added := args.Get(0).(*flow.ExecutionReceipt)
			ms.Assert().Equal(receipt, added)
		},
	).Return(true)

	// pretend the result is already in the mempool
	ms.resultsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			incorporatedResult := args.Get(0).(*flow.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.Result, &receipt.ExecutionResult)
		},
	).Return(false, nil)

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().NoError(err, "should ignore receipt for already pending result")

	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnReceiptValid() {

	// try to submit a receipt that should be valid
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	originID := ms.exeID
	receipt := unittest.ReceiptForBlockFixture(&block)
	receipt.ExecutorID = originID

	ms.receiptsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			added := args.Get(0).(*flow.ExecutionReceipt)
			ms.Assert().Equal(receipt, added)
		},
	).Return(true)

	ms.resultsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			incorporatedResult := args.Get(0).(*flow.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.Result, &receipt.ExecutionResult)
		},
	).Return(true, nil)

	err := ms.matching.onReceipt(originID, receipt)
	ms.Require().NoError(err, "should add receipt and result to mempool if valid")

	ms.receiptsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnApprovalInvalidOrigin() {

	// try to submit an approval with a random origin ID
	originID := ms.verID
	approval := unittest.ResultApprovalFixture()

	err := ms.matching.onApproval(originID, approval)
	ms.Require().Error(err, "should reject approval with mismatching origin and executor")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnApprovalUnknownBlock() {
	// try to submit an approval for an unknown block
	originID := ms.verID
	approval := unittest.ResultApprovalFixture()
	approval.Body.ApproverID = originID

	// make sure the approval is added to the cache for future processing
	// check calls have the correct parameters
	ms.approvalsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			added := args.Get(0).(*flow.ResultApproval)
			ms.Assert().Equal(approval, added)
		},
	).Return(false, nil)

	// onApproval should not throw an error
	err := ms.matching.onApproval(originID, approval)
	ms.Require().NoError(err, "should ignore receipt for unknown block")

	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnApprovalSealedResult() {

	// try to submit an approval for a sealed result
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	originID := ms.verID
	approval := unittest.ResultApprovalFixture()
	approval.Body.BlockID = block.ID()
	approval.Body.ApproverID = originID

	// pretend that the block is already sealed
	ms.lastSealed = block.Header

	err := ms.matching.onApproval(originID, approval)
	ms.Require().NoError(err, "should ignore approval for sealed result")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnApprovalInvalidRole() {

	// try to submit an approval from a consensus node
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	originID := ms.conID
	approval := unittest.ResultApprovalFixture()
	approval.Body.BlockID = block.ID()
	approval.Body.ApproverID = originID

	err := ms.matching.onApproval(originID, approval)
	ms.Require().Error(err, "should reject approval from wrong approver role")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnApprovalInvalidStake() {

	// try to submit an approval from an unstaked approver
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	originID := ms.verID
	approval := unittest.ResultApprovalFixture()
	approval.Body.BlockID = block.ID()
	approval.Body.ApproverID = originID

	ms.identities[originID].Stake = 0

	err := ms.matching.onApproval(originID, approval)
	ms.Require().Error(err, "should reject approval from unstaked approver")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnApprovalPendingApproval() {

	// try to submit an approval that is already in the mempool
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	originID := ms.verID
	approval := unittest.ResultApprovalFixture()
	approval.Body.BlockID = block.ID()
	approval.Body.ApproverID = originID

	// check calls have the correct parameters
	ms.approvalsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			added := args.Get(0).(*flow.ResultApproval)
			ms.Assert().Equal(approval, added)
		},
	).Return(false, nil)

	err := ms.matching.onApproval(originID, approval)
	ms.Require().NoError(err, "should ignore approval if already pending")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestOnApprovalValid() {

	// try to submit an approval for a sealed result
	block := unittest.BlockFixture()
	ms.blocks[block.ID()] = &block

	originID := ms.verID
	approval := unittest.ResultApprovalFixture()
	approval.Body.BlockID = block.ID()
	approval.Body.ApproverID = originID

	// check calls have the correct parameters
	ms.approvalsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			added := args.Get(0).(*flow.ResultApproval)
			ms.Assert().Equal(approval, added)
		},
	).Return(true, nil)

	err := ms.matching.onApproval(originID, approval)
	ms.Require().NoError(err, "should add approval to mempool if valid")

	ms.approvalsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

func (ms *MatchingSuite) TestSealableResultsEmptyMempools() {

	// try to get matched results with nothing in memory pools
	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err, "should not error with empty mempools")
	ms.Assert().Empty(results, "should not have matched results with empty mempools")
}

func (ms *MatchingSuite) TestSealableResultsMissingBlock() {

	// try to seal a result for which we don't have the index payload
	incorporatedResult := unittest.IncorporatedResultFixture()

	ms.pendingResults[incorporatedResult.ID()] = incorporatedResult

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)

	ms.Assert().Empty(results, "should not select result with unknown block")
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Rem", 0)
}

func (ms *MatchingSuite) TestSealableResulstUnknownPrevious() {

	// try to seal a result with a missing previous result
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	incorporatedResult := unittest.IncorporatedResultForBlockFixture(&block)

	ms.pendingResults[incorporatedResult.ID()] = incorporatedResult

	// check that it is looking for the previous result, but return nil as if
	// not found
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(nil, nil)

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)

	ms.Assert().Empty(results, "should not select result with unsealed previous")
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Rem", 0)
}

// if the previous is not found in mempool, we should look for it in the
// resultsDB
func (ms *MatchingSuite) TestSealableResultsPreviousNotInMempool() {
	// try to seal a result with a persisted previous result
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	incorporatedResult := unittest.IncorporatedResultForBlockFixture(&block)
	previous := unittest.ExecutionResultFixture() // previous does not reference the same block as block parent
	incorporatedResult.Result.PreviousResultID = previous.ID()
	ms.results[previous.ID()] = previous

	// add incorporated result to mempool
	ms.pendingResults[incorporatedResult.Result.ID()] = incorporatedResult

	// check that it is looking for the previous result in the mempool and
	// return nil
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(nil, nil)

	// check that it is looking for previous in resultsDB, and return previous
	ms.resultsDB.On("ByID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(previous)

	// check that we are trying to remove the incorporated result from mempool
	ms.resultsPL.On("Rem", mock.Anything).Run(
		func(args mock.Arguments) {
			incResult := args.Get(0).(*flow.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.ID(), incResult.ID())
		},
	).Return(true)

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)

	ms.Assert().Empty(results, "should not select result with invalid subgraph")
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Rem", 1)
}

// let R1 be a result that references block A, and R2 be R1's parent result.
// Then R2 should reference A's parent.
func (ms *MatchingSuite) TestSealableResultsInvalidSubgraph() {
	// try to seal a result with a persisted previous result
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	incorporatedResult := unittest.IncorporatedResultForBlockFixture(&block)
	previous := unittest.ExecutionResultFixture() // previous does not reference the same block as block parent
	incorporatedResult.Result.PreviousResultID = previous.ID()

	// add incorporated result to mempool
	ms.pendingResults[incorporatedResult.Result.ID()] = incorporatedResult

	// check that it is looking for the previous result, and return previous
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(previous, nil)

	// check that we are trying to remove the incorporated result from mempool
	ms.resultsPL.On("Rem", mock.Anything).Run(
		func(args mock.Arguments) {
			incResult := args.Get(0).(*flow.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.ID(), incResult.ID())
		},
	).Return(true)

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)

	ms.Assert().Empty(results, "should not select result with invalid subgraph")
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Rem", 1)
}

func (ms *MatchingSuite) TestSealResultInvalidChunks() {

	// try to seal a result with a mismatching chunk count (one too many)
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	incorporatedResult := unittest.IncorporatedResultForBlockFixture(&block)
	previous := unittest.ExecutionResultFixture()
	previous.BlockID = block.Header.ParentID
	incorporatedResult.Result.PreviousResultID = previous.ID()

	// add an extra chunk
	chunk := unittest.ChunkFixture(block.ID())
	chunk.Index = uint64(len(block.Payload.Guarantees))
	incorporatedResult.Result.Chunks = append(incorporatedResult.Result.Chunks, chunk)

	// add incorporated result to mempool
	ms.pendingResults[incorporatedResult.Result.ID()] = incorporatedResult

	// check that it is looking for the previous result, and return previous
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(previous, nil)

	// check that we are trying to remove the incorporated result from mempool
	ms.resultsPL.On("Rem", mock.Anything).Run(
		func(args mock.Arguments) {
			incResult := args.Get(0).(*flow.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.ID(), incResult.ID())
		},
	).Return(true)

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)

	ms.Assert().Empty(results, "should not select result with invalid number of chunks")
	ms.resultsPL.AssertNumberOfCalls(ms.T(), "Rem", 1)
}

func (ms *MatchingSuite) TestSealableResultsNoPayload() {

	block := unittest.BlockFixture()
	block.Payload = nil // empty payload
	ms.blocks[block.Header.ID()] = &block
	incorporatedResult := unittest.IncorporatedResultForBlockFixture(&block)
	previous := unittest.ExecutionResultFixture()
	previous.BlockID = block.Header.ParentID
	incorporatedResult.Result.PreviousResultID = previous.ID()

	// add incorporated result to mempool
	ms.pendingResults[incorporatedResult.Result.ID()] = incorporatedResult

	// check that it is looking for the previous result, and return previous
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(previous, nil)

	// check that we are trying to remove the incorporated result from mempool
	ms.resultsPL.On("Rem", mock.Anything).Run(
		func(args mock.Arguments) {
			incResult := args.Get(0).(*flow.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.ID(), incResult.ID())
		},
	).Return(true)

	assignment := chunks.NewAssignment()
	ms.assigner.On("Assign", incorporatedResult.Result, incorporatedResult.IncorporatedBlockID).Return(assignment, nil)

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	if ms.Assert().Len(results, 1, "should select result for empty block") {
		sealable := results[0]
		ms.Assert().Equal(incorporatedResult, sealable)
	}
}

func (ms *MatchingSuite) TestSealableResultsInsufficientApprovals() {

	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	incorporatedResult := unittest.IncorporatedResultForBlockFixture(&block)
	previous := unittest.ExecutionResultFixture()
	previous.BlockID = block.Header.ParentID
	incorporatedResult.Result.PreviousResultID = previous.ID()

	// add incorporated result to mempool
	ms.pendingResults[incorporatedResult.Result.ID()] = incorporatedResult

	// check that it is looking for the previous result, and return previous
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(previous, nil)

	// check that we are trying to remove the incorporated result from mempool
	ms.resultsPL.On("Rem", mock.Anything).Run(
		func(args mock.Arguments) {
			incResult := args.Get(0).(*flow.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.ID(), incResult.ID())
		},
	).Return(true)

	assignment := chunks.NewAssignment()
	ms.assigner.On("Assign", incorporatedResult.Result, incorporatedResult.IncorporatedBlockID).Return(assignment, nil)

	// check that we are looking for chunk approvals, but return nil as if not
	// found
	ms.approvalsPL.On("ByChunk", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			resultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.ID(), resultID)
		},
	).Return(nil)

	// with requireApprovals = true ( default test case ), it should not collect
	// any results because we haven't added any approvals to the mempool
	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Empty(results, "should not select result with insufficient approvals")

	// with requireApprovals = false,  it should collect the result even if
	// there are no corresponding approvals
	ms.matching.requireApprovals = false
	results, err = ms.matching.sealableResults()
	ms.Require().NoError(err)
	if ms.Assert().Len(results, 1, "should select result when requireApprovals flag is false") {
		sealable := results[0]
		ms.Assert().Equal(incorporatedResult, sealable)
	}
}

func (ms *MatchingSuite) TestSealableResultsSufficientApprovals() {

	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	incorporatedResult := unittest.IncorporatedResultForBlockFixture(&block)
	previous := unittest.ExecutionResultFixture()
	previous.BlockID = block.Header.ParentID
	incorporatedResult.Result.PreviousResultID = previous.ID()

	// add incorporated result to mempool
	ms.pendingResults[incorporatedResult.Result.ID()] = incorporatedResult

	// check that it is looking for the previous result, and return previous
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(previous, nil)

	// check that we are trying to remove the incorporated result from mempool
	ms.resultsPL.On("Rem", mock.Anything).Run(
		func(args mock.Arguments) {
			incResult := args.Get(0).(*flow.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.ID(), incResult.ID())
		},
	).Return(true)

	// assign each chunk to each approver
	assignment := chunks.NewAssignment()
	for _, chunk := range incorporatedResult.Result.Chunks {
		assignment.Add(chunk, ms.approvers.NodeIDs())
	}
	ms.assigner.On("Assign", incorporatedResult.Result, incorporatedResult.IncorporatedBlockID).Return(assignment, nil)

	// not using mock for approvals pool because we need the internal indexing
	// logic
	realApprovalPool, err := stdmap.NewApprovals(1000)
	ms.Require().NoError(err)
	ms.matching.approvals = realApprovalPool

	// add enough approvals for each chunk
	for _, approver := range ms.approvers {
		for index := uint64(0); index < uint64(len(incorporatedResult.Result.Chunks)); index++ {
			approval := unittest.ResultApprovalFixture()
			approval.Body.BlockID = block.Header.ID()
			approval.Body.ExecutionResultID = incorporatedResult.Result.ID()
			approval.Body.ApproverID = approver.NodeID
			approval.Body.ChunkIndex = index
			_, err := ms.matching.approvals.Add(approval)
			ms.Require().NoError(err)
		}
	}

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	if ms.Assert().Len(results, 1, "should select result with sufficient approvals") {
		sealable := results[0]
		ms.Assert().Equal(incorporatedResult, sealable)
	}
}

func (ms *MatchingSuite) TestSealableResultsUnassignedVerifiers() {

	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	incorporatedResult := unittest.IncorporatedResultForBlockFixture(&block)
	previous := unittest.ExecutionResultFixture()
	previous.BlockID = block.Header.ParentID
	incorporatedResult.Result.PreviousResultID = previous.ID()

	// add incorporated result to mempool
	ms.pendingResults[incorporatedResult.Result.ID()] = incorporatedResult

	// check that it is looking for the previous result, and return previous
	ms.resultsPL.On("ByResultID", mock.Anything).Run(
		func(args mock.Arguments) {
			previousResultID := args.Get(0).(flow.Identifier)
			ms.Assert().Equal(incorporatedResult.Result.PreviousResultID, previousResultID)
		},
	).Return(previous, nil)

	// check that we are trying to remove the incorporated result from mempool
	ms.resultsPL.On("Rem", mock.Anything).Run(
		func(args mock.Arguments) {
			incResult := args.Get(0).(*flow.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.ID(), incResult.ID())
		},
	).Return(true)

	// list of 3 approvers
	assignedApprovers := ms.approvers[:3]

	// create assignment with 3 verification node assigned to every chunk
	assignment := chunks.NewAssignment()
	for _, chunk := range incorporatedResult.Result.Chunks {
		assignment.Add(chunk, assignedApprovers.NodeIDs())
	}
	// mock assigner
	ms.assigner.On("Assign", incorporatedResult.Result, incorporatedResult.IncorporatedBlockID).Return(assignment, nil)

	realApprovalPool, err := stdmap.NewApprovals(1000)
	ms.Require().NoError(err)
	ms.matching.approvals = realApprovalPool

	// approve every chunk by an unassigned verifier.
	unassignedApprover := ms.approvers[3]
	for index := uint64(0); index < uint64(len(incorporatedResult.Result.Chunks)); index++ {
		approval := unittest.ResultApprovalFixture()
		approval.Body.BlockID = block.Header.ID()
		approval.Body.ExecutionResultID = incorporatedResult.Result.ID()
		approval.Body.ApproverID = unassignedApprover.NodeID
		approval.Body.ChunkIndex = index
		_, err := ms.matching.approvals.Add(approval)
		ms.Require().NoError(err)
	}

	results, err := ms.matching.sealableResults()
	ms.Require().NoError(err)
	ms.Assert().Len(results, 0, "should not count approvals from unassigned verifiers")
}

func (ms *MatchingSuite) TestSealResultValid() {

	// try to seal a result with a persisted previous result
	block := unittest.BlockFixture()
	ms.blocks[block.Header.ID()] = &block
	incorporatedResult := unittest.IncorporatedResultForBlockFixture(&block)
	previous := unittest.ExecutionResultFixture()
	previous.BlockID = block.Header.ParentID
	incorporatedResult.Result.PreviousResultID = previous.ID()

	realApprovalPool, err := stdmap.NewApprovals(1000)
	ms.Require().NoError(err)
	ms.matching.approvals = realApprovalPool

	// create 1 approval for each chunk in result and add to mempool
	approver := ms.approvers[0]
	for index := uint64(0); index < uint64(len(incorporatedResult.Result.Chunks)); index++ {
		approval := unittest.ResultApprovalFixture()
		approval.Body.BlockID = block.Header.ID()
		approval.Body.ExecutionResultID = incorporatedResult.Result.ID()
		approval.Body.ApproverID = approver.NodeID
		approval.Body.ChunkIndex = index
		_, err := ms.matching.approvals.Add(approval)
		ms.Require().NoError(err)
	}

	// check match when we are storing entities
	ms.resultsDB.On("Store", mock.Anything).Run(
		func(args mock.Arguments) {
			stored := args.Get(0).(*flow.ExecutionResult)
			ms.Assert().Equal(incorporatedResult.Result, stored)
		},
	).Return(nil)
	ms.sealsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			seal := args.Get(0).(*flow.IncorporatedResultSeal)
			ms.Assert().Equal(incorporatedResult, seal.IncorporatedResult)
			ms.Assert().Equal(incorporatedResult.Result.BlockID, seal.Seal.BlockID)
			ms.Assert().Equal(incorporatedResult.Result.ID(), seal.Seal.ResultID)
		},
	).Return(true)

	// check that sigs has chunks many signatures
	sigs := ms.matching.collectAggregateSignatures(incorporatedResult.Result)
	ms.Equal(len(sigs), incorporatedResult.Result.Chunks.Len())

	// check that each aggregated signature has 1 signer and signature
	for _, sig := range sigs {
		ms.Equal(len(sig.SignerIDs), 1)
		ms.Equal(len(sig.VerifierSignatures), 1)
	}

	err = ms.matching.sealResult(incorporatedResult)
	ms.Require().NoError(err, "should generate seal on persisted previous result")

	ms.sealsPL.AssertNumberOfCalls(ms.T(), "Add", 1)
}

func (ms *MatchingSuite) TestRequestReceiptsPendingBlocks() {
	n := 100

	// Create n consecutive blocks
	// the first one is sealed and the last one is final

	headers := []flow.Header{}

	parentHeader := flow.Header{
		ChainID:        flow.Emulator,
		ParentID:       unittest.IdentifierFixture(),
		Height:         0,
		PayloadHash:    unittest.IdentifierFixture(),
		Timestamp:      time.Now().UTC(),
		View:           uint64(rand.Intn(1000)),
		ParentVoterIDs: unittest.IdentifierListFixture(4),
		ParentVoterSig: unittest.SignatureFixture(),
		ProposerID:     unittest.IdentifierFixture(),
		ProposerSig:    unittest.SignatureFixture(),
	}

	for i := 0; i < n; i++ {
		newHeader := unittest.BlockHeaderWithParentFixture(&parentHeader)
		parentHeader = newHeader
		headers = append(headers, newHeader)
	}

	orderedBlocks := []flow.Block{}
	for i := 0; i < n; i++ {
		payload := unittest.PayloadFixture()
		header := headers[i]
		header.PayloadHash = payload.Hash()
		block := flow.Block{
			Header:  &header,
			Payload: payload,
		}
		ms.blocks[block.ID()] = &block
		orderedBlocks = append(orderedBlocks, block)
	}

	ms.state = &protocol.State{}

	ms.state.On("Final").Return(
		func() realproto.Snapshot {
			snapshot := &protocol.Snapshot{}
			snapshot.On("Head").Return(
				func() *flow.Header {
					return orderedBlocks[n-1].Header
				},
				nil,
			)
			return snapshot
		},
		nil,
	)

	ms.state.On("Sealed").Return(
		func() realproto.Snapshot {
			snapshot := &protocol.Snapshot{}
			snapshot.On("Head").Return(
				func() *flow.Header {
					return orderedBlocks[0].Header
				},
				nil,
			)
			return snapshot
		},
		nil,
	)

	ms.matching.state = ms.state

	// the receipts pool is empty, which will trigger request
	ms.receiptsPL.On("All").Return(nil)

	// keep track of requested blocks
	requestedBlocks := []flow.Identifier{}
	ms.requester.On("EntityByID", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			blockID := args.Get(0).(flow.Identifier)
			requestedBlocks = append(requestedBlocks, blockID)
		},
	).Return()

	err := ms.matching.requestPending()
	ms.Require().NoError(err, "should request results for pending blocks")

	// should request n-1 blocks if n > requestReceiptThreshold
	ms.Assert().Equal(len(requestedBlocks), n-1)
}
