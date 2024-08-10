// Code generated by MockGen. DO NOT EDIT.
// Source: internal/node/package_manager/package_manager.go
//
// Generated by this command:
//
//	mockgen -source=internal/node/package_manager/package_manager.go -package mocks -destination=internal/node/package_manager/mocks/mock_package_manager.go
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	node "github.com/eagraf/habitat-new/core/state/node"
	gomock "go.uber.org/mock/gomock"
)

// MockPackageManager is a mock of PackageManager interface.
type MockPackageManager struct {
	ctrl     *gomock.Controller
	recorder *MockPackageManagerMockRecorder
}

// MockPackageManagerMockRecorder is the mock recorder for MockPackageManager.
type MockPackageManagerMockRecorder struct {
	mock *MockPackageManager
}

// NewMockPackageManager creates a new mock instance.
func NewMockPackageManager(ctrl *gomock.Controller) *MockPackageManager {
	mock := &MockPackageManager{ctrl: ctrl}
	mock.recorder = &MockPackageManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPackageManager) EXPECT() *MockPackageManagerMockRecorder {
	return m.recorder
}

// Driver mocks base method.
func (m *MockPackageManager) Driver() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Driver")
	ret0, _ := ret[0].(string)
	return ret0
}

// Driver indicates an expected call of Driver.
func (mr *MockPackageManagerMockRecorder) Driver() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Driver", reflect.TypeOf((*MockPackageManager)(nil).Driver))
}

// InstallPackage mocks base method.
func (m *MockPackageManager) InstallPackage(packageSpec *node.Package, version string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "InstallPackage", packageSpec, version)
	ret0, _ := ret[0].(error)
	return ret0
}

// InstallPackage indicates an expected call of InstallPackage.
func (mr *MockPackageManagerMockRecorder) InstallPackage(packageSpec, version any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InstallPackage", reflect.TypeOf((*MockPackageManager)(nil).InstallPackage), packageSpec, version)
}

// IsInstalled mocks base method.
func (m *MockPackageManager) IsInstalled(packageSpec *node.Package, version string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsInstalled", packageSpec, version)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsInstalled indicates an expected call of IsInstalled.
func (mr *MockPackageManagerMockRecorder) IsInstalled(packageSpec, version any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsInstalled", reflect.TypeOf((*MockPackageManager)(nil).IsInstalled), packageSpec, version)
}

// UninstallPackage mocks base method.
func (m *MockPackageManager) UninstallPackage(packageSpec *node.Package, version string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UninstallPackage", packageSpec, version)
	ret0, _ := ret[0].(error)
	return ret0
}

// UninstallPackage indicates an expected call of UninstallPackage.
func (mr *MockPackageManagerMockRecorder) UninstallPackage(packageSpec, version any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UninstallPackage", reflect.TypeOf((*MockPackageManager)(nil).UninstallPackage), packageSpec, version)
}
