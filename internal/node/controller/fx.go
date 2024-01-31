package controller

import (
	"context"

	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/habitat_db"
	"go.uber.org/fx"
)

func NewNodeController(lc fx.Lifecycle, habitatDBManager *habitat_db.DatabaseManager, config *config.NodeConfig) *NodeController {
	controller := &NodeController{
		databaseManager: habitatDBManager,
		nodeConfig:      config,
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return controller.InitializeNodeDB()
		},
		OnStop: func(ctx context.Context) error {
			return nil
		},
	})
	return controller
}
