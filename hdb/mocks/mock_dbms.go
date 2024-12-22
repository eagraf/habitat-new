// Code generated by MockGen. DO NOT EDIT.
// Source: internal/node/hdb/dbms.go
//
// Generated by this command:
//
//	mockgen -source=internal/node/hdb/dbms.go -package mocks --destination internal/node/hdb/mocks/mock_dbms.go
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	hdb "github.com/eagraf/habitat-new/hdb"

	gomock "go.uber.org/mock/gomock"
)

// MockHDBManager is a mock of HDBManager interface.
type MockHDBManager struct {
	ctrl     *gomock.Controller
	recorder *MockHDBManagerMockRecorder
}

// MockHDBManagerMockRecorder is the mock recorder for MockHDBManager.
type MockHDBManagerMockRecorder struct {
	mock *MockHDBManager
}

// NewMockHDBManager creates a new mock instance.
func NewMockHDBManager(ctrl *gomock.Controller) *MockHDBManager {
	mock := &MockHDBManager{ctrl: ctrl}
	mock.recorder = &MockHDBManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockHDBManager) EXPECT() *MockHDBManagerMockRecorder {
	return m.recorder
}

// CreateDatabase mocks base method.
func (m *MockHDBManager) CreateDatabase(name, schemaType string, initialTransitions []hdb.Transition) (hdb.Client, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateDatabase", name, schemaType, initialTransitions)
	ret0, _ := ret[0].(hdb.Client)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateDatabase indicates an expected call of CreateDatabase.
func (mr *MockHDBManagerMockRecorder) CreateDatabase(name, schemaType, initialTransitions any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateDatabase", reflect.TypeOf((*MockHDBManager)(nil).CreateDatabase), name, schemaType, initialTransitions)
}

// GetDatabaseClient mocks base method.
func (m *MockHDBManager) GetDatabaseClient(id string) (hdb.Client, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDatabaseClient", id)
	ret0, _ := ret[0].(hdb.Client)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDatabaseClient indicates an expected call of GetDatabaseClient.
func (mr *MockHDBManagerMockRecorder) GetDatabaseClient(id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDatabaseClient", reflect.TypeOf((*MockHDBManager)(nil).GetDatabaseClient), id)
}

// GetDatabaseClientByName mocks base method.
func (m *MockHDBManager) GetDatabaseClientByName(name string) (hdb.Client, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDatabaseClientByName", name)
	ret0, _ := ret[0].(hdb.Client)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDatabaseClientByName indicates an expected call of GetDatabaseClientByName.
func (mr *MockHDBManagerMockRecorder) GetDatabaseClientByName(name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDatabaseClientByName", reflect.TypeOf((*MockHDBManager)(nil).GetDatabaseClientByName), name)
}

// RestartDBs mocks base method.
func (m *MockHDBManager) RestartDBs() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RestartDBs")
	ret0, _ := ret[0].(error)
	return ret0
}

// RestartDBs indicates an expected call of RestartDBs.
func (mr *MockHDBManagerMockRecorder) RestartDBs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RestartDBs", reflect.TypeOf((*MockHDBManager)(nil).RestartDBs))
}

// Start mocks base method.
func (m *MockHDBManager) Start() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Start")
}

// Start indicates an expected call of Start.
func (mr *MockHDBManagerMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockHDBManager)(nil).Start))
}

// Stop mocks base method.
func (m *MockHDBManager) Stop() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Stop")
}

// Stop indicates an expected call of Stop.
func (mr *MockHDBManagerMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockHDBManager)(nil).Stop))
}

// MockDatabaseConfig is a mock of DatabaseConfig interface.
type MockDatabaseConfig struct {
	ctrl     *gomock.Controller
	recorder *MockDatabaseConfigMockRecorder
}

// MockDatabaseConfigMockRecorder is the mock recorder for MockDatabaseConfig.
type MockDatabaseConfigMockRecorder struct {
	mock *MockDatabaseConfig
}

// NewMockDatabaseConfig creates a new mock instance.
func NewMockDatabaseConfig(ctrl *gomock.Controller) *MockDatabaseConfig {
	mock := &MockDatabaseConfig{ctrl: ctrl}
	mock.recorder = &MockDatabaseConfigMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDatabaseConfig) EXPECT() *MockDatabaseConfigMockRecorder {
	return m.recorder
}

// ID mocks base method.
func (m *MockDatabaseConfig) ID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ID")
	ret0, _ := ret[0].(string)
	return ret0
}

// ID indicates an expected call of ID.
func (mr *MockDatabaseConfigMockRecorder) ID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ID", reflect.TypeOf((*MockDatabaseConfig)(nil).ID))
}

// Path mocks base method.
func (m *MockDatabaseConfig) Path() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Path")
	ret0, _ := ret[0].(string)
	return ret0
}

// Path indicates an expected call of Path.
func (mr *MockDatabaseConfigMockRecorder) Path() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Path", reflect.TypeOf((*MockDatabaseConfig)(nil).Path))
}
