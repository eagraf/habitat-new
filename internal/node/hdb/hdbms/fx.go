package hdbms

import (
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/node/pubsub"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

type HDBResult struct {
	fx.Out
	Manager              hdb.HDBManager
	StateUpdatePublisher pubsub.Publisher[hdb.StateUpdate] `group:"state_update_publishers"`
}

func NewHabitatDB(logger *zerolog.Logger, config *config.NodeConfig) (HDBResult, func(), error) {
	publisher := pubsub.NewSimplePublisher[hdb.StateUpdate]()
	dbManager, err := NewDatabaseManager(config, publisher)
	if err != nil {
		return HDBResult{}, func() {}, err
	}

	err = dbManager.RestartDBs()
	if err != nil {
		return HDBResult{}, func() {}, err
	}

	go dbManager.Start()

	return HDBResult{
		Manager:              dbManager,
		StateUpdatePublisher: publisher,
	}, dbManager.Stop, nil
}

// StateUpdateLogger is a subscriber for StateUpdates that logs them.
type StateUpdateLogger struct {
	logger *zerolog.Logger
}

func (s *StateUpdateLogger) Name() string {
	return "StateUpdateLogger"
}

func (s *StateUpdateLogger) ConsumeEvent(event *hdb.StateUpdate) error {
	if event.Restore {
		s.logger.Info().Msgf("Restoring state for %s", event.DatabaseID)
		return nil
	} else {
		s.logger.Info().Msgf("Applying transition %s to %s", string(event.Transition), event.DatabaseID)
		return nil
	}
}

func NewStateUpdateLogger(logger *zerolog.Logger) *StateUpdateLogger {
	return &StateUpdateLogger{
		logger: logger,
	}
}
