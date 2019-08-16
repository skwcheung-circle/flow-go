package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/grpc/services/observe"
	"github.com/dapperlabs/bamboo-node/pkg/types"
	"github.com/dapperlabs/bamboo-node/pkg/types/proto"
)

// RPCClient is an RPC client compatible with the Bamboo Observation API.
type RPCClient observe.ObserveServiceClient

// Client is a Bamboo user agent client.
type Client struct {
	rpcClient RPCClient
	close     func() error
}

// New initializes a Bamboo client with the default gRPC provider.
//
// An error will be returned if the host is unreachable.
func New(host string, port int) (*Client, error) {
	addr := fmt.Sprintf("%s:%d", host, port)

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	grpcClient := observe.NewObserveServiceClient(conn)

	return &Client{
		rpcClient: grpcClient,
		close:     func() error { return conn.Close() },
	}, nil
}

// NewFromRPCClient initializes a Bamboo client using a pre-configured gRPC provider.
func NewFromRPCClient(rpcClient RPCClient) *Client {
	return &Client{
		rpcClient: rpcClient,
		close:     func() error { return nil },
	}
}

// Close closes the client connection.
func (c *Client) Close() error {
	return c.close()
}

// SendTransaction submits a transaction to the network.
func (c *Client) SendTransaction(ctx context.Context, tx types.SignedTransaction) error {
	txMsg, err := proto.SignedTransactionToMessage(tx)
	if err != nil {
		return err
	}

	_, err = c.rpcClient.SendTransaction(
		ctx,
		&observe.SendTransactionRequest{Transaction: txMsg},
	)
	return err
}

// GetTransaction fetches a transaction by hash.
func (c *Client) GetTransaction(ctx context.Context, h crypto.Hash) (*types.SignedTransaction, error) {
	res, err := c.rpcClient.GetTransaction(
		ctx,
		&observe.GetTransactionRequest{Hash: h.Bytes()},
	)
	if err != nil {
		return nil, err
	}

	tx := res.GetTransaction()
	payerSig := tx.GetPayerSignature()

	return &types.SignedTransaction{
		Script:       tx.GetScript(),
		Nonce:        tx.GetNonce(),
		ComputeLimit: tx.GetComputeLimit(),
		ComputeUsed:  tx.GetComputeUsed(),
		PayerSignature: types.AccountSignature{
			Account:   types.BytesToAddress(payerSig.GetAccount()),
			Signature: payerSig.GetSignature(),
		},
		Status: types.TransactionStatus(tx.GetStatus()),
	}, nil
}

// GetAccount fetches an account by address.
func (c *Client) GetAccount(ctx context.Context, address types.Address) (*types.Account, error) {
	res, err := c.rpcClient.GetAccount(
		ctx,
		&observe.GetAccountRequest{Address: address.Bytes()},
	)
	if err != nil {
		return nil, err
	}

	account := res.GetAccount()

	return &types.Account{
		Address:    types.BytesToAddress(account.GetAddress()),
		Balance:    account.GetBalance(),
		Code:       account.GetCode(),
		PublicKeys: account.GetPublicKeys(),
	}, nil
}
