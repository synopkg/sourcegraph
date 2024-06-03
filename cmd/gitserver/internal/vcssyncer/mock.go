// Code generated by go-mockgen 1.3.7; DO NOT EDIT.
//
// This file was generated by running `sg generate` (or `go-mockgen`) at the root of
// this repository. To add additional mocks to this or another package, add a new entry
// to the mockgen.yaml file in the root of this repository.

package vcssyncer

import (
	"context"
	"io"
	"sync"

	common "github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	api "github.com/sourcegraph/sourcegraph/internal/api"
)

// MockVCSSyncer is a mock implementation of the VCSSyncer interface (from
// the package
// github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/vcssyncer) used
// for unit testing.
type MockVCSSyncer struct {
	// FetchFunc is an instance of a mock function object controlling the
	// behavior of the method Fetch.
	FetchFunc *VCSSyncerFetchFunc
	// IsCloneableFunc is an instance of a mock function object controlling
	// the behavior of the method IsCloneable.
	IsCloneableFunc *VCSSyncerIsCloneableFunc
	// TypeFunc is an instance of a mock function object controlling the
	// behavior of the method Type.
	TypeFunc *VCSSyncerTypeFunc
}

// NewMockVCSSyncer creates a new mock of the VCSSyncer interface. All
// methods return zero values for all results, unless overwritten.
func NewMockVCSSyncer() *MockVCSSyncer {
	return &MockVCSSyncer{
		FetchFunc: &VCSSyncerFetchFunc{
			defaultHook: func(context.Context, api.RepoName, common.GitDir, io.Writer) (r0 error) {
				return
			},
		},
		IsCloneableFunc: &VCSSyncerIsCloneableFunc{
			defaultHook: func(context.Context, api.RepoName) (r0 error) {
				return
			},
		},
		TypeFunc: &VCSSyncerTypeFunc{
			defaultHook: func() (r0 string) {
				return
			},
		},
	}
}

// NewStrictMockVCSSyncer creates a new mock of the VCSSyncer interface. All
// methods panic on invocation, unless overwritten.
func NewStrictMockVCSSyncer() *MockVCSSyncer {
	return &MockVCSSyncer{
		FetchFunc: &VCSSyncerFetchFunc{
			defaultHook: func(context.Context, api.RepoName, common.GitDir, io.Writer) error {
				panic("unexpected invocation of MockVCSSyncer.Fetch")
			},
		},
		IsCloneableFunc: &VCSSyncerIsCloneableFunc{
			defaultHook: func(context.Context, api.RepoName) error {
				panic("unexpected invocation of MockVCSSyncer.IsCloneable")
			},
		},
		TypeFunc: &VCSSyncerTypeFunc{
			defaultHook: func() string {
				panic("unexpected invocation of MockVCSSyncer.Type")
			},
		},
	}
}

// NewMockVCSSyncerFrom creates a new mock of the MockVCSSyncer interface.
// All methods delegate to the given implementation, unless overwritten.
func NewMockVCSSyncerFrom(i VCSSyncer) *MockVCSSyncer {
	return &MockVCSSyncer{
		FetchFunc: &VCSSyncerFetchFunc{
			defaultHook: i.Fetch,
		},
		IsCloneableFunc: &VCSSyncerIsCloneableFunc{
			defaultHook: i.IsCloneable,
		},
		TypeFunc: &VCSSyncerTypeFunc{
			defaultHook: i.Type,
		},
	}
}

// VCSSyncerFetchFunc describes the behavior when the Fetch method of the
// parent MockVCSSyncer instance is invoked.
type VCSSyncerFetchFunc struct {
	defaultHook func(context.Context, api.RepoName, common.GitDir, io.Writer) error
	hooks       []func(context.Context, api.RepoName, common.GitDir, io.Writer) error
	history     []VCSSyncerFetchFuncCall
	mutex       sync.Mutex
}

// Fetch delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockVCSSyncer) Fetch(v0 context.Context, v1 api.RepoName, v2 common.GitDir, v3 io.Writer) error {
	r0 := m.FetchFunc.nextHook()(v0, v1, v2, v3)
	m.FetchFunc.appendCall(VCSSyncerFetchFuncCall{v0, v1, v2, v3, r0})
	return r0
}

// SetDefaultHook sets function that is called when the Fetch method of the
// parent MockVCSSyncer instance is invoked and the hook queue is empty.
func (f *VCSSyncerFetchFunc) SetDefaultHook(hook func(context.Context, api.RepoName, common.GitDir, io.Writer) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Fetch method of the parent MockVCSSyncer instance invokes the hook at the
// front of the queue and discards it. After the queue is empty, the default
// hook function is invoked for any future action.
func (f *VCSSyncerFetchFunc) PushHook(hook func(context.Context, api.RepoName, common.GitDir, io.Writer) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *VCSSyncerFetchFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context, api.RepoName, common.GitDir, io.Writer) error {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *VCSSyncerFetchFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context, api.RepoName, common.GitDir, io.Writer) error {
		return r0
	})
}

