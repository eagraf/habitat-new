package node

import "github.com/eagraf/habitat-new/internal/node/hdb"

type NodeStateUpdate struct {
	state             *NodeState
	transitionWrapper *hdb.TransitionWrapper
}

func NewNodeStateUpdateInternal(state *NodeState, transitionWrapper *hdb.TransitionWrapper) *NodeStateUpdate {
	return &NodeStateUpdate{
		state:             state,
		transitionWrapper: transitionWrapper,
	}
}

func (n *NodeStateUpdate) NewState() hdb.State {
	return n.state
}

func (n *NodeStateUpdate) Transition() []byte {
	return n.transitionWrapper.Transition
}

func (n *NodeStateUpdate) TransitionType() string {
	return n.transitionWrapper.Type
}

func (n *NodeStateUpdate) NodeState() *NodeState {
	return n.state
}
