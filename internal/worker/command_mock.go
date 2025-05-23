// Code generated by MockGen. DO NOT EDIT.
// Source: command.go
//
// Generated by this command:
//
//	mockgen -source=command.go -destination=command_mock.go -package=worker
//

// Package worker is a generated GoMock package.
package worker

import (
	io "io"
	os "os"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockCommand is a mock of Command interface.
type MockCommand struct {
	ctrl     *gomock.Controller
	recorder *MockCommandMockRecorder
	isgomock struct{}
}

// MockCommandMockRecorder is the mock recorder for MockCommand.
type MockCommandMockRecorder struct {
	mock *MockCommand
}

// NewMockCommand creates a new mock instance.
func NewMockCommand(ctrl *gomock.Controller) *MockCommand {
	mock := &MockCommand{ctrl: ctrl}
	mock.recorder = &MockCommandMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCommand) EXPECT() *MockCommandMockRecorder {
	return m.recorder
}

// Kill mocks base method.
func (m *MockCommand) Kill() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Kill")
	ret0, _ := ret[0].(error)
	return ret0
}

// Kill indicates an expected call of Kill.
func (mr *MockCommandMockRecorder) Kill() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Kill", reflect.TypeOf((*MockCommand)(nil).Kill))
}

// ProcessState mocks base method.
func (m *MockCommand) ProcessState() *os.ProcessState {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ProcessState")
	ret0, _ := ret[0].(*os.ProcessState)
	return ret0
}

// ProcessState indicates an expected call of ProcessState.
func (mr *MockCommandMockRecorder) ProcessState() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ProcessState", reflect.TypeOf((*MockCommand)(nil).ProcessState))
}

// SetEnv mocks base method.
func (m *MockCommand) SetEnv(arg0 []string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetEnv", arg0)
}

// SetEnv indicates an expected call of SetEnv.
func (mr *MockCommandMockRecorder) SetEnv(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetEnv", reflect.TypeOf((*MockCommand)(nil).SetEnv), arg0)
}

// SetStderr mocks base method.
func (m *MockCommand) SetStderr(arg0 io.Writer) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetStderr", arg0)
}

// SetStderr indicates an expected call of SetStderr.
func (mr *MockCommandMockRecorder) SetStderr(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetStderr", reflect.TypeOf((*MockCommand)(nil).SetStderr), arg0)
}

// SetStdout mocks base method.
func (m *MockCommand) SetStdout(arg0 io.Writer) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetStdout", arg0)
}

// SetStdout indicates an expected call of SetStdout.
func (mr *MockCommandMockRecorder) SetStdout(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetStdout", reflect.TypeOf((*MockCommand)(nil).SetStdout), arg0)
}

// Start mocks base method.
func (m *MockCommand) Start() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start")
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start.
func (mr *MockCommandMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockCommand)(nil).Start))
}

// Stop mocks base method.
func (m *MockCommand) Stop() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop")
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop.
func (mr *MockCommandMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockCommand)(nil).Stop))
}

// Wait mocks base method.
func (m *MockCommand) Wait() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Wait")
	ret0, _ := ret[0].(error)
	return ret0
}

// Wait indicates an expected call of Wait.
func (mr *MockCommandMockRecorder) Wait() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Wait", reflect.TypeOf((*MockCommand)(nil).Wait))
}

// MockCommandExecutor is a mock of CommandExecutor interface.
type MockCommandExecutor struct {
	ctrl     *gomock.Controller
	recorder *MockCommandExecutorMockRecorder
	isgomock struct{}
}

// MockCommandExecutorMockRecorder is the mock recorder for MockCommandExecutor.
type MockCommandExecutorMockRecorder struct {
	mock *MockCommandExecutor
}

// NewMockCommandExecutor creates a new mock instance.
func NewMockCommandExecutor(ctrl *gomock.Controller) *MockCommandExecutor {
	mock := &MockCommandExecutor{ctrl: ctrl}
	mock.recorder = &MockCommandExecutorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCommandExecutor) EXPECT() *MockCommandExecutorMockRecorder {
	return m.recorder
}

// NewCommand mocks base method.
func (m *MockCommandExecutor) NewCommand(name string, arg ...string) Command {
	m.ctrl.T.Helper()
	varargs := []any{name}
	for _, a := range arg {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "NewCommand", varargs...)
	ret0, _ := ret[0].(Command)
	return ret0
}

// NewCommand indicates an expected call of NewCommand.
func (mr *MockCommandExecutorMockRecorder) NewCommand(name any, arg ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{name}, arg...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewCommand", reflect.TypeOf((*MockCommandExecutor)(nil).NewCommand), varargs...)
}
