package appstore

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"

	types "github.com/eagraf/habitat-new/core/api"
	yaml "gopkg.in/yaml.v3"
)

//go:embed apps.dev.yml
var appsDevYml embed.FS

// getAppsList returns the contents of the embedded apps.dev.yml file
func getDevAppsList() ([]*types.PostAppRequest, error) {
	yml, err := fs.ReadFile(appsDevYml, "apps.dev.yml")
	if err != nil {
		return nil, err
	}

	var appsList []*types.PostAppRequest
	err = yaml.Unmarshal(yml, &appsList)
	if err != nil {
		return nil, err
	}

	return appsList, nil
}

// AvailableAppsRoute lists apps the user is able to install.
type AvailableAppsRoute struct {
}

func NewAvailableAppsRoute() *AvailableAppsRoute {
	return &AvailableAppsRoute{}
}

func (h *AvailableAppsRoute) Pattern() string {
	return "/app_store/available_apps"
}

func (h *AvailableAppsRoute) Method() string {
	return http.MethodGet
}

func (h *AvailableAppsRoute) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO only run this in dev mode
	apps, err := getDevAppsList()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	marsahalled, err := json.Marshal(apps)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(marsahalled)
}
