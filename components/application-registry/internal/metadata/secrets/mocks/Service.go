// Code generated by mockery v2.2.1. DO NOT EDIT.

package mocks

import (
	apperrors "github.com/kyma-project/kyma/components/application-registry/internal/apperrors"
	applications "github.com/kyma-project/kyma/components/application-registry/internal/metadata/applications"

	mock "github.com/stretchr/testify/mock"

	model "github.com/kyma-project/kyma/components/application-registry/internal/metadata/model"

	types "k8s.io/apimachinery/pkg/types"
)

// Service is an autogenerated mock type for the Service type
type Service struct {
	mock.Mock
}

// Create provides a mock function with given fields: application, appUID, serviceID, credentials
func (_m *Service) Create(application string, appUID types.UID, serviceID string, credentials *model.CredentialsWithCSRF) (applications.Credentials, apperrors.AppError) {
	ret := _m.Called(application, appUID, serviceID, credentials)

	var r0 applications.Credentials
	if rf, ok := ret.Get(0).(func(string, types.UID, string, *model.CredentialsWithCSRF) applications.Credentials); ok {
		r0 = rf(application, appUID, serviceID, credentials)
	} else {
		r0 = ret.Get(0).(applications.Credentials)
	}

	var r1 apperrors.AppError
	if rf, ok := ret.Get(1).(func(string, types.UID, string, *model.CredentialsWithCSRF) apperrors.AppError); ok {
		r1 = rf(application, appUID, serviceID, credentials)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(apperrors.AppError)
		}
	}

	return r0, r1
}

// Delete provides a mock function with given fields: name
func (_m *Service) Delete(name string) apperrors.AppError {
	ret := _m.Called(name)

	var r0 apperrors.AppError
	if rf, ok := ret.Get(0).(func(string) apperrors.AppError); ok {
		r0 = rf(name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(apperrors.AppError)
		}
	}

	return r0
}

// Get provides a mock function with given fields: application, credentials
func (_m *Service) Get(application string, credentials applications.Credentials) (model.CredentialsWithCSRF, apperrors.AppError) {
	ret := _m.Called(application, credentials)

	var r0 model.CredentialsWithCSRF
	if rf, ok := ret.Get(0).(func(string, applications.Credentials) model.CredentialsWithCSRF); ok {
		r0 = rf(application, credentials)
	} else {
		r0 = ret.Get(0).(model.CredentialsWithCSRF)
	}

	var r1 apperrors.AppError
	if rf, ok := ret.Get(1).(func(string, applications.Credentials) apperrors.AppError); ok {
		r1 = rf(application, credentials)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(apperrors.AppError)
		}
	}

	return r0, r1
}

// Upsert provides a mock function with given fields: application, appUID, serviceID, credentials
func (_m *Service) Upsert(application string, appUID types.UID, serviceID string, credentials *model.CredentialsWithCSRF) (applications.Credentials, apperrors.AppError) {
	ret := _m.Called(application, appUID, serviceID, credentials)

	var r0 applications.Credentials
	if rf, ok := ret.Get(0).(func(string, types.UID, string, *model.CredentialsWithCSRF) applications.Credentials); ok {
		r0 = rf(application, appUID, serviceID, credentials)
	} else {
		r0 = ret.Get(0).(applications.Credentials)
	}

	var r1 apperrors.AppError
	if rf, ok := ret.Get(1).(func(string, types.UID, string, *model.CredentialsWithCSRF) apperrors.AppError); ok {
		r1 = rf(application, appUID, serviceID, credentials)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(apperrors.AppError)
		}
	}

	return r0, r1
}