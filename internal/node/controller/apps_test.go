package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	types "github.com/eagraf/habitat-new/core/api"
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	hdb_mocks "github.com/eagraf/habitat-new/internal/node/hdb/mocks"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestPostAppHandler(t *testing.T) {
	ctrl := gomock.NewController(t)

	m := NewMockNodeController(ctrl)

	handler := NewPostAppHandler(m)

	router := mux.NewRouter()
	router.Handle(handler.Pattern(), handler).Methods(handler.Method())

	server := httptest.NewServer(router)

	body := &types.PostAppRequest{
		AppInstallation: &node.AppInstallation{
			Name:            "app_name1",
			Version:         "1",
			Driver:          "docker",
			RegistryURLBase: "https://registry.com",
			RegistryAppID:   "app_name1",
			RegistryTag:     "v1",
		},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		t.Error(err)
	}

	m.EXPECT().InstallApp("0", body.AppInstallation).Return(nil).Times(1)

	client := server.Client()
	url := fmt.Sprintf("%s/node/users/0/apps", server.URL)

	// Test the happy path
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(bodyBytes))
	assert.Nil(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(respBody))

	// Test an error returned by the controller
	m.EXPECT().InstallApp("0", body.AppInstallation).Return(errors.New("Couldn't install app")).Times(1)
	resp, err = client.Post(url, "application/json", bytes.NewBuffer(bodyBytes))
	assert.Nil(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestInstallAppController(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockedManager := hdb_mocks.NewMockHDBManager(ctrl)
	mockedClient := hdb_mocks.NewMockClient(ctrl)

	controller := &BaseNodeController{
		databaseManager: mockedManager,
		nodeConfig:      &config.NodeConfig{},
	}

	mockedManager.EXPECT().GetDatabaseClientByName(NodeDBDefaultName).Return(mockedClient, nil).Times(1)
	mockedClient.EXPECT().ProposeTransitions(gomock.Eq(
		[]hdb.Transition{
			&node.StartInstallationTransition{
				UserID: "0",
				AppInstallation: &node.AppInstallation{
					Name:            "app_name1",
					Version:         "1",
					Driver:          "docker",
					RegistryURLBase: "https://registry.com",
					RegistryAppID:   "app_name1",
					RegistryTag:     "v1",
				},
			},
		},
	)).Return(nil, nil).Times(1)

	err := controller.InstallApp("0", &node.AppInstallation{
		Name:            "app_name1",
		Version:         "1",
		Driver:          "docker",
		RegistryURLBase: "https://registry.com",
		RegistryAppID:   "app_name1",
		RegistryTag:     "v1",
	})
	assert.Nil(t, err)
}
