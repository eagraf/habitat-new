package main

import (
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name: "Habitat Dev CLI",
		Commands: []*cli.Command{
			xrpcCmd(),
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		os.Stderr.WriteString(err.Error())
		os.Exit(1)
	}
}
