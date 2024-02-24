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

	"github.com/stretchr/testify/assert"
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
	proxy := &ProxyServer{
		Rules: make(RuleSet),
	}
	err = proxy.Rules.Add("backend1", &RedirectRule{
		Matcher:         "/backend1",
		ForwardLocation: redirectedServerURL,
	})
	assert.Nil(t, err)

	// Test adding naming conflict
	err = proxy.Rules.Add("backend1", &RedirectRule{
		Matcher:         "/backend1",
		ForwardLocation: redirectedServerURL,
	})
	assert.NotNil(t, err)

	err = proxy.Rules.Add("fileserver", &FileServerRule{
		Matcher: "/fileserver",
		Path:    fileDir,
	})
	assert.Nil(t, err)
	assert.Equal(t, 2, len(proxy.Rules))

	frontendServer := httptest.NewServer(proxy)
	defer frontendServer.Close()

	// Check redirection
	resp, err := http.Get(frontendServer.URL + "/backend1")
	if err != nil {
		t.Errorf(err.Error())
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf(err.Error())
	}

	assert.Equal(t, "Hello, World!", string(b))

	// Check file serving
	resp, err = http.Get(fmt.Sprintf("%s/fileserver/%s", frontendServer.URL, filepath.Base(file.Name())))
	if err != nil {
		t.Errorf(err.Error())
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	b, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf(err.Error())
	}

	assert.Equal(t, "Hello, World!", string(b))

	// Check getting a file that doesn't exist
	resp, err = http.Get(fmt.Sprintf("%s/fileserver/%s", frontendServer.URL, "nonexistentfile"))
	if err != nil {
		t.Errorf(err.Error())
	}

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Test removing a rule
	err = proxy.Rules.Remove("fileserver")
	assert.Nil(t, err)

	assert.Equal(t, 1, len(proxy.Rules))

	// Removing it again should fail
	err = proxy.Rules.Remove("fileserver")
	assert.NotNil(t, err)

	assert.Equal(t, 1, len(proxy.Rules))
}
