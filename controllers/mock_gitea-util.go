// Code generated by MockGen. DO NOT EDIT.
// Source: controllers/gitea-util.go
//
// Generated by this command:
//
//	mockgen -source controllers/gitea-util.go -package controllers -self_package=github.com/hybrid-cloud-patterns/patterns-operator/controllers
//
// Package controllers is a generated GoMock package.
package controllers

import (
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
	kubernetes "k8s.io/client-go/kubernetes"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

// MockGiteaOperations is a mock of GiteaOperations interface.
type MockGiteaOperations struct {
	ctrl     *gomock.Controller
	recorder *MockGiteaOperationsMockRecorder
}

// MockGiteaOperationsMockRecorder is the mock recorder for MockGiteaOperations.
type MockGiteaOperationsMockRecorder struct {
	mock *MockGiteaOperations
}

// NewMockGiteaOperations creates a new mock instance.
func NewMockGiteaOperations(ctrl *gomock.Controller) *MockGiteaOperations {
	mock := &MockGiteaOperations{ctrl: ctrl}
	mock.recorder = &MockGiteaOperationsMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockGiteaOperations) EXPECT() *MockGiteaOperationsMockRecorder {
	return m.recorder
}

// CreateGiteaInstance mocks base method.
func (m *MockGiteaOperations) CreateGiteaInstance(c client.Client) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateGiteaInstance", c)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateGiteaInstance indicates an expected call of CreateGiteaInstance.
func (mr *MockGiteaOperationsMockRecorder) CreateGiteaInstance(c any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateGiteaInstance", reflect.TypeOf((*MockGiteaOperations)(nil).CreateGiteaInstance), c)
}

// HasGiteaInstance mocks base method.
func (m *MockGiteaOperations) HasGiteaInstance(c client.Client) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasGiteaInstance", c)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// HasGiteaInstance indicates an expected call of HasGiteaInstance.
func (mr *MockGiteaOperationsMockRecorder) HasGiteaInstance(c any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasGiteaInstance", reflect.TypeOf((*MockGiteaOperations)(nil).HasGiteaInstance), c)
}

// MigrateGiteaRepo mocks base method.
func (m *MockGiteaOperations) MigrateGiteaRepo(fullClient kubernetes.Interface, username, password, upstreamURL, giteaServerRoute string) (bool, string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MigrateGiteaRepo", fullClient, username, password, upstreamURL, giteaServerRoute)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(string)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// MigrateGiteaRepo indicates an expected call of MigrateGiteaRepo.
func (mr *MockGiteaOperationsMockRecorder) MigrateGiteaRepo(fullClient, username, password, upstreamURL, giteaServerRoute any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MigrateGiteaRepo", reflect.TypeOf((*MockGiteaOperations)(nil).MigrateGiteaRepo), fullClient, username, password, upstreamURL, giteaServerRoute)
}
