package verification

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/local"
	module_mock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

const epochCounter = uint64(42)

func MakeSigners(t *testing.T,
	committee hotstuff.Committee,
	signerIDs []flow.Identifier,
	stakingKeys []crypto.PrivateKey,
	beaconKeys []crypto.PrivateKey) []hotstuff.SignerVerifier {

	// generate our consensus node identities
	require.NotEmpty(t, signerIDs)

	var signers []hotstuff.SignerVerifier
	if len(beaconKeys) != len(stakingKeys) {
		for i, signerID := range signerIDs {
			signer := MakeStakingSigner(t, committee, signerID, stakingKeys[i])
			signers = append(signers, signer)
		}
	} else {
		for i, signerID := range signerIDs {
			signer := MakeBeaconSigner(t, committee, signerID, stakingKeys[i], beaconKeys[i])
			signers = append(signers, signer)
		}
	}

	return signers
}

func makeLocalWithSignerAndKey(signerID flow.Identifier, sk crypto.PrivateKey) (module.Local, error) {
	nodeID := unittest.IdentityFixture()
	nodeID.NodeID = signerID
	nodeID.StakingPubKey = sk.PublicKey()
	return local.New(nodeID, sk)
}

func MakeStakingSigner(t *testing.T, committee hotstuff.Committee, signerID flow.Identifier, priv crypto.PrivateKey) *SingleSignerVerifier {
	local, err := makeLocalWithSignerAndKey(signerID, priv)
	require.NoError(t, err)
	staking := signature.NewAggregationProvider("test_staking", local)
	signer := NewSingleSignerVerifier(committee, staking, signerID)
	return signer
}

func MakeBeaconSigner(t *testing.T,
	committee hotstuff.Committee,
	signerID flow.Identifier,
	stakingPriv crypto.PrivateKey,
	beaconPriv crypto.PrivateKey) *CombinedSigner {

	local, err := makeLocalWithSignerAndKey(signerID, stakingPriv)
	require.NoError(t, err)

	combiner := signature.NewCombiner(encodable.ConsensusVoteSigLen, encodable.RandomBeaconSigLen)
	staking := signature.NewAggregationProvider("test_staking", local)
	thresholdVerifier := signature.NewThresholdVerifier("test_beacon")
	thresholdSigner := signature.NewThresholdProvider("test_beacon", beaconPriv)
	thresholdSignerStore := &module_mock.ThresholdSignerStore{}
	thresholdSignerStore.On("GetThresholdSigner", mock.Anything).Return(thresholdSigner, nil)

	signer := NewCombinedSigner(committee, staking, thresholdVerifier, combiner, thresholdSignerStore, signerID)

	return signer
}

func MakeHotstuffCommitteeState(t *testing.T, identities flow.IdentityList, beaconEnabled bool, epochCounter uint64) (hotstuff.Committee, []crypto.PrivateKey, []crypto.PrivateKey) {

	// program the MembersSnapshot
	committee := &mocks.Committee{}
	committee.On("Identities", mock.Anything, mock.Anything).Return(
		func(blockID flow.Identifier, selector flow.IdentityFilter) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)
	for _, identity := range identities {
		committee.On("Identity", mock.Anything, identity.NodeID).Return(identity, nil)
	}

	// generate the staking keys
	var stakingKeys []crypto.PrivateKey
	for i := 0; i < len(identities); i++ {
		stakingKey := helper.MakeBLSKey(t)
		identities[i].StakingPubKey = stakingKey.PublicKey()
		stakingKeys = append(stakingKeys, stakingKey)
	}

	// generate the dkg keys (only if beacon is enabled)
	var beaconSKs []crypto.PrivateKey
	var beaconPKs []crypto.PublicKey
	var beaconGroupPK crypto.PublicKey
	if beaconEnabled {
		seed := make([]byte, crypto.SeedMinLenDKG)
		n, err := rand.Read(seed)
		require.NoError(t, err)
		require.Equal(t, n, crypto.SeedMinLenDKG)
		beaconSKs, beaconPKs, beaconGroupPK, err = crypto.ThresholdSignKeyGen(len(identities),
			signature.RandomBeaconThreshold(len(identities)), seed)
		require.NoError(t, err)

		dkg := &mocks.DKG{}
		committee.On("DKG", mock.Anything).Return(dkg, nil)
		dkg.On("Counter").Return(epochCounter)
		dkg.On("Size").Return(uint(len(identities)))
		dkg.On("GroupKey").Return(beaconGroupPK)
		for i, node := range identities {
			share := beaconPKs[i]
			dkg.On("KeyShare", node.NodeID).Return(share, nil)
			dkg.On("Index", node.NodeID).Return(uint(i), nil)
		}
	}

	return committee, stakingKeys, beaconSKs
}
