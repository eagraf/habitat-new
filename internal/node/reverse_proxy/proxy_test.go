package reverse_proxy

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/logging"
	"github.com/stretchr/testify/require"
)

func TestProxy(t *testing.T) {
	// Simulate a server sitting behind the reverse proxy
	redirectedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello, World!")
	}))
	defer redirectedServer.Close()

	redirectedServerURL, err := url.Parse(redirectedServer.URL)
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Simulate a static file dir to be served
	fileDir := t.TempDir()
	defer os.RemoveAll(fileDir)

	file, err := os.CreateTemp(fileDir, "file")
	if err != nil {
		log.Fatalf(err.Error())
	}

	_, _ = file.Write([]byte("Hello, World!"))
	file.Close()

	// Create proxy server
	proxy := NewProxyServer(logging.NewLogger(), &config.NodeConfig{})
	err = proxy.Rules.Add("backend1", &RedirectRule{
		Matcher:         "/backend1",
		ForwardLocation: redirectedServerURL,
	})
	require.Nil(t, err)

	// Test adding naming conflict
	err = proxy.Rules.Add("backend1", &RedirectRule{
		Matcher:         "/backend1",
		ForwardLocation: redirectedServerURL,
	})
	require.NotNil(t, err)

	err = proxy.Rules.Add("fileserver", &FileServerRule{
		Matcher: "/fileserver",
		Path:    fileDir,
	})
	require.Nil(t, err)
	require.Equal(t, 2, len(proxy.Rules))

	close, err := proxy.Start("127.0.0.1:9898", nil)
	require.Nil(t, err)
	defer close()

	url := "http://" + proxy.server.Addr
	// Check redirection
	resp, err := http.Get(url + "/backend1")
	if err != nil {
		t.Errorf(err.Error())
	}

	require.Equal(t, http.StatusOK, resp.StatusCode)

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf(err.Error())
	}

	require.Equal(t, "Hello, World!", string(b))

	// Check file serving
	resp, err = http.Get(fmt.Sprintf("%s/fileserver/%s", url, filepath.Base(file.Name())))
	if err != nil {
		t.Errorf(err.Error())
	}

	require.Equal(t, http.StatusOK, resp.StatusCode)

	b, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf(err.Error())
	}

	require.Equal(t, "Hello, World!", string(b))

	// Check getting a file that doesn't exist
	resp, err = http.Get(fmt.Sprintf("%s/fileserver/%s", url, "nonexistentfile"))
	if err != nil {
		t.Errorf(err.Error())
	}

	require.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Test removing a rule
	err = proxy.Rules.Remove("fileserver")
	require.Nil(t, err)
	resp, err = http.Get(fmt.Sprintf("%s/fileserver/%s", url, "nonexistentfile"))
	require.Nil(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Equal(t, 1, len(proxy.Rules))

	// Removing it again should fail
	err = proxy.Rules.Remove("fileserver")
	require.NotNil(t, err)

	require.Equal(t, 1, len(proxy.Rules))
}

func TestAddRule(t *testing.T) {
	proxy := &ProxyServer{
		Rules: make(RuleSet),
	}

	err := proxy.Rules.AddRule(&node.ReverseProxyRule{
		ID:      "backend1",
		Type:    "redirect",
		Matcher: "/backend1",
		Target:  "http://localhost:8080",
	})
	require.Nil(t, err)
	require.Equal(t, 1, len(proxy.Rules))
	require.Equal(t, ProxyRuleRedirect, proxy.Rules["backend1"].Type())

	err = proxy.Rules.AddRule(&node.ReverseProxyRule{
		ID:      "backend2",
		Type:    "file",
		Matcher: "/backend2",
		Target:  "http://localhost:8080",
	})
	require.Nil(t, err)
	require.Equal(t, 2, len(proxy.Rules))
	require.Equal(t, ProxyRuleFileServer, proxy.Rules["backend2"].Type())

	// Test unknown rule
	err = proxy.Rules.AddRule(&node.ReverseProxyRule{
		ID:      "backend3",
		Type:    "unknown",
		Matcher: "/backend3",
		Target:  "http://localhost:8080",
	})
	require.NotNil(t, err)
}
