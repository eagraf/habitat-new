package reverse_proxy

import "fmt"

type ProxyRuleType string

const (
	ProxyRuleFileServer = "file"
	ProxyRuleRedirect   = "redirect"
)

type ProxyRule struct {
	Type    string `yaml:"type"`
	Matcher string `yaml:"matcher"`
	Target  string `yaml:"target"`
}

func (r ProxyRule) Identifer() string {
	// TODO this is not a foolproof way of uniquely identifying rules, need to fix
	return fmt.Sprintf("type-%s-matcher-%s-target-%s", r.Type, r.Matcher, r.Target)
}
