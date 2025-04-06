package types

import node "github.com/eagraf/habitat-new/core/state/node"

type GetNodeResponse struct {
	State node.State `json:"state"`
}
