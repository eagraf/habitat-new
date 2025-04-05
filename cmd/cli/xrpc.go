package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/bluesky-social/indigo/did"
	"github.com/bluesky-social/indigo/plc"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/urfave/cli/v2"
)

func xrpcCmd() *cli.Command {
	var didStr string
	var hostStr string
	var kindStr string
	var method string
	var inputStr string
	var mimetype string
	return &cli.Command{
		Name: "xrpc",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "method",
				Usage:       "XRPC method to call",
				Destination: &method,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "did",
				Usage:       "DID of the repo",
				Destination: &didStr,
				Required:    false,
			},
			&cli.StringFlag{
				Name:        "host",
				Usage:       "host of the repo",
				Destination: &hostStr,
				Required:    false,
			},
			&cli.StringFlag{
				Name:        "kind",
				Usage:       "kind of request to make (query or procedure)",
				Destination: &kindStr,
				Required:    false,
				DefaultText: "query",
			},
			&cli.StringFlag{
				Name:        "input",
				Usage:       "json to pass to the method",
				Destination: &inputStr,
				Required:    false,
			},
			&cli.StringFlag{
				Name:        "mimetype",
				Usage:       "mimetype of the input",
				Destination: &mimetype,
				Required:    false,
				DefaultText: "application/json",
			},
		},
		Action: func(ctx *cli.Context) error {
			kind, err := getKind(kindStr)
			if err != nil {
				return err
			}

			if method == "" {
				return fmt.Errorf("method is required")
			}

			params, obj, err := getInput(kind, inputStr)
			if err != nil {
				return err
			}

			host, err := getHost(ctx.Context, didStr, hostStr)
			if err != nil {
				return err
			}

			authInfo := &xrpc.AuthInfo{
				AccessJwt: os.Getenv("ACCESS_JWT"),
			}

			client := &xrpc.Client{
				Host:   host,
				Client: http.DefaultClient,
				Auth:   authInfo,
			}

			out := map[string]any{}
			err = client.Do(ctx.Context, kind, mimetype, method, params, obj, &out)
			if err != nil {
				return err
			}

			return json.NewEncoder(os.Stdout).Encode(out)
		},
	}
}

func getHost(ctx context.Context, didstr string, host string) (string, error) {
	if host != "" {
		return host, nil
	}
	mr := did.NewMultiResolver()
	mr.AddHandler("plc", &plc.PLCServer{
		Host: "https://plc.directory",
	})
	mr.AddHandler("web", &did.WebResolver{})

	doc, err := mr.GetDocument(ctx, didstr)
	if err != nil {
		return "", err
	}
	for _, service := range doc.Service {
		if service.Type == "AtprotoPersonalDataServer" {
			return service.ServiceEndpoint, nil
		}
	}

	return "", fmt.Errorf("TODO")
}

func getKind(kindStr string) (xrpc.XRPCRequestType, error) {
	switch kindStr {
	case "query":
		return xrpc.Query, nil
	case "procedure":
		return xrpc.Procedure, nil
	}
	return 0, fmt.Errorf("unknown kind: %s", kindStr)
}

func getInput(
	kind xrpc.XRPCRequestType,
	paramsStr string,
) (map[string]any, interface{}, error) {
	if paramsStr == "" {
		return nil, nil, nil
	}
	var params map[string]any
	var obj interface{}
	if kind == xrpc.Query {
		if err := json.Unmarshal([]byte(paramsStr), &params); err != nil {
			return nil, nil, err
		}
	} else {
		if err := json.Unmarshal([]byte(paramsStr), &obj); err != nil {
			return nil, nil, err
		}
	}

	return params, obj, nil
}
