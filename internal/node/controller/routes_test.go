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
	"github.com/eagraf/habitat-new/internal/node/constants"
	hdb_mocks "github.com/eagraf/habitat-new/internal/node/hdb/mocks"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestInstallAppHandler(t *testing.T) {
	ctrl := gomock.NewController(t)

	m := NewMockNodeController(ctrl)

	handler := NewInstallAppRoute(m)

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

func TestGetNodeHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := hdb_mocks.NewMockHDBManager(ctrl)
	mockClient := hdb_mocks.NewMockClient(ctrl)

	mockDB.EXPECT().GetDatabaseClientByName(constants.NodeDBDefaultName).Return(mockClient, nil)

	handler := NewGetNodeRoute(mockDB)

	router := mux.NewRouter()
	router.Handle(handler.Pattern(), handler).Methods(handler.Method())

	server := httptest.NewServer(router)
	client := server.Client()
	url := fmt.Sprintf("%s/%s", server.URL, handler.Pattern())

	resp, err := client.Get(url)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	bytes, err := io.ReadAll(resp.Body)
	require.Nil(t, err)
	fmt.Println(string(bytes))
}

func TestAddUserHandler(t *testing.T) {}
