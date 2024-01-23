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
	return dbManager
}
