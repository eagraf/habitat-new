package frontend

import (
	"net/url"

	"github.com/eagraf/habitat-new/internal/node/reverse_proxy"
	"github.com/spf13/viper"
)

func NewFrontendProxyRule() reverse_proxy.RuleHandler {
	viper.BindEnv("frontend_dev", "FRONTEND_DEV")
	if viper.GetBool("frontend_dev") {
		feDevServer, _ := url.Parse("http://habitat_frontend:8000/")
		// The root matcher is empty, so this rule will match all requests that don't have a more specific rule
		return &reverse_proxy.RedirectRule{
			Matcher:         "",
			ForwardLocation: feDevServer,
		}
	} else {
		return &reverse_proxy.FileServerRule{
			Matcher: "",
			Path:    "frontend/out",
		}
	}
}
