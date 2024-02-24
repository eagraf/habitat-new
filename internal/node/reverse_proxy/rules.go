package reverse_proxy

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
