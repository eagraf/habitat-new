package fishtail

import (
	"encoding/json"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/eagraf/habitat-new/internal/node/hdb"
)

func NewFishtailIngestSubscriber(
	nodeConfig *config.NodeConfig,
	nodeController controller.NodeController,
	publisher *ATProtoEventPublisher,
) (*hdb.IdempotentStateUpdateSubscriber, error) {
	return hdb.NewIdempotentStateUpdateSubscriber(
		"FishtailIngestSubscriber",
		node.SchemaName,
		[]hdb.IdempotentStateUpdateExecutor{
			NewFishtailIngestExecutor(nodeConfig, nodeController, publisher),
		},
		NewFishtailIngestRestorer(nodeConfig, publisher),
	)
}

type FishtailIngestExecutor struct {
	nodeController        controller.NodeController
	atProtoEventPublisher *ATProtoEventPublisher
}

func NewFishtailIngestExecutor(nodeConfig *config.NodeConfig, controller controller.NodeController, publisher *ATProtoEventPublisher) *FishtailIngestExecutor {
	return &FishtailIngestExecutor{
		nodeController:        controller,
		atProtoEventPublisher: publisher,
	}
}

func (f *FishtailIngestExecutor) TransitionType() string {
	return node.TransitionStartProcess
}

func (f *FishtailIngestExecutor) ShouldExecute(update hdb.StateUpdate) (bool, error) {
	return true, nil
}

func (f *FishtailIngestExecutor) Execute(update hdb.StateUpdate) error {
	newState := update.NewState().(*node.State)

	var processStartTransition node.ProcessStartTransition
	err := json.Unmarshal(update.Transition(), &processStartTransition)
	if err != nil {
		return err
	}
	app := processStartTransition.EnrichedData.App

	if newState.ReverseProxyRules == nil {
		return nil
	}

	for _, rule := range *newState.ReverseProxyRules {
		if rule.AppID == app.ID && rule.Type == node.ProxyRuleFishtailIngest {
			if rule.FishtailIngestConfig != nil {
				for _, collection := range rule.FishtailIngestConfig.SubscribedCollections {
					f.atProtoEventPublisher.AddSubscription(collection.Lexicon, rule.Target)
				}
			}
		}
	}

	return nil
}

func (f *FishtailIngestExecutor) PostHook(update hdb.StateUpdate) error {
	return nil
}

type FishtailIngestRestorer struct {
	atProtoEventPublisher *ATProtoEventPublisher
}

func NewFishtailIngestRestorer(nodeConfig *config.NodeConfig, publisher *ATProtoEventPublisher) *FishtailIngestRestorer {
	return &FishtailIngestRestorer{
		atProtoEventPublisher: publisher,
	}
}

func (r *FishtailIngestRestorer) Restore(restoreEvent hdb.StateUpdate) error {
	nodeState := restoreEvent.NewState().(*node.State)
	for _, process := range nodeState.Processes {
		rules, err := nodeState.GetReverseProxyRulesForProcess(process.ID)
		if err != nil {
			return err
		}

		for _, rule := range rules {
			if rule.Type == node.ProxyRuleFishtailIngest {
				if rule.FishtailIngestConfig != nil {
					for _, collection := range rule.FishtailIngestConfig.SubscribedCollections {
						r.atProtoEventPublisher.AddSubscription(collection.Lexicon, rule.Target)
					}
				}
			}
		}
	}
	return nil
}
