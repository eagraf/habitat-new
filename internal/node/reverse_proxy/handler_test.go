package reverse_proxy

import (
	"testing"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/config"
)

func TestGetHandlerFromRule(t *testing.T) {

	fakeConfig, err := config.NewNodeConfig()
	if err != nil {
		t.Error(err)
	}

	redirectRule := &node.ReverseProxyRule{
		ID:     "redirect1",
		Type:   node.ProxyRuleRedirect,
		Target: "http://fake-target/api",
	}

	getHandlerFromRule(redirectRule, fakeConfig)
}
