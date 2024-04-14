package controller

import (
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/hdb"
)

func NewNodeController(habitatDBManager hdb.HDBManager, config *config.NodeConfig) *BaseNodeController {
	controller := &BaseNodeController{
		databaseManager: habitatDBManager,
		nodeConfig:      config,
	}
	controller.InitializeNodeDB()
	return controller
}
