// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package errors

import "fmt"

// ServiceUnavailable indicates a downstream dependency is not reachable.
type ServiceUnavailable struct {
	Message string
	Cause   error
}

func (e *ServiceUnavailable) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *ServiceUnavailable) Unwrap() error { return e.Cause }

// NewServiceUnavailable creates a ServiceUnavailable error.
func NewServiceUnavailable(msg string, cause ...error) error {
	e := &ServiceUnavailable{Message: msg}
	if len(cause) > 0 {
		e.Cause = cause[0]
	}
	return e
}

// Unexpected wraps an unclassified internal error.
type Unexpected struct {
	Message string
	Cause   error
}

func (e *Unexpected) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *Unexpected) Unwrap() error { return e.Cause }

// NewUnexpected creates an Unexpected error.
func NewUnexpected(msg string, cause ...error) error {
	e := &Unexpected{Message: msg}
	if len(cause) > 0 {
		e.Cause = cause[0]
	}
	return e
}
