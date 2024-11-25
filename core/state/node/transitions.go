package node

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/google/uuid"
	"github.com/wI2L/jsondiff"
)

var (
	TransitionInitialize          = "initialize"
	TransitionMigrationUp         = "migration_up"
	TransitionAddUser             = "add_user"
	TransitionStartInstallation   = "start_installation"
	TransitionFinishInstallation  = "finish_installation"
	TransitionStartUninstallation = "start_uninstallation"
	TransitionStartProcess        = "process_start"
	TransitionProcessRunning      = "process_running"
	TransitionStopProcess         = "process_stop"
	TransitionAddReverseProxyRule = "add_reverse_proxy_rule"
	TransitionStartAppUpgrade     = "start_app_upgrade"
	TransitionFinishAppUpgrade    = "finish_app_upgrade"
)

type InitalizationTransition struct {
	InitState *State `json:"init_state"`
}

func (t *InitalizationTransition) Type() string {
	return TransitionInitialize
}

func (t *InitalizationTransition) Patch(oldState hdb.SerializedState) (jsondiff.Patch, error) {
	if t.InitState.Users == nil {
		t.InitState.Users = make(map[string]*User, 0)
	}

	if t.InitState.AppInstallations == nil {
		t.InitState.AppInstallations = make(map[string]*AppInstallationState)
	}

	if t.InitState.Processes == nil {
		t.InitState.Processes = make(map[string]*ProcessState)
	}

	return jsondiff.Patch{
		{
			Path:  "",
			Type:  jsondiff.OperationAdd,
			Value: t.InitState,
		},
	}, nil
}

func (t *InitalizationTransition) Enrich(oldState hdb.SerializedState) error {
	return nil
}

func (t *InitalizationTransition) Validate(oldState hdb.SerializedState) error {
	if t.InitState == nil {
		return fmt.Errorf("init state cannot be nil")
	}
	return nil
}

type MigrationTransition struct {
	TargetVersion string
}

func (t *MigrationTransition) Type() string {
	return TransitionMigrationUp
}

func (t *MigrationTransition) Patch(oldState hdb.SerializedState) (jsondiff.Patch, error) {
	var oldNode State
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return nil, err
	}

	patch, err := NodeDataMigrations.GetMigrationPatch(oldNode.SchemaVersion, t.TargetVersion, &oldNode)
	if err != nil {
		return nil, err
	}

	return patch, nil
}

func (t *MigrationTransition) Enrich(oldState hdb.SerializedState) error {
	return nil
}

func (t *MigrationTransition) Validate(oldState hdb.SerializedState) error {
	var oldNode State
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	patch, err := NodeDataMigrations.GetMigrationPatch(oldNode.SchemaVersion, t.TargetVersion, &oldNode)
	if err != nil {
		return err
	}

	newState, err := applyPatchToState(patch, &oldNode)
	if err != nil {
		return err
	}

	err = newState.Validate()
	if err != nil {
		return err
	}

	return nil
}

type AddUserTransition struct {
	Username    string `json:"username"`
	Certificate string `json:"certificate"`
	AtprotoDID  string `json:"atproto_did"`

	EnrichedData *AddUserTranstitionEnrichedData `json:"enriched_data"`
}

type AddUserTranstitionEnrichedData struct {
	User *User `json:"user"`
}

func (t *AddUserTransition) Type() string {
	return TransitionAddUser
}

func (t *AddUserTransition) Patch(oldState hdb.SerializedState) (jsondiff.Patch, error) {
	return jsondiff.Patch{
		{
			Path:  fmt.Sprintf("/users/%s", t.EnrichedData.User.ID),
			Type:  jsondiff.OperationAdd,
			Value: t.EnrichedData.User,
		},
	}, nil
}

func (t *AddUserTransition) Enrich(oldState hdb.SerializedState) error {

	id := uuid.New().String()

	t.EnrichedData = &AddUserTranstitionEnrichedData{
		User: &User{
			ID:          id,
			Username:    t.Username,
			Certificate: t.Certificate,
			AtprotoDID:  t.AtprotoDID,
		},
	}
	return nil
}

