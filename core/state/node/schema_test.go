package node

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qri-io/jsonschema"
	"github.com/stretchr/testify/assert"
)

func TestSchemaParsing(t *testing.T) {
	rs := &jsonschema.Schema{}
	err := json.Unmarshal([]byte(nodeSchemaRaw), rs)
	assert.Nil(t, err)

	// Test that an empty state from InitState() is valid
	ns := &NodeSchema{}
	initState, err := ns.InitState()
	assert.Nil(t, err)
	assert.NotNil(t, ns.Type())
	assert.Equal(t, "node", ns.Name())

	marshaled, err := json.Marshal(initState)
	assert.Nil(t, err)
	keyErrs, err := rs.ValidateBytes(context.Background(), marshaled)
	assert.Nil(t, err)
	if len(keyErrs) != 0 {
		for _, e := range keyErrs {
			t.Log(e)
		}
		t.Error()
	}
}

func TestInitializationTransition(t *testing.T) {
	ns := &NodeSchema{}
	initState, err := ns.InitState()
	assert.Nil(t, err)
	initStateBytes, err := json.Marshal(initState)
	assert.Nil(t, err)

	// Test that the initialization transition works
	it, err := ns.InitializationTransition(initStateBytes)
	assert.Nil(t, err)

	// Test that the transition is valid
	err = it.Validate([]byte{})
	assert.Nil(t, err)

	// Test that the patch is valid
	_, err = it.Patch([]byte{})
	assert.Nil(t, err)
}

func TestGetAppByID(t *testing.T) {
	state := &NodeState{
		AppInstallations: map[string]*AppInstallationState{
			"app1": {
				AppInstallation: &AppInstallation{
					ID: "app1",
				},
			},
		},
	}

	app, err := state.GetAppByID("app1")
	assert.Nil(t, err)
	assert.Equal(t, "app1", app.ID)

	_, err = state.GetAppByID("app2")
	assert.NotNil(t, err)
}
