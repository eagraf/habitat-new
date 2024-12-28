package node

// Core structs for the node state. These are intended to be embedable in other structs
// throughout the application. That way, it's easy to modify the core struct, while having
// the component specific structs to be decoupled. Fields in these structs should be immutable.

// TODO to make these truly immutable, only methods should be exported, all fields should be private.

// ReverseProxyRule matches a URL path to a target of the given type.
// There are two types of rules currently:
//  1. File server: serves files from a given directory (useful for serving websites from Habitat)
//  2. Redirect: redirects to a given URL (useful for exposing APIs for Habitat applications)
//
// The matcher field represents the path that the rule should match.
// The semantics of the target field changes depending on the type. For file servers, it represents the
// path to the directory to serve files from. For redirects, it represents the URL to redirect to.
type ReverseProxyRule struct {
	ID      string               `json:"id" yaml:"id"`
	Type    ReverseProxyRuleType `json:"type" yaml:"type"`
	Matcher string               `json:"matcher" yaml:"matcher"`
	Target  string               `json:"target" yaml:"target"`
	AppID   string               `json:"app_id" yaml:"app_id"`
}

type ReverseProxyRuleType = string

const (
	ProxyRuleFileServer       ReverseProxyRuleType = "file"
	ProxyRuleRedirect         ReverseProxyRuleType = "redirect"
	ProxyRuleEmbeddedFrontend ReverseProxyRuleType = "embedded_frontend"
)
