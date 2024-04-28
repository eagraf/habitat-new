package reverse_proxy

type ProxyRuleType string

const (
	ProxyRuleFileServer = "file"
	ProxyRuleRedirect   = "redirect"
)
