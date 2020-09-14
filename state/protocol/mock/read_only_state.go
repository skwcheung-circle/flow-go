// Code generated by mockery v1.0.0. DO NOT EDIT.

package mock

import (
	flow "github.com/dapperlabs/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"

	protocol "github.com/dapperlabs/flow-go/state/protocol"
)

// ReadOnlyState is an autogenerated mock type for the ReadOnlyState type
type ReadOnlyState struct {
	mock.Mock
}

// AtBlockID provides a mock function with given fields: blockID
func (_m *ReadOnlyState) AtBlockID(blockID flow.Identifier) protocol.Snapshot {
	ret := _m.Called(blockID)

	var r0 protocol.Snapshot
	if rf, ok := ret.Get(0).(func(flow.Identifier) protocol.Snapshot); ok {
		r0 = rf(blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(protocol.Snapshot)
		}
	}

	return r0
}

// AtHeight provides a mock function with given fields: height
func (_m *ReadOnlyState) AtHeight(height uint64) protocol.Snapshot {
	ret := _m.Called(height)

	var r0 protocol.Snapshot
	if rf, ok := ret.Get(0).(func(uint64) protocol.Snapshot); ok {
		r0 = rf(height)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(protocol.Snapshot)
		}
	}

	return r0
}

// Final provides a mock function with given fields:
func (_m *ReadOnlyState) Final() protocol.Snapshot {
	ret := _m.Called()

	var r0 protocol.Snapshot
	if rf, ok := ret.Get(0).(func() protocol.Snapshot); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(protocol.Snapshot)
		}
	}

	return r0
}

// Params provides a mock function with given fields:
func (_m *ReadOnlyState) Params() protocol.Params {
	ret := _m.Called()

	var r0 protocol.Params
	if rf, ok := ret.Get(0).(func() protocol.Params); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(protocol.Params)
		}
	}

	return r0
}

// Sealed provides a mock function with given fields:
func (_m *ReadOnlyState) Sealed() protocol.Snapshot {
	ret := _m.Called()

	var r0 protocol.Snapshot
	if rf, ok := ret.Get(0).(func() protocol.Snapshot); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(protocol.Snapshot)
		}
	}

	return r0
}
