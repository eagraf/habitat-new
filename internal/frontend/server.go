package frontend

import (
	"net/url"

	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/reverse_proxy"
)

func NewFrontendProxyRule(config *config.NodeConfig) reverse_proxy.RuleHandler {
	if config.FrontendDev() {
		feDevServer, _ := url.Parse("http://habitat_frontend:8000/")
		return &reverse_proxy.RedirectRule{
			Matcher:         "/",
			ForwardLocation: feDevServer,
		}
	} else {
		return &reverse_proxy.FileServerRule{
			Matcher: "/",
			Path:    "frontend/out",
		}
	}
}
