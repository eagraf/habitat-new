package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/api/test_helpers"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/process"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mock process driver for tests
type entry struct {
	isStart bool
	id      string
}

type mockDriver struct {
	returnErr error
	log       []entry
	name      string
}

var _ process.Driver = &mockDriver{}

func newMockDriver(driver string) *mockDriver {
	return &mockDriver{
		name: driver,
	}
}

func (d *mockDriver) Type() string {
	return d.name
}

func (d *mockDriver) StartProcess(process *node.Process, app *node.AppInstallation) (string, error) {
	if d.returnErr != nil {
		return "", d.returnErr
	}
	id := uuid.New().String()
	d.log = append(d.log, entry{isStart: true, id: id})
	return id, nil
}

func (d *mockDriver) StopProcess(extProcessID string) error {
	d.log = append(d.log, entry{isStart: false, id: extProcessID})
	return nil
}

/*
func TestProcessRestorer(t *testing.T) {
	mockDriver := newMockDriver()
	pm := process.NewProcessManager([]process.Driver{mockDriver})

	ctrl := gomock.NewController(t)
	nc := controller_mocks.NewMockNodeController(ctrl)

	pr := &ProcessRestorer{
		processManager: pm,
		nodeController: nc,
	}

	state := &node.State{
		Users: map[string]*node.User{
			"user1": {
				ID: "user1",
			},
		},
		AppInstallations: map[string]*node.AppInstallationState{
			"app1": {
				AppInstallation: &node.AppInstallation{
					ID:   "app1",
					Name: "appname1",
					Package: node.Package{
						Driver: "test",
					},
				},
			},
			"app2": {
				AppInstallation: &node.AppInstallation{
					ID:   "app2",
					Name: "appname2",
					Package: node.Package{
						Driver: "test",
					},
				},
			},
			"app3": {
				AppInstallation: &node.AppInstallation{
					ID:   "app3",
					Name: "appname3",
					Package: node.Package{
						Driver: "test",
					},
				},
			},

			"app4": {
				AppInstallation: &node.AppInstallation{
					ID:   "app4",
					Name: "appname4",
					Package: node.Package{
						Driver: "test",
					},
				},
			},
		},
		Processes: map[string]*node.ProcessState{
			"proc1": {
				Process: &node.Process{
					ID:     "proc1",
					AppID:  "app1",
					Driver: "test",
				},
				State: node.ProcessStateStarted,
			},
			// This process was not in a running state, but should be started
			"proc2": {
				Process: &node.Process{
					ID:     "proc2",
					AppID:  "app2",
					Driver: "test",
				},
				State: node.ProcessStateStarted,
			},
			// Error out when restoring starting
			"proc3": {
				Process: &node.Process{
					ID:     "proc3",
					AppID:  "app3",
					Driver: "test",
				},
				State: node.ProcessStateStarted,
			},
			// Error out when restoring running
			"proc4": {
				Process: &node.Process{
					ID:    "proc4",
					AppID: "app4",
				},
				State: node.ProcessStateStarted,
			},
		},
	}
	restoreUpdate, err := test_helpers.StateUpdateTestHelper(&node.InitalizationTransition{}, state)
	require.Nil(t, err)

	nc.EXPECT().SetProcessRunning("proc2").Times(1)
	nc.EXPECT().SetProcessRunning("proc3").Times(1)

	err = pr.Restore(restoreUpdate)
	require.Nil(t, err)

	require.Len(t, mockDriver.log, 4)
	for _, entry := range mockDriver.log {
		require.True(t, entry.isStart)
	}

	// Test ListProcesses() and StopProcess()
	procs, err := pm.ListProcesses()
	require.NoError(t, err)
	require.Len(t, procs, 4)

	require.NoError(t, pm.StopProcess("proc2"))
	require.ErrorContains(t, pm.StopProcess("proc4"), "driver  not found")

	procs, err = pm.ListProcesses()
	require.NoError(t, err)
	require.Len(t, procs, 3)

	mockDriver.returnErr = fmt.Errorf("test error")
	err = pm.StartProcess(&node.Process{
		ID:    "proc5",
		AppID: "app5",
	}, &node.AppInstallation{
		ID:   "app5",
		Name: "appname5",
		Package: node.Package{
			Driver: "test",
		},
	})
	require.ErrorContains(t, err, "test error")

	restoreUpdate, err = test_helpers.StateUpdateTestHelper(&node.InitalizationTransition{}, state)
	require.NoError(t, err)
	err = pr.Restore(restoreUpdate)
	require.ErrorContains(t, err, "test error")
}
*/

// mock hdb for tests
type mockHDB struct {
	schema    hdb.Schema
	jsonState *hdb.JSONState
}

func (db *mockHDB) Bytes() []byte {
	return db.jsonState.Bytes()
}
func (db *mockHDB) DatabaseID() string {
	return "test"
}
func (db *mockHDB) ProposeTransitions(transitions []hdb.Transition) (*hdb.JSONState, error) {
	// Blindly apply all transitions
	var state hdb.SerializedState = db.jsonState.Bytes()
	for _, t := range transitions {
		patch, err := t.Patch(state)
		if err != nil {
			return nil, err
		}
		fmt.Println("patch", string(patch))
		err = db.jsonState.ApplyPatch(patch)
		if err != nil {
			return nil, err
		}
	}
	fmt.Println("new state", string(db.jsonState.Bytes()))
	return db.jsonState, nil
}
func (db *mockHDB) ProposeTransitionsEnriched(ts []hdb.Transition) (*hdb.JSONState, error) {
	return db.ProposeTransitions(ts)
}

