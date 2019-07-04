package types

import (
	"time"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type Block struct {
	ChainID                string
	Height                 uint64
	PreviousBlockHash      crypto.Hash
	Timestamp              time.Time
	SignedCollectionHashes []SignedCollectionHash
	BlockSeals             []BlockSeal
	Signatures             []crypto.Signature
}

type BlockSeal struct {
	BlockHash                  crypto.Hash
	ExecutionReceiptHash       crypto.Hash
	ExecutionReceiptSignatures []crypto.Signature
	ResultApprovalSignatures   []crypto.Signature
}
