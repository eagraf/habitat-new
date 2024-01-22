package habitat_db

import (
	"context"

	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

func NewHabitatDB(lc fx.Lifecycle, logger *zerolog.Logger) *DatabaseManager {
	dbManager, err := NewDatabaseManager()
	if err != nil {
		logger.Fatal().Err(err).Msg("Error initializing Habitat DB")
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go dbManager.RestartDBs()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			// TODO gracefully stop all databases
			return nil
		},
	})
	return dbManager
}
