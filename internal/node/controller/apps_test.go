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

	"github.com/eagraf/habitat-new/api/types"
	"github.com/eagraf/habitat-new/internal/node/habitat_db/state/schemas/node"
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
