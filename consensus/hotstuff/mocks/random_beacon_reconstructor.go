// Code generated by mockery v2.13.1. DO NOT EDIT.

package mocks

import (
	crypto "github.com/onflow/flow-go/crypto"
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"
)

// RandomBeaconReconstructor is an autogenerated mock type for the RandomBeaconReconstructor type
type RandomBeaconReconstructor struct {
	mock.Mock
}

// EnoughShares provides a mock function with given fields:
func (_m *RandomBeaconReconstructor) EnoughShares() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Reconstruct provides a mock function with given fields:
func (_m *RandomBeaconReconstructor) Reconstruct() (crypto.Signature, error) {
	ret := _m.Called()

	var r0 crypto.Signature
	if rf, ok := ret.Get(0).(func() crypto.Signature); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(crypto.Signature)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// TrustedAdd provides a mock function with given fields: signerID, sig
func (_m *RandomBeaconReconstructor) TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (bool, error) {
	ret := _m.Called(signerID, sig)

	var r0 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier, crypto.Signature) bool); ok {
		r0 = rf(signerID, sig)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(flow.Identifier, crypto.Signature) error); ok {
		r1 = rf(signerID, sig)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Verify provides a mock function with given fields: signerID, sig
func (_m *RandomBeaconReconstructor) Verify(signerID flow.Identifier, sig crypto.Signature) error {
	ret := _m.Called(signerID, sig)

	var r0 error
	if rf, ok := ret.Get(0).(func(flow.Identifier, crypto.Signature) error); ok {
		r0 = rf(signerID, sig)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewRandomBeaconReconstructor interface {
	mock.TestingT
	Cleanup(func())
}

// NewRandomBeaconReconstructor creates a new instance of RandomBeaconReconstructor. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewRandomBeaconReconstructor(t mockConstructorTestingTNewRandomBeaconReconstructor) *RandomBeaconReconstructor {
	mock := &RandomBeaconReconstructor{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
