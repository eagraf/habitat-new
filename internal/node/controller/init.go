package controller

import (
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/google/uuid"
)

// TODO this is basically a placeholder until we actually have a way of generating
// the certificate for the node.
func generateInitState(nodeConfig *config.NodeConfig) (*node.State, error) {
	nodeUUID := uuid.New().String()

	rootCert := nodeConfig.RootUserCertB64()

	initState, err := node.GetEmptyStateForVersion(node.LatestVersion)
	if err != nil {
		return nil, err
	}

	initState.NodeID = nodeUUID
	initState.Users[constants.RootUserID] = &node.User{
		ID:          constants.RootUserID,
		Username:    constants.RootUsername,
		Certificate: rootCert,
	}

	return initState, nil
}

func initTranstitions(nodeConfig *config.NodeConfig) ([]hdb.Transition, error) {

	initState, err := generateInitState(nodeConfig)
	if err != nil {
		return nil, err
	}

	transitions := []hdb.Transition{
		&node.InitalizationTransition{
			InitState: initState,
		},
	}
	return transitions, nil
}
