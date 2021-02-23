package dkg

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/signature"
)

// ControllerFactory is a factory object that creates new Controllers with the
// same underlying tunnel to communicate with the network engine, and dkg
// smart-contract client to relay broadcast messages.
type ControllerFactory struct {
	log               zerolog.Logger
	dkgContractClient module.DKGContractClient
	tunnel            *BrokerTunnel
}

// NewControllerFactory creates a new factory that generates Controllers with
// the specified tunnel and dkg smart-contract client.
func NewControllerFactory(
	log zerolog.Logger,
	dkgContractClient module.DKGContractClient,
	tunnel *BrokerTunnel) *ControllerFactory {

	return &ControllerFactory{
		log:               log,
		dkgContractClient: dkgContractClient,
		tunnel:            tunnel,
	}
}

// Create creates a new epoch-specific Controller equipped with a broker which
// is capable of communicating with other nodes.
func (f *ControllerFactory) Create(
	dkgInstanceID string,
	participants []flow.Identifier,
	myIndex int,
	seed []byte) (*Controller, error) {

	broker := NewBroker(
		f.log,
		dkgInstanceID,
		participants,
		myIndex,
		f.dkgContractClient,
		f.tunnel,
	)

	n := len(participants)
	threshold := signature.RandomBeaconThreshold(n)
	dkg, err := crypto.NewJointFeldman(n, threshold, myIndex, broker)
	if err != nil {
		return nil, err
	}

	controller := NewController(
		f.log,
		dkgInstanceID,
		dkg,
		seed,
		broker,
	)

	return controller, nil
}
