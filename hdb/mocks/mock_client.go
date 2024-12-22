// Code generated by MockGen. DO NOT EDIT.
// Source: internal/node/hdb/client.go
//
// Generated by this command:
//
//	mockgen -source=internal/node/hdb/client.go -package hdb
//

// Package hdb is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	hdb "github.com/eagraf/habitat-new/hdb"

	gomock "go.uber.org/mock/gomock"
)

// MockClient is a mock of Client interface.
type MockClient struct {
	ctrl     *gomock.Controller
	recorder *MockClientMockRecorder
}

// MockClientMockRecorder is the mock recorder for MockClient.
type MockClientMockRecorder struct {
	mock *MockClient
}

// NewMockClient creates a new mock instance.
func NewMockClient(ctrl *gomock.Controller) *MockClient {
	mock := &MockClient{ctrl: ctrl}
	mock.recorder = &MockClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClient) EXPECT() *MockClientMockRecorder {
	return m.recorder
}

// Bytes mocks base method.
func (m *MockClient) Bytes() []byte {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Bytes")
	ret0, _ := ret[0].([]byte)
	return ret0
}

// Bytes indicates an expected call of Bytes.
func (mr *MockClientMockRecorder) Bytes() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Bytes", reflect.TypeOf((*MockClient)(nil).Bytes))
}

// DatabaseID mocks base method.
func (m *MockClient) DatabaseID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DatabaseID")
	ret0, _ := ret[0].(string)
	return ret0
}

// DatabaseID indicates an expected call of DatabaseID.
func (mr *MockClientMockRecorder) DatabaseID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DatabaseID", reflect.TypeOf((*MockClient)(nil).DatabaseID))
}

// ProposeTransitions mocks base method.
func (m *MockClient) ProposeTransitions(transitions []hdb.Transition) (*hdb.JSONState, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ProposeTransitions", transitions)
	ret0, _ := ret[0].(*hdb.JSONState)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ProposeTransitions indicates an expected call of ProposeTransitions.
func (mr *MockClientMockRecorder) ProposeTransitions(transitions any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ProposeTransitions", reflect.TypeOf((*MockClient)(nil).ProposeTransitions), transitions)
}
