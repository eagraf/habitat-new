package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	node_state "github.com/eagraf/habitat-new/internal/node/state"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/eagraf/habitat-new/internal/package_manager"
	"github.com/stretchr/testify/require"
)

func fakeInitState(rootUserID, rootUsername string) *node_state.NodeState {
	initState, err := node_state.GetEmptyStateForVersion(node_state.LatestVersion)
	if err != nil {
		panic(err)
	}

	initState.Users[rootUserID] = &node_state.User{
		ID:       rootUserID,
		Username: rootUsername,
	}

	return initState
}

func TestAddUser(t *testing.T) {
	mockedClient := &mockHDB{
		state: fakeInitState("fake_user_id", "fake_username"),
	}

	did := "did"
	mockPDS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/xrpc/com.atproto.server.createAccount", r.URL.String())
		bytes, err := json.Marshal(
			atproto.ServerCreateAccount_Output{
				Did:    did,
				Handle: "user-handle",
			},
		)
		require.NoError(t, err)
		_, err = w.Write(bytes)
		require.NoError(t, err)
	}))
	defer mockPDS.Close()

	ctrl2, err := NewController(context.Background(), fakeProcessManager(), nil, mockedClient, nil, mockPDS.URL)
	require.NoError(t, err)

	email := "user@user.com"
	pass := "pass"
	input := &atproto.ServerCreateAccount_Input{
		Did:      &did,
		Email:    &email,
		Handle:   "user-handle",
		Password: &pass,
	}

	out, err := ctrl2.addUser(context.Background(), input)
	require.Nil(t, err)
	require.Equal(t, out.Did, did)
}

func TestMigrations(t *testing.T) {
	fakestate := &node_state.NodeState{
		SchemaVersion:     "v0.0.1",
		Users:             map[string]*node_state.User{},
		AppInstallations:  map[string]*node_state.AppInstallation{},
		Processes:         map[node_state.ProcessID]*node_state.Process{},
		ReverseProxyRules: map[string]*node_state.ReverseProxyRule{},
	}

	db := &mockHDB{
		state: fakestate,
	}
	ctrl2, err := NewController(context.Background(), fakeProcessManager(), nil, db, nil, "fak-pds")
	require.NoError(t, err)
	s, err := NewCtrlServer(context.Background(), ctrl2, fakestate)
	require.NoError(t, err)
	handler := http.HandlerFunc(s.MigrateDB)

	b, err := json.Marshal(MigrateRequest{
		TargetVersion: "v0.0.2",
	})
	require.NoError(t, err)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(
		resp,
		httptest.NewRequest(http.MethodPost, "/doesntmatter", bytes.NewReader(b)),
	)
	require.Equal(t, http.StatusOK, resp.Result().StatusCode)
}

func TestGetNodeState(t *testing.T) {
	ctrl2, err := NewController(context.Background(), fakeProcessManager(),
		map[node_state.DriverType]package_manager.PackageManager{
			node_state.DriverTypeDocker: &mockPkgManager{
				installs: make(map[*node_state.Package]struct{}),
			},
		},
		&mockHDB{
			state: state,
		}, nil, "fake-pds")
	require.NoError(t, err)
	ctrlServer, err := NewCtrlServer(
		context.Background(),
		ctrl2,
		state,
	)
	require.NoError(t, err)

	handler := http.HandlerFunc(ctrlServer.GetNodeState)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest("get", "/test", nil))
	bytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	sb, err := state.Bytes()
	require.NoError(t, err)
	require.Equal(t, bytes, sb)
}
