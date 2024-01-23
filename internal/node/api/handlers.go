package api

import "net/http"

type VersionHandler struct {
}

func NewVersionHandler() *VersionHandler {
	return &VersionHandler{}
}

func (h *VersionHandler) Pattern() string {
	return "/version"
}

func (h *VersionHandler) Method() string {
	return http.MethodGet
}

func (h *VersionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("v0.0.1"))
}
