package reverse_proxy

import (
	"encoding/json"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/rs/zerolog/log"
)

type ReverseProxyRestorer struct {
	ruleSet RuleSet
}

func (r *ReverseProxyRestorer) Restore(restoreEvent *hdb.StateUpdate) error {
	var nodeState node.NodeState
	err := json.Unmarshal(restoreEvent.NewState, &nodeState)
	if err != nil {
		return err
	}

	for _, process := range nodeState.Processes {
		rules, err := nodeState.GetReverseProxyRulesForProcess(process.ID)
		if err != nil {
			return err
		}

		for _, rule := range rules {
			log.Info().Msgf("Restoring rule %s", rule)
			err = r.ruleSet.AddRule(rule)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