func (t *AddUserTransition) Validate(oldState hdb.SerializedState) error {

	var oldNode State
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	_, ok := oldNode.Users[t.EnrichedData.User.ID]
	if ok {
		return fmt.Errorf("user with id %s already exists", t.EnrichedData.User.ID)
	}

	// Check for conflicting usernames
	for _, user := range oldNode.Users {
		if user.Username == t.Username {
			return fmt.Errorf("user with username %s already exists", t.Username)
		}
	}
	return nil
}

type StartInstallationTransition struct {
	UserID                 string `json:"user_id"`
	StartAfterInstallation bool   `json:"start_after_installation"`

	*AppInstallation
	NewProxyRules []*ReverseProxyRule                      `json:"new_proxy_rules"`
	EnrichedData  *StartInstallationTransitionEnrichedData `json:"enriched_data"`
}

type StartInstallationTransitionEnrichedData struct {
	AppState      *AppInstallationState `json:"app_state"`
	NewProxyRules []*ReverseProxyRule   `json:"new_proxy_rules"`
}

func (t *StartInstallationTransition) Type() string {
	return TransitionStartInstallation
}

func (t *StartInstallationTransition) Patch(oldState hdb.SerializedState) (jsondiff.Patch, error) {
	var oldNode State
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return nil, err
	}

	_, ok := oldNode.Users[t.UserID]
	if !ok {
		return nil, fmt.Errorf("user with id %s not found", t.UserID)
	}

	ruleOps := make([]jsondiff.Operation, 0)
	for _, rule := range t.EnrichedData.NewProxyRules {
		ruleOps = append(ruleOps, jsondiff.Operation{
			Type:  jsondiff.OperationAdd,
			Path:  fmt.Sprintf("/reverse_proxy_rules/%s", rule.ID),
			Value: rule,
		})
	}

	res := make([]jsondiff.Operation, 0)
	res = append(res, jsondiff.Operation{
		Type:  jsondiff.OperationAdd,
		Path:  fmt.Sprintf("/app_installations/%s", t.EnrichedData.AppState.ID),
		Value: t.EnrichedData.AppState,
	})
	res = append(res, ruleOps...)

	return res, nil
}

func (t *StartInstallationTransition) Enrich(oldState hdb.SerializedState) error {
	id := uuid.New().String()
	appInstallState := &AppInstallationState{
		AppInstallation: &AppInstallation{
			ID:      id,
			UserID:  t.UserID,
			Name:    t.Name,
			Version: t.Version,
			Package: t.Package,
		},
		State: AppLifecycleStateInstalling,
	}

	enrichedRules := make([]*ReverseProxyRule, 0)

	for _, rule := range t.NewProxyRules {
		enrichedRules = append(enrichedRules, &ReverseProxyRule{
			ID:      uuid.New().String(),
			AppID:   appInstallState.ID,
			Type:    rule.Type,
			Matcher: rule.Matcher,
			Target:  rule.Target,
		})
		rule.ID = uuid.New().String()
		rule.AppID = appInstallState.ID
	}

	t.EnrichedData = &StartInstallationTransitionEnrichedData{
		AppState:      appInstallState,
		NewProxyRules: enrichedRules,
	}
	return nil
}

func (t *StartInstallationTransition) Validate(oldState hdb.SerializedState) error {
	var oldNode State
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	_, ok := oldNode.Users[t.UserID]
	if !ok {
		return fmt.Errorf("user with id %s not found", t.UserID)
	}

	app, ok := oldNode.AppInstallations[t.EnrichedData.AppState.ID]
	if ok {
		if app.Version == t.Version {
			return fmt.Errorf("app %s version %s for user %s found in state %s", t.Name, t.Version, t.UserID, app.State)
		} else {
			// TODO eventually this will be part of an upgrade flow
			return fmt.Errorf("app %s for user %s found in state with different version %s", t.Name, t.UserID, app.Version)
		}
	}

	// Look for matching registry URL and package ID
	// TODO @eagraf - we need a way to update apps
	for _, app := range oldNode.AppInstallations {
		if app.RegistryURLBase == t.RegistryURLBase && app.RegistryPackageID == t.RegistryPackageID {
			return fmt.Errorf("app %s for user %s found in state with different version %s", app.Name, t.UserID, app.Version)
		}
	}

	if t.AppInstallation.DriverConfig == nil {
		return fmt.Errorf("driver config is required for starting an installation")
	}

	return nil
}

