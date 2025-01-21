package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"golang.org/x/exp/maps"

	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/urfave/cli/v2"
)

var port string

func printResponse(res *http.Response) error {
	slurp, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	fmt.Printf("Status: %s\nResponse: %s\n", res.Status, string(slurp))
	return nil
}

func startProcess() *cli.Command {
	var req controller.StartProcessRequest
	return &cli.Command{
		Name:  "start-process",
		Usage: "Start a new process for a given app installation.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "app",
				Usage:       "The name of the desired app for which to start the process.",
				Destination: &req.AppInstallationID,
				Required:    true,
			},
		},
		Action: func(ctx *cli.Context) error {
			url := fmt.Sprintf("http://localhost:%s/node/processes/start", port)
			marshalled, err := json.Marshal(req)
			if err != nil {
				return err
			}
			res, err := http.Post(url, "application/json", bytes.NewReader(marshalled))
			if err != nil {
				return err
			}
			return printResponse(res)
		},
	}
}

func stopProcess() *cli.Command {
	var req controller.StopProcessRequest
	return &cli.Command{
		Name:  "stop-process",
		Usage: "Stop the process with the given ID.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "id",
				Usage:       "The process ID for the desired process to stop.",
				Destination: &req.ProcessID,
				Required:    true,
			},
		},
		Action: func(ctx *cli.Context) error {
			url := fmt.Sprintf("http://localhost:%s/node/processes/stop", port)
			marshalled, err := json.Marshal(req)
			if err != nil {
				return err
			}
			res, err := http.Post(url, "application/json", bytes.NewReader(marshalled))
			if err != nil {
				return err
			}
			return printResponse(res)
		},
	}
}

func listProcesses() *cli.Command {
	return &cli.Command{
		Name:  "list-processes",
		Usage: "List all running processes",
		Action: func(ctx *cli.Context) error {
			url := fmt.Sprintf("http://localhost:%s/node/processes/list", port)
			res, err := http.Get(url)
			if err != nil {
				return err
			}
			return printResponse(res)
		},
	}
}

func main() {
	commands := map[string]*cli.Command{
		"start-process":  startProcess(),
		"stop-process":   stopProcess(),
		"list-processes": listProcesses(),
	}

	for name, command := range commands {
		if command.Name != name {
			panic(fmt.Sprintf("command %s's name didn't match", name))
		}
	}

	app := &cli.App{
		Name:  "node_ctl",
		Usage: "CLI interface for interacting with the Node Control server",
		CommandNotFound: func(ctx *cli.Context, s string) {
			fmt.Println("command not found: ", s)
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "port",
				Aliases:     []string{"p"},
				Usage:       "Ctrl Server port to connect to",
				Value:       constants.DefaultPortHabitatAPI,
				Destination: &port,
			},
		},
		Commands: maps.Values(commands),
	}

	err := app.Run(os.Args)
	if err != nil {
		os.Stderr.WriteString(err.Error())
		os.Exit(1)
	}
}