func (f *VCSSyncerFetchFunc) nextHook() func(context.Context, api.RepoName, common.GitDir, io.Writer) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *VCSSyncerFetchFunc) appendCall(r0 VCSSyncerFetchFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of VCSSyncerFetchFuncCall objects describing
// the invocations of this function.
func (f *VCSSyncerFetchFunc) History() []VCSSyncerFetchFuncCall {
	f.mutex.Lock()
	history := make([]VCSSyncerFetchFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// VCSSyncerFetchFuncCall is an object that describes an invocation of
// method Fetch on an instance of MockVCSSyncer.
type VCSSyncerFetchFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 api.RepoName
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 common.GitDir
	// Arg3 is the value of the 4th argument passed to this method
	// invocation.
	Arg3 io.Writer
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c VCSSyncerFetchFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2, c.Arg3}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c VCSSyncerFetchFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}

// VCSSyncerIsCloneableFunc describes the behavior when the IsCloneable
// method of the parent MockVCSSyncer instance is invoked.
type VCSSyncerIsCloneableFunc struct {
	defaultHook func(context.Context, api.RepoName) error
	hooks       []func(context.Context, api.RepoName) error
	history     []VCSSyncerIsCloneableFuncCall
	mutex       sync.Mutex
}

// IsCloneable delegates to the next hook function in the queue and stores
// the parameter and result values of this invocation.
func (m *MockVCSSyncer) IsCloneable(v0 context.Context, v1 api.RepoName) error {
	r0 := m.IsCloneableFunc.nextHook()(v0, v1)
	m.IsCloneableFunc.appendCall(VCSSyncerIsCloneableFuncCall{v0, v1, r0})
	return r0
}

// SetDefaultHook sets function that is called when the IsCloneable method
// of the parent MockVCSSyncer instance is invoked and the hook queue is
// empty.
func (f *VCSSyncerIsCloneableFunc) SetDefaultHook(hook func(context.Context, api.RepoName) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// IsCloneable method of the parent MockVCSSyncer instance invokes the hook
// at the front of the queue and discards it. After the queue is empty, the
// default hook function is invoked for any future action.
func (f *VCSSyncerIsCloneableFunc) PushHook(hook func(context.Context, api.RepoName) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *VCSSyncerIsCloneableFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context, api.RepoName) error {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *VCSSyncerIsCloneableFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context, api.RepoName) error {
		return r0
	})
}

func (f *VCSSyncerIsCloneableFunc) nextHook() func(context.Context, api.RepoName) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *VCSSyncerIsCloneableFunc) appendCall(r0 VCSSyncerIsCloneableFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of VCSSyncerIsCloneableFuncCall objects
// describing the invocations of this function.
func (f *VCSSyncerIsCloneableFunc) History() []VCSSyncerIsCloneableFuncCall {
	f.mutex.Lock()
	history := make([]VCSSyncerIsCloneableFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// VCSSyncerIsCloneableFuncCall is an object that describes an invocation of
// method IsCloneable on an instance of MockVCSSyncer.
type VCSSyncerIsCloneableFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 api.RepoName
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c VCSSyncerIsCloneableFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c VCSSyncerIsCloneableFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}

// VCSSyncerTypeFunc describes the behavior when the Type method of the
// parent MockVCSSyncer instance is invoked.
type VCSSyncerTypeFunc struct {
	defaultHook func() string
	hooks       []func() string
	history     []VCSSyncerTypeFuncCall
	mutex       sync.Mutex
}

// Type delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockVCSSyncer) Type() string {
	r0 := m.TypeFunc.nextHook()()
	m.TypeFunc.appendCall(VCSSyncerTypeFuncCall{r0})
	return r0
}

// SetDefaultHook sets function that is called when the Type method of the
// parent MockVCSSyncer instance is invoked and the hook queue is empty.
func (f *VCSSyncerTypeFunc) SetDefaultHook(hook func() string) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Type method of the parent MockVCSSyncer instance invokes the hook at the
// front of the queue and discards it. After the queue is empty, the default
// hook function is invoked for any future action.
func (f *VCSSyncerTypeFunc) PushHook(hook func() string) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *VCSSyncerTypeFunc) SetDefaultReturn(r0 string) {
	f.SetDefaultHook(func() string {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *VCSSyncerTypeFunc) PushReturn(r0 string) {
	f.PushHook(func() string {
		return r0
	})
}

func (f *VCSSyncerTypeFunc) nextHook() func() string {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *VCSSyncerTypeFunc) appendCall(r0 VCSSyncerTypeFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of VCSSyncerTypeFuncCall objects describing
// the invocations of this function.
func (f *VCSSyncerTypeFunc) History() []VCSSyncerTypeFuncCall {
	f.mutex.Lock()
	history := make([]VCSSyncerTypeFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// VCSSyncerTypeFuncCall is an object that describes an invocation of method
// Type on an instance of MockVCSSyncer.
type VCSSyncerTypeFuncCall struct {
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 string
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c VCSSyncerTypeFuncCall) Args() []interface{} {
	return []interface{}{}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c VCSSyncerTypeFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}