type FinishInstallationTransition struct {
	UserID          string `json:"user_id"`
	AppID           string `json:"app_id"`
	RegistryURLBase string `json:"registry_url_base"`
	RegistryAppID   string `json:"registry_app_id"`

	StartAfterInstallation bool `json:"start_after_installation"`
}

func (t *FinishInstallationTransition) Type() string {
	return TransitionFinishInstallation
}

func (t *FinishInstallationTransition) Patch(oldState hdb.SerializedState) (jsondiff.Patch, error) {
	return jsondiff.Patch{
		{
			Type:  jsondiff.OperationReplace,
			Path:  fmt.Sprintf("/app_installations/%s/state", t.AppID),
			Value: AppLifecycleStateInstalled,
		},
	}, nil
}

func (t *FinishInstallationTransition) Enrich(oldState hdb.SerializedState) error {
	return nil
}

func (t *FinishInstallationTransition) Validate(oldState hdb.SerializedState) error {
	var oldNode State
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	app, ok := oldNode.AppInstallations[t.AppID]
	if !ok {
		return fmt.Errorf("app with id %s not found", t.AppID)
	}

	_, ok = oldNode.Users[t.UserID]
	if !ok {
		return fmt.Errorf("user with id %s not found", t.UserID)
	}

	if app.RegistryURLBase != t.RegistryURLBase || app.RegistryPackageID != t.RegistryAppID {
		return fmt.Errorf("app %s for user %s found in state with different registry url %s and package id %s", app.Name, t.UserID, app.RegistryURLBase, app.RegistryPackageID)
	}

	if app.State != "installing" {
		return fmt.Errorf("app %s for user %s is in state %s", app.Name, t.UserID, app.State)
	}

	return nil
}

// Transition for updating app installations

type StartAppUpgradeTransition struct {
	AppID              string                `json:"app_id"`
	NewAppInstallation *AppInstallationState `json:"new_app_installation"`
	NewProxyRules      []*ReverseProxyRule   `json:"new_proxy_rules"`
	StartAfterUpgrade  bool                  `json:"start_after_upgrade"`
}

func (t *StartAppUpgradeTransition) Type() string {
	return TransitionStartAppUpgrade
}

func (t *StartAppUpgradeTransition) Patch(oldState hdb.SerializedState) (jsondiff.Patch, error) {
	// Copy over the ID from the existing app
	t.NewAppInstallation.ID = t.AppID
	t.NewAppInstallation.State = AppLifecycleStateUpgrading

	ruleOps := make([]jsondiff.Operation, 0)
	for _, rule := range t.NewProxyRules {
		ruleOps = append(ruleOps, jsondiff.Operation{
			Type:  jsondiff.OperationAdd,
			Path:  fmt.Sprintf("/reverse_proxy_rules/%s", rule.ID),
			Value: rule,
		})
	}

	res := make([]jsondiff.Operation, 0)
	res = append(res, jsondiff.Operation{
		Type:  jsondiff.OperationReplace,
		Path:  fmt.Sprintf("/app_installations/%s", t.AppID),
		Value: t.NewAppInstallation,
	})
	res = append(res, ruleOps...)

	return res, nil
}

func (t *StartAppUpgradeTransition) Enrich(oldState hdb.SerializedState) error {
	return nil
}

