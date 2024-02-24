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
	"github.com/eagraf/habitat-new/internal/node/api/test_helpers"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/eagraf/habitat-new/internal/node/controller/mocks"
	hdb_mocks "github.com/eagraf/habitat-new/internal/node/hdb/mocks"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestInstallAppHandler(t *testing.T) {
	ctrl := gomock.NewController(t)

	m := mocks.NewMockNodeController(ctrl)

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

	// Test invalid request
	m.EXPECT().StartProcess(body.AppInstallation).Times(0)
	resp, err = client.Post(url, "application/json", bytes.NewBuffer([]byte("invalid")))
	assert.Nil(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestStartProcessHandler(t *testing.T) {
	ctrl := gomock.NewController(t)

	m := mocks.NewMockNodeController(ctrl)

	handler := NewStartProcessHandler(m)

	router := mux.NewRouter()
	middleware := &test_helpers.TestAuthMiddleware{UserID: "user_1"}
	router.Use(middleware.Middleware)
	router.Handle(handler.Pattern(), handler).Methods(handler.Method())

	server := httptest.NewServer(router)

	body := &types.PostProcessRequest{
		Process: &node.Process{
			ID:     "process_1",
			AppID:  "app_1",
			UserID: "user_1",
		},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		t.Error(err)
	}

	m.EXPECT().StartProcess(&node.Process{
		ID:     "process_1",
		AppID:  "app_1",
		UserID: "user_1",
	}).Return(nil).Times(1)

	client := server.Client()
	url := fmt.Sprintf("%s/node/processes", server.URL)

	// Test the happy path
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(bodyBytes))
	assert.Nil(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(respBody))

	// Test an error returned by the controller
	m.EXPECT().StartProcess(body.Process).Return(errors.New("Couldn't install app")).Times(1)
	resp, err = client.Post(url, "application/json", bytes.NewBuffer(bodyBytes))
	assert.Nil(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	// Test invalid request
	m.EXPECT().StartProcess(body.Process).Times(0)
	resp, err = client.Post(url, "application/json", bytes.NewBuffer([]byte("invalid")))
	assert.Nil(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetNodeHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := hdb_mocks.NewMockHDBManager(ctrl)
	mockClient := hdb_mocks.NewMockClient(ctrl)

	// Note: not using generateInitState() to test
	testState := map[string]interface{}{
		"state_prop":   "state_val",
		"state_prop_2": "state_val_2",
		// "number":       int(1), <-- this does not pass because the unmarshal after writing decodes number as float64 by default
		"bool":  true,
		"float": 1.65,
	}
	bytes, err := json.Marshal(testState)
	require.Nil(t, err)

	mockDB.EXPECT().GetDatabaseClientByName(constants.NodeDBDefaultName).Return(mockClient, nil)
	mockClient.EXPECT().Bytes().Return(bytes)
	id := uuid.New().String()
	mockClient.EXPECT().DatabaseID().Return(id)

	handler := NewGetNodeRoute(mockDB)

	router := mux.NewRouter()
	router.Handle(handler.Pattern(), handler).Methods(handler.Method())

	server := httptest.NewServer(router)
	client := server.Client()
	url := fmt.Sprintf("%s%s", server.URL, handler.Pattern())

	resp, err := client.Get(url)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	bytes, err = io.ReadAll(resp.Body)
	require.Nil(t, err)

	var respBody types.GetNodeResponse
	require.Nil(t, json.Unmarshal(bytes, &respBody))
	for k, v := range respBody.State {
		v2, ok := testState[k]
		require.True(t, ok)
		require.Equal(t, v, v2)
	}

}

func TestAddUserHandler(t *testing.T) {
	ctrl := gomock.NewController(t)

	m := mocks.NewMockNodeController(ctrl)
	handler := NewAddUserRoute(m)

	router := mux.NewRouter()
	router.Handle(handler.Pattern(), handler).Methods(handler.Method())

	server := httptest.NewServer(router)
	client := server.Client()
	url := fmt.Sprintf("%s%s", server.URL, handler.Pattern())

	body := &types.PostAddUserRequest{
		UserID:      "myUserID",
		Username:    "myUsername",
		Certificate: "myCert",
	}
	b, err := json.Marshal(body)
	require.Nil(t, err)

	m.EXPECT().AddUser("myUserID", "myUsername", "myCert").Return(nil)

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(b))
	require.Nil(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Test internal server error
	m.EXPECT().AddUser("myUserID", "myUsername", "myCert").Return(errors.New("error adding user"))

	resp, err = client.Post(url, "application/json", bytes.NewBuffer(b))
	require.Nil(t, err)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	// Test invalid request
	m.EXPECT().AddUser("myUserID", "myUsername", "myCert").Times(0)
	resp, err = client.Post(url, "application/json", bytes.NewBuffer([]byte("invalid")))
	require.Nil(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