var (
	testPkg = node.Package{
		Driver:       "docker",
		DriverConfig: map[string]interface{}{},
	}
	proc1 = &node.ProcessState{
		Process: &node.Process{
			ID:     "proc1",
			AppID:  "app1",
			Driver: "docker",
		},
		State: node.ProcessStateStarted,
	}
	proc2 = &node.ProcessState{
		Process: &node.Process{
			ID:     "proc2",
			AppID:  "app2",
			Driver: "docker",
		},
		State: node.ProcessStateStarted,
	}
	state = &node.State{
		SchemaVersion: "v0.0.6",
		Users: map[string]*node.User{
			"user1": {
				ID: "user1",
			},
		},
		AppInstallations: map[string]*node.AppInstallationState{
			"app1": {
				AppInstallation: &node.AppInstallation{
					ID:      "app1",
					Name:    "appname1",
					Package: testPkg,
				},
				State: node.AppLifecycleStateInstalled,
			},
			"app2": {
				AppInstallation: &node.AppInstallation{
					ID:      "app2",
					Name:    "appname2",
					Package: testPkg,
				},
				State: node.AppLifecycleStateInstalled,
			},
		},
		Processes: map[string]*node.ProcessState{
			"proc1": proc1,
			// This process was not in a running state, but should be started
			"proc2": proc2,
		},
	}
)

func TestStartProcessHandler(t *testing.T) {
	middleware := &test_helpers.TestAuthMiddleware{UserID: "user_1"}

	mockDriver := newMockDriver("docker")
	s, err := NewCtrlServer(process.NewProcessManager([]process.Driver{mockDriver}), &mockHDB{
		schema:    state.Schema(),
		jsonState: jsonStateFromNodeState(state),
	})
	require.NoError(t, err)

	startProcessHandler := http.HandlerFunc(s.StartProcess)
	startProcessRoute := newRoute(http.MethodPost, "/node/processes", startProcessHandler)
	handler := middleware.Middleware(startProcessHandler)

	b, err := json.Marshal(PostProcessRequest{
		AppInstallationID: "app1",
	})
	if err != nil {
		t.Error(err)
	}

	// Test the happy path
	resp := httptest.NewRecorder()
	handler.ServeHTTP(
		resp,
		httptest.NewRequest(http.MethodPost, startProcessRoute.Pattern(), bytes.NewReader(b)),
	)
	fmt.Println(string(resp.Body.Bytes()))
	require.Equal(t, http.StatusCreated, resp.Result().StatusCode)

	respBody, err := io.ReadAll(resp.Result().Body)
	require.NoError(t, err)
	require.Equal(t, 0, len(respBody))

	// Test an error returned by the controller
	b, err = json.Marshal(PostProcessRequest{
		AppInstallationID: "app3", // non-existent app installation
	})
	if err != nil {
		t.Error(err)
	}
	resp = httptest.NewRecorder()
	handler.ServeHTTP(
		resp,
		httptest.NewRequest(http.MethodPost, startProcessRoute.Pattern(), bytes.NewReader(b)),
	)
	require.Equal(t, http.StatusInternalServerError, resp.Result().StatusCode)

	// Test invalid request
	resp = httptest.NewRecorder()
	handler.ServeHTTP(
		resp,
		httptest.NewRequest(
			http.MethodPost,
			startProcessRoute.Pattern(),
			bytes.NewReader([]byte("invalid")),
		),
	)
	assert.Equal(t, http.StatusBadRequest, resp.Result().StatusCode)
}

// Kind of annoying helper to do some typing
// In the long term, types across packages should be more aligned so we don't have to do this
// For example, hdb.NewJSONState() could take in node.State, but right now that causes an import cycle
// Which leads me to believer that maybe NewJSONState() shouldn't be in the hdb package but somewhere else
// For now, just work with it
func jsonStateFromNodeState(s *node.State) *hdb.JSONState {
	bytes, err := s.Bytes()
	if err != nil {
		panic(err)
	}
	state, err := hdb.NewJSONState(state.Schema(), bytes)
	if err != nil {
		panic(err)
	}
	return state
}

func TestControllerRestoreProcess(t *testing.T) {
	mockDriver := newMockDriver("docker")
	pm := process.NewProcessManager([]process.Driver{mockDriver})

	// newController2 calls restore() on the initial state
	ctrl, err := newController2(pm, &mockHDB{
		schema:    state.Schema(),
		jsonState: jsonStateFromNodeState(state),
	})
	require.NoError(t, err)

	procs, err := ctrl.processManager.ListProcesses()
	require.NoError(t, err)

	// Sort by procID so we can assert on the states
	require.Len(t, procs, 2)
	slices.SortFunc(procs, func(a, b *node.ProcessState) int {
		return strings.Compare(a.ID, b.ID)
	})

	// Ensure processManager has expected state
	require.Equal(t, procs[0].ID, proc1.ID)
	require.Equal(t, procs[0].AppID, proc1.AppID)
	require.Equal(t, procs[0].Driver, "docker")
	require.Equal(t, procs[0].State, node.ProcessStateStarted)

	require.Equal(t, procs[1].ID, proc2.ID)
	require.Equal(t, procs[1].AppID, proc2.AppID)
	require.Equal(t, procs[1].Driver, "docker")
	require.Equal(t, procs[1].State, node.ProcessStateStarted)
}
