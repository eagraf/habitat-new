package main

import (
	"fmt"
	"net/http"

	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/logging"
	"github.com/eagraf/habitat-new/internal/privi"
	"tailscale.com/tsnet"
)

func main() {
	logger := logging.NewLogger()
	nodeConfig, err := config.NewNodeConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("error loading node config")
	}
	priviServer := privi.NewServer(nil)

	mux := http.NewServeMux()
	mux.HandleFunc("/xrpc/com.habitat.putRecord", priviServer.PutRecord)
	mux.HandleFunc(
		"/xrpc/com.habitat.getRecord",
		priviServer.PdsAuthMiddleware(priviServer.GetRecord),
	)
	mux.HandleFunc("/xrpc/com.habitat.listPermissions", priviServer.ListPermissions)
	mux.HandleFunc("/xrpc/com.habitat.addPermission", priviServer.AddPermission)
	mux.HandleFunc("/xrpc/com.habitat.removePermission", priviServer.RemovePermission)

	mux.HandleFunc("/.well-known/did.json", func(w http.ResponseWriter, r *http.Request) {
		template := `{
  "id": "did:web:%s",
  "@context": [
    "https://www.w3.org/ns/did/v1",
    "https://w3id.org/security/multikey/v1", 
    "https://w3id.org/security/suites/secp256k1-2019/v1"
  ],
  "service": [
    {
      "id": "#privi",
      "serviceEndpoint": "https://%s",
      "type": "PriviServer"
    }
  ]
}`
		domain := nodeConfig.Domain()
		w.Write([]byte(fmt.Sprintf(template, domain, domain)))
	})

	tsnet := &tsnet.Server{
		Hostname: "privi",
		Dir:      nodeConfig.TailScaleStatePath(),
		Logf: func(msg string, args ...any) {
			logger.Debug().Msgf(msg, args...)
		},
		AuthKey: nodeConfig.TailscaleAuthkey(),
	}
	defer tsnet.Close()

	ln, err := tsnet.ListenFunnel("tcp", ":443")
	if err != nil {
		logger.Panic().Err(err).Msg("error creating listener")
	}
	defer ln.Close()

	err = http.Serve(ln, mux)
	if err != nil {
		logger.Fatal().Err(err).Msg("error serving http")
	}
}