func (t *StartAppUpgradeTransition) Validate(oldState hdb.SerializedState) error {
	var oldNode State
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	// Check that app exists and is in installed state
	existingApp, ok := oldNode.AppInstallations[t.AppID]
	if !ok {
		return fmt.Errorf("app with id %s not found", t.AppID)
	}

	if existingApp.State != AppLifecycleStateInstalled {
		return fmt.Errorf("app %s is in state %s, must be in installed state", existingApp.Name, existingApp.State)
	}

	// Check that no process is running for this app
	for _, process := range oldNode.Processes {
		if process.AppID == t.AppID {
			return fmt.Errorf("process %s is still running for app %s", process.ID, t.AppID)
		}
	}

	// Validate version is higher
	if t.NewAppInstallation.Version <= existingApp.Version {
		return fmt.Errorf("new version %s must be higher than existing version %s", t.NewAppInstallation.Version, existingApp.Version)
	}

	return nil
}

type FinishAppUpgradeTransition struct {
	AppID             string `json:"app_id"`
	StartAfterUpgrade bool   `json:"start_after_upgrade"`
}

func (t *FinishAppUpgradeTransition) Type() string {
	return TransitionFinishAppUpgrade
}

func (t *FinishAppUpgradeTransition) Patch(oldState hdb.SerializedState) (jsondiff.Patch, error) {
	return jsondiff.Patch{
		{
			Type:  jsondiff.OperationReplace,
			Path:  fmt.Sprintf("/app_installations/%s/state", t.AppID),
			Value: AppLifecycleStateInstalled,
		},
	}, nil
}

func (t *FinishAppUpgradeTransition) Enrich(oldState hdb.SerializedState) error {
	return nil
}

func (t *FinishAppUpgradeTransition) Validate(oldState hdb.SerializedState) error {
	var oldNode State
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	app, ok := oldNode.AppInstallations[t.AppID]
	if !ok {
		return fmt.Errorf("app with id %s not found", t.AppID)
	}

	if app.State != "upgrading" {
		return fmt.Errorf("app %s is in state %s, must be in upgrading state", app.Name, app.State)
	}

	return nil
}

// TODO handle uninstallation

type ProcessStartTransition struct {
	// Requested data
	AppID string

	EnrichedData *ProcessStartTransitionEnrichedData
}

type ProcessStartTransitionEnrichedData struct {
	Process *ProcessState
	App     *AppInstallationState
}

func (t *ProcessStartTransition) Type() string {
	return TransitionStartProcess
}

func (t *ProcessStartTransition) Patch(oldState hdb.SerializedState) (jsondiff.Patch, error) {
	return jsondiff.Patch{
		{
			Type:  jsondiff.OperationAdd,
			Path:  fmt.Sprintf("/processes/%s", t.EnrichedData.Process.ID),
			Value: t.EnrichedData.Process,
		},
	}, nil
}

func (t *ProcessStartTransition) Enrich(oldState hdb.SerializedState) error {
	var oldNode State
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	app, err := oldNode.GetAppByID(t.AppID)
	if err != nil {
		return err
	}

	proc := &ProcessState{
		Process: &Process{
			UserID: app.UserID,
			Driver: app.Driver,
			AppID:  app.ID,
		},
		State:       ProcessStateStarting,
		ExtDriverID: "", // this should not be set yet
	}

	proc.ID = uuid.New().String()
	proc.Process.Created = time.Now().Format(time.RFC3339)

	t.EnrichedData = &ProcessStartTransitionEnrichedData{
		Process: proc,
		App:     app,
	}
	return nil
}

func (t *ProcessStartTransition) Validate(oldState hdb.SerializedState) error {

	var oldNode State
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	if t.EnrichedData.Process == nil {
		return fmt.Errorf("process was not properly enriched")
	}

	if t.EnrichedData.App == nil {
		return fmt.Errorf("app was not properly enriched")
	}

	// Make sure the app installation is in the installed state
	userID := t.EnrichedData.Process.UserID
	if t.EnrichedData.App.State != AppLifecycleStateInstalled {
		return fmt.Errorf("app with id %s for user %s is not in state %s", t.AppID, userID, AppLifecycleStateInstalled)
	}

	// Check user exists
	_, ok := oldNode.Users[t.EnrichedData.Process.UserID]
	if !ok {
		return fmt.Errorf("user with id %s does not exist", userID)
	}
	if _, ok := oldNode.Processes[t.EnrichedData.Process.ID]; ok {
		return fmt.Errorf("Process with id %s already exists", t.EnrichedData.Process.ID)
	}

	for _, proc := range oldNode.Processes {
		// Make sure that no app with the same ID has a process
		if proc.AppID == t.AppID {
			return fmt.Errorf("app with id %s already has a process", t.AppID)
		}
	}

	return nil
}

