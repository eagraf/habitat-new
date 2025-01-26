package process_test

import (
	"context"
	"testing"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/process"
	"github.com/stretchr/testify/require"
)

func TestNoopDriver(t *testing.T) {
	driver := process.NewNoopDriver(node.DriverNoop)
	require.Equal(t, node.DriverNoop, driver.Type())
	err := driver.StartProcess(context.Background(), "my-id", nil)
	require.NoError(t, err)
	err = driver.StopProcess(context.Background(), "my-id")
	require.NoError(t, err)
}
