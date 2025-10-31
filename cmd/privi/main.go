package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"

	"github.com/eagraf/habitat-new/internal/permissions"
	"github.com/eagraf/habitat-new/internal/privi"
	"github.com/rs/zerolog/log"
	altsrc "github.com/urfave/cli-altsrc/v3"
	yaml "github.com/urfave/cli-altsrc/v3/yaml"
	"github.com/urfave/cli/v3"
)

const (
	defaultPort = "443"
)

var profileFile string

func main() {
	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "profile",
				Usage:       "The configuration profile to use. ",
				Destination: &profileFile,
			},
			&cli.StringFlag{
				Name:     "domain",
				Required: true,
				Usage:    "The publicly available domain at which the server can be found",
				Sources:  flagSources("domain", "HABITAT_DOMAIN"),
			},
			&cli.StringFlag{
				Name:    "db",
				Usage:   "The path to the sqlite file to use as the backing database for this server",
				Value:   "./repo.db",
				Sources: flagSources("db", "HABITAT_DB"),
			},
			&cli.StringFlag{
				Name:    "port",
				Usage:   "The port on which to run the server. Default 9000",
				Value:   defaultPort,
				Sources: flagSources("port", "HABITAT_PORT"),
			},
			&cli.StringFlag{
				Name:    "certs",
				Usage:   "The directory in which TLS certs can be found. Should contain fullchain.pem and privkey.pem",
				Value:   "/etc/letsencrypt/live/habitat.network/",
				Sources: flagSources("certs", "HABITAT_CERTS"),
			},
		},
		Action: run,
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal().Err(err).Msg("error running command")
	}
}

func run(_ context.Context, cli *cli.Command) error {
	dbPath := cli.String("db")
	// Create database file if it does not exist
	_, err := os.Stat(dbPath)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Println("Privi repo file does not exist; creating...")
		_, err := os.Create(dbPath)
		if err != nil {
			return fmt.Errorf("unable to create privi repo file at %s: %w", dbPath, err)
		}
	} else if err != nil {
		return fmt.Errorf("error finding privi repo file: %w", err)
	}

	priviDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("unable to open sqlite file backing privi server: %w", err)
	}

	repo, err := privi.NewSQLiteRepo(priviDB)
	if err != nil {
		return fmt.Errorf("unable to setup privi repo: %w", err)
	}

	adapter, err := permissions.NewSQLiteStore(priviDB)
	if err != nil {
		return fmt.Errorf("unable to setup permissions store: %w", err)
	}
	priviServer := privi.NewServer(adapter, repo)

	mux := http.NewServeMux()

	loggingMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			x, err := httputil.DumpRequest(r, true)
			if err != nil {
				http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
				return
			}
			fmt.Println("Got a request: ", string(x))
			next.ServeHTTP(w, r)
		})
	}

	mux.HandleFunc("/xrpc/com.habitat.putRecord", priviServer.PutRecord)
	mux.HandleFunc("/xrpc/com.habitat.getRecord", priviServer.GetRecord)
	mux.HandleFunc("/xrpc/network.habitat.uploadBlob", priviServer.UploadBlob)
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
      "id": "#habitat",
      "serviceEndpoint": "https://%s",
      "type": "HabitatServer"
    }
  ]
}`
		domain := cli.String("domain")
		_, err := fmt.Fprintf(w, template, domain, domain)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	port := cli.String("port")
	s := &http.Server{
		Handler: loggingMiddleware(mux),
		Addr:    fmt.Sprintf(":%s", port),
	}

	fmt.Println("Starting server on port :" + port)
	certs := cli.String("certs")
	return s.ListenAndServeTLS(
		fmt.Sprintf("%s%s", certs, "fullchain.pem"),
		fmt.Sprintf("%s%s", certs, "privkey.pem"),
	)
}

func flagSources(name string, envName string) cli.ValueSourceChain {
	return cli.NewValueSourceChain(
		cli.EnvVar(envName),
		yaml.YAML(name, altsrc.NewStringPtrSourcer(&profileFile)),
	)
}
