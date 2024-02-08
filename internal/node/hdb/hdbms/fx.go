package hdbms

import (
	"context"

	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/node/hdb/state"
	"github.com/eagraf/habitat-new/internal/node/pubsub"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

type HDBResult struct {
	fx.Out
	HabitatDBManager     hdb.HDBManager
	StateUpdatePublisher pubsub.Publisher[state.StateUpdate] `group:"state_update_publishers"`
}

func NewHabitatDB(lc fx.Lifecycle, logger *zerolog.Logger) HDBResult {
	publisher := pubsub.NewSimplePublisher[state.StateUpdate]()
	dbManager, err := NewDatabaseManager(publisher)
	if err != nil {
		logger.Fatal().Err(err).Msg("Error initializing Habitat DB")
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			err := dbManager.RestartDBs()
			if err != nil {
				logger.Error().Err(err).Msg("Error restarting databases")
			}

			go dbManager.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			go dbManager.Stop()
			return nil
		},
	})
	return HDBResult{
		HabitatDBManager:     dbManager,
		StateUpdatePublisher: publisher,
	}
}

// StateUpdateLogger is a subscriber for StateUpdates that logs them.
type StateUpdateLogger struct {
	logger *zerolog.Logger
}

func (s *StateUpdateLogger) Name() string {
	return "StateUpdateLogger"
}

func (s *StateUpdateLogger) ConsumeEvent(event *state.StateUpdate) error {
	s.logger.Info().Msgf("Applying transition %s to %s", string(event.Transition), event.DatabaseID)
	return nil
}

func NewStateUpdateLogger(logger *zerolog.Logger) *StateUpdateLogger {
	return &StateUpdateLogger{
		logger: logger,
	}
}
