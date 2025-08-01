package main

import (
	"context"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	outgoinghandler "github.com/eagraf/habitat-new/wasi/http/outgoing-handler"
)

func main() {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	_, err := rt.NewHostModuleBuilder("outgoing-handler").
		NewFunctionBuilder().
		WithFunc(func(x outgoinghandler.OutgoingRequest, y uint32) {
			log.Info().Msgf("x: %v, y: %v", x, y)
		}).
		Export("handle").Instantiate(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to instantiate outgoing-handler module")
	}

	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	// Load the test.wasm app from .habitat/apps/
	testWasmBytes, err := os.ReadFile("test.wasm")
	if err != nil {
		log.Error().Err(err).Msg("failed to read index.wasm file")
		panic(err)
	}

	mod, err := rt.InstantiateWithConfig(ctx, testWasmBytes, wazero.NewModuleConfig())
	if err != nil {
		log.Error().Err(err).Msg("failed to instantiate test.wasm module")
	}

	log.Info().Str("name", mod.Name()).Msg("module name") // mod.Name()

	// resp, err := mod.ExportedFunction("handle").Call(ctx, 1, 2)
	// if err != nil {
	// 	log.Error().Err(err).Msg("failed to call add function")
	// }
	// log.Info().Uints64("resp", resp).Msg("response from wasm")
}
