package appstore

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	types "github.com/eagraf/habitat-new/core/api"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestRenderDevAppsList(t *testing.T) {
	raw := []byte(`
- app_installation:
    name: pouch_backend
    version: 1
    driver: docker

    driver_config:
      env:
        - PORT=6000
      mounts:
        - type: bind
          source: {{.HabitatPath}}/apps/pouch/database.sqlite
          target: /app/database.sqlite
      exposed_ports:
        - "6000"
      port_bindings:
        "6000/tcp":
          - HostIp: "0.0.0.0"
            HostPort: "6000"

    registry_url_base: registry.hub.docker.com
    registry_app_id: ethangraf/pouch-backend
    registry_tag: release-3`)

	// Create a test node config
	v := viper.New()
	v.Set("habitat_path", "/home/fakeuser/.habitat")
	config, err := config.NewTestNodeConfig(v)
	if err != nil {
		t.Fatal(err)
	}

	apps, err := renderDevAppsList(config, raw)
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, 1, len(apps))
	require.Equal(t, "pouch_backend", apps[0].AppInstallation.Name)
	require.Equal(t, "1", apps[0].AppInstallation.Version)
	require.NotNil(t, apps[0].AppInstallation.DriverConfig)

	driverConfig := apps[0].AppInstallation.DriverConfig
	require.NotNil(t, driverConfig["mounts"])
	require.Equal(t, 1, len(driverConfig["mounts"].([]interface{})))

	mounts := driverConfig["mounts"].([]interface{})
	require.Equal(t, 1, len(mounts))

	mount := mounts[0].(map[string]interface{})
	require.Equal(t, "bind", mount["type"])
	require.Equal(t, "/home/fakeuser/.habitat/apps/pouch/database.sqlite", mount["source"])
}

func TestAvailableAppsRouteDev(t *testing.T) {
	v := viper.New()
	v.Set("environment", constants.EnvironmentDev)
	config, err := config.NewTestNodeConfig(v)
	if err != nil {
		t.Fatal(err)
	}

	handler := NewAvailableAppsRoute(config)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, handler.Pattern(), nil))
	require.Equal(t, http.StatusOK, resp.Result().StatusCode)

	bytes, err := io.ReadAll(resp.Result().Body)
	require.NoError(t, err)

	var respBody []*types.PostAppRequest
	require.NoError(t, json.Unmarshal(bytes, &respBody))

	require.Equal(t, 2, len(respBody))
	require.Equal(t, "pouch_frontend", respBody[0].AppInstallation.Name)
	require.Equal(t, "4", respBody[0].AppInstallation.Version)
}

func TestAvailableAppsRouteProd(t *testing.T) {
	v := viper.New()
	v.Set("environment", constants.EnvironmentProd)
	config, err := config.NewTestNodeConfig(v)
	if err != nil {
		t.Fatal(err)
	}

	handler := NewAvailableAppsRoute(config)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, handler.Pattern(), nil))
	require.Equal(t, http.StatusNotImplemented, resp.Result().StatusCode)
}