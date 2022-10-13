package corruptnet

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/connection"
	"github.com/onflow/flow-go/network/p2p/unicast"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
)

// AcceptAllTopicValidator pubsub validator func that always returns pubsub.ValidationAccept.
func AcceptAllTopicValidator(context.Context, peer.ID, *pubsub.Message) pubsub.ValidationResult {
	return pubsub.ValidationAccept
}

// CorruptP2PNode is a wrapper around the original LibP2P node.
type CorruptP2PNode struct {
	*p2pnode.Node
}

// Subscribe subscribes the node to the given topic with a noop topic validator.
// All errors returned from this function can be considered benign.
func (n *CorruptP2PNode) Subscribe(topic channels.Topic, _ pubsub.ValidatorEx) (*pubsub.Subscription, error) {
	return n.Node.Subscribe(topic, AcceptAllTopicValidator)
}

func NewCorruptLibP2PNode(logger zerolog.Logger, host host.Host, pCache *p2pnode.ProtocolPeerCache, uniMgr *unicast.Manager, peerManager *connection.PeerManager) p2p.LibP2PNode {
	node := p2pnode.NewNode(logger, host, pCache, uniMgr, peerManager)
	return &CorruptP2PNode{node}
}