type ProcessRunningTransition struct {
	ProcessID string `json:"process_id"`
	// External process ID used by the driver. For example, docker container ID.
	ExtProcessID string `json:"ext_process_id"`
}

func (t *ProcessRunningTransition) Type() string {
	return TransitionProcessRunning
}

func (t *ProcessRunningTransition) Patch(oldState hdb.SerializedState) (jsondiff.Patch, error) {
	return jsondiff.Patch{
		{
			Type:  jsondiff.OperationReplace,
			Path:  fmt.Sprintf("/processes/%s/state", t.ProcessID),
			Value: ProcessStateRunning,
		},
		{
			Type:  jsondiff.OperationReplace,
			Path:  fmt.Sprintf("/processes/%s/ext_driver_id", t.ProcessID),
			Value: t.ExtProcessID,
		},
	}, nil
}

func (t *ProcessRunningTransition) Enrich(oldState hdb.SerializedState) error {
	return nil
}

func (t *ProcessRunningTransition) Validate(oldState hdb.SerializedState) error {
	var oldNode State
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	// Make sure there is a matching process
	proc, ok := oldNode.Processes[t.ProcessID]
	if !ok {
		return fmt.Errorf("process with id %s not found", t.ProcessID)
	}
	if proc.State != ProcessStateStarting {
		return fmt.Errorf("Process with id %s is in state %s, must be in state %s", t.ProcessID, proc.State, ProcessStateStarting)
	}

	return nil
}

type ProcessStopTransition struct {
	ProcessID string `json:"process_id"`
}

func (t *ProcessStopTransition) Type() string {
	return TransitionStopProcess
}

func (t *ProcessStopTransition) Patch(oldState hdb.SerializedState) (jsondiff.Patch, error) {
	return jsondiff.Patch{
		{
			Type: jsondiff.OperationRemove,
			Path: fmt.Sprintf("/processes/%s", t.ProcessID),
		},
	}, nil
}

func (t *ProcessStopTransition) Enrich(oldState hdb.SerializedState) error {
	return nil
}

func (t *ProcessStopTransition) Validate(oldState hdb.SerializedState) error {
	var oldNode State
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	// Make sure there is a matching process
	_, ok := oldNode.Processes[t.ProcessID]
	if !ok {
		return fmt.Errorf("process with id %s not found", t.ProcessID)
	}
	return nil
}

type AddReverseProxyRuleTransition struct {
	Rule *ReverseProxyRule `json:"rule"`
}

func (t *AddReverseProxyRuleTransition) Type() string {
	return TransitionAddReverseProxyRule
}

func (t *AddReverseProxyRuleTransition) Patch(oldState hdb.SerializedState) (jsondiff.Patch, error) {
	return jsondiff.Patch{
		{
			Type:  jsondiff.OperationAdd,
			Path:  fmt.Sprintf("/reverse_proxy_rules/%s", t.Rule.ID),
			Value: t.Rule,
		},
	}, nil
}

func (t *AddReverseProxyRuleTransition) Enrich(oldState hdb.SerializedState) error {
	if t.Rule.ID == "" {
		t.Rule.ID = uuid.New().String()
	}

	return nil
}

func (t *AddReverseProxyRuleTransition) Validate(oldState hdb.SerializedState) error {
	var oldNode State
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	for _, rule := range *oldNode.ReverseProxyRules {
		if rule.ID == t.Rule.ID {
			return fmt.Errorf("reverse proxy rule with id %s already exists", t.Rule.ID)
		}
		if rule.Matcher == t.Rule.Matcher {
			return fmt.Errorf("reverse proxy rule with matcher %s already exists", t.Rule.Matcher)
		}
	}

	return nil
}
