// Code generated by MockGen. DO NOT EDIT.
// Source: excel.go
//
// Generated by this command:
//
//	mockgen -typed -source=excel.go -destination=../../infrastructure/mock/excel.go -package mock
//

// Package mock is a generated GoMock package.
package mock

import (
	reflect "reflect"

	model "github.com/nao1215/sqly/domain/model"
	gomock "go.uber.org/mock/gomock"
)

// MockExcelRepository is a mock of ExcelRepository interface.
type MockExcelRepository struct {
	ctrl     *gomock.Controller
	recorder *MockExcelRepositoryMockRecorder
	isgomock struct{}
}

// MockExcelRepositoryMockRecorder is the mock recorder for MockExcelRepository.
type MockExcelRepositoryMockRecorder struct {
	mock *MockExcelRepository
}

// NewMockExcelRepository creates a new mock instance.
func NewMockExcelRepository(ctrl *gomock.Controller) *MockExcelRepository {
	mock := &MockExcelRepository{ctrl: ctrl}
	mock.recorder = &MockExcelRepositoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockExcelRepository) EXPECT() *MockExcelRepositoryMockRecorder {
	return m.recorder
}

// Dump mocks base method.
func (m *MockExcelRepository) Dump(excelFilePath string, table *model.Table) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Dump", excelFilePath, table)
	ret0, _ := ret[0].(error)
	return ret0
}

// Dump indicates an expected call of Dump.
func (mr *MockExcelRepositoryMockRecorder) Dump(excelFilePath, table any) *MockExcelRepositoryDumpCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Dump", reflect.TypeOf((*MockExcelRepository)(nil).Dump), excelFilePath, table)
	return &MockExcelRepositoryDumpCall{Call: call}
}

// MockExcelRepositoryDumpCall wrap *gomock.Call
type MockExcelRepositoryDumpCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockExcelRepositoryDumpCall) Return(arg0 error) *MockExcelRepositoryDumpCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockExcelRepositoryDumpCall) Do(f func(string, *model.Table) error) *MockExcelRepositoryDumpCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockExcelRepositoryDumpCall) DoAndReturn(f func(string, *model.Table) error) *MockExcelRepositoryDumpCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// List mocks base method.
func (m *MockExcelRepository) List(excelFilePath, sheetName string) (*model.Excel, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", excelFilePath, sheetName)
	ret0, _ := ret[0].(*model.Excel)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockExcelRepositoryMockRecorder) List(excelFilePath, sheetName any) *MockExcelRepositoryListCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockExcelRepository)(nil).List), excelFilePath, sheetName)
	return &MockExcelRepositoryListCall{Call: call}
}

// MockExcelRepositoryListCall wrap *gomock.Call
type MockExcelRepositoryListCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockExcelRepositoryListCall) Return(arg0 *model.Excel, arg1 error) *MockExcelRepositoryListCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockExcelRepositoryListCall) Do(f func(string, string) (*model.Excel, error)) *MockExcelRepositoryListCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockExcelRepositoryListCall) DoAndReturn(f func(string, string) (*model.Excel, error)) *MockExcelRepositoryListCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
