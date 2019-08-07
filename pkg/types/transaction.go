package types

import (
	"fmt"
	"strings"
	"time"

	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
)

// TransactionStatus represents the status of a Transaction.
type TransactionStatus int

const (
	// TransactionPending is the status of a pending transaction.
	TransactionPending TransactionStatus = iota
	// TransactionFinalized is the status of a finalized transaction.
	TransactionFinalized
	// TransactionReverted is the status of a reverted transaction.
	TransactionReverted
	// TransactionSealed is the status of a sealed transaction.
	TransactionSealed
)

// String returns the string representation of a transaction status.
func (s TransactionStatus) String() string {
	return [...]string{"PENDING", "FINALIZED", "REVERTED", "SEALED"}[s]
}

// RawTransaction is an unsigned transaction.
type RawTransaction struct {
	Script       []byte
	Nonce        uint64
	ComputeLimit uint64
	Timestamp    time.Time
}

// Hash computes the hash over the necessary transaction data.
func (tx *RawTransaction) Hash() crypto.Hash {
	bytes := crypto.EncodeAsBytes(
		tx.Script,
		tx.Nonce,
		tx.ComputeLimit,
		tx.Timestamp,
	)
	return crypto.NewHash(bytes)
}

// SetNonce sets the nonce for the transaction.
func (tx *RawTransaction) SetNonce(nonce uint64) {
	tx.Nonce = nonce
}

// SetComputeLimit sets the compute limit for the transaction.
func (tx *RawTransaction) SetComputeLimit(computeLimit uint64) {
	tx.ComputeLimit = computeLimit
}

// SignPayer signs the transaction with the given account and keypair.
//
// The function returns a new SignedTransaction that includes the generated signature.
func (tx *RawTransaction) SignPayer(key *AccountKey) *SignedTransaction {
	hash := tx.Hash()

	// TODO: include account in signature
	sig := crypto.Sign(hash, key.KeyPair)

	accountSig := AccountSignature{
		Account:   key.Account,
		PublicKey: key.KeyPair.PublicKey,
		Signature: sig.Bytes(),
	}

	return &SignedTransaction{
		Script:         tx.Script,
		Nonce:          tx.Nonce,
		ComputeLimit:   tx.ComputeLimit,
		Timestamp:      tx.Timestamp,
		PayerSignature: accountSig,
	}
}

// SignedTransaction is a transaction that has been signed by at least one account.
type SignedTransaction struct {
	Script         []byte
	Nonce          uint64
	ComputeLimit   uint64
	ComputeUsed    uint64
	Timestamp      time.Time
	PayerSignature AccountSignature
	Status         TransactionStatus
}

// Hash computes the hash over the necessary transaction data.
func (tx *SignedTransaction) Hash() crypto.Hash {
	bytes := crypto.EncodeAsBytes(
		tx.Script,
		tx.Nonce,
		tx.ComputeLimit,
		tx.Timestamp,
		tx.PayerSignature.Bytes(),
	)
	return crypto.NewHash(bytes)
}

// InvalidTransactionError indicates that a transaction does not contain all
// required information.
type InvalidTransactionError struct {
	missingFields []string
}

func (e InvalidTransactionError) Error() string {
	return fmt.Sprintf(
		"required fields are not set: %s",
		strings.Join(e.missingFields[:], ", "),
	)
}

// Validate returns and error if the transaction does not contain all required
// fields.
func (tx *SignedTransaction) Validate() error {
	missingFields := make([]string, 0)

	if len(tx.Script) == 0 {
		missingFields = append(missingFields, "script")
	}

	if tx.ComputeLimit == 0 {
		missingFields = append(missingFields, "compute_limit")
	}

	if len(missingFields) > 0 {
		return InvalidTransactionError{missingFields}
	}

	return nil
}
