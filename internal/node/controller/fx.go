package controller

import (
	"context"

	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"go.uber.org/fx"
)

func NewNodeController(lc fx.Lifecycle, habitatDBManager hdb.HDBManager, config *config.NodeConfig) *BaseNodeController {
	controller := &BaseNodeController{
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
