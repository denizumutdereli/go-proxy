package common

import (
	"fmt"
	"net/http"
)

type Error interface {
	error
	StatusCode() int
	InternalCode() string
	InternalError() error
	ErrorMessage() string
}

type appError struct {
	statusCode    int
	internalCode  string
	internalError error
	message       string
}

func AppError(statusCode int, internalCode, message string, err error) Error {
	return &appError{
		statusCode:    statusCode,
		internalCode:  internalCode,
		internalError: err,
		message:       message,
	}
}

func (e *appError) StatusCode() int {
	if e.statusCode != 0 {
		return e.statusCode
	}
	return http.StatusInternalServerError
}

func (e *appError) InternalCode() string {
	return e.internalCode
}

func (e *appError) ErrorMessage() string {
	return e.message
}

func (e *appError) Error() string {
	if e.internalError != nil {
		return fmt.Sprintf("Error: %s; InternalCode: %s; Message: %s", e.internalError, e.internalCode, e.message)
	}
	return fmt.Sprintf("InternalCode: %s; Message: %s", e.internalCode, e.message)
}

func (e *appError) WithInternalError(err error) *appError {
	e.internalError = err
	return e
}

func (e *appError) SetStatus(status int) *appError {
	e.statusCode = status
	return e
}

func (e *appError) SetMessage(message string) *appError {
	e.message = message
	return e
}

func (e *appError) Unwrap() error {
	return e.internalError
}

func (e *appError) InternalError() error {
	return e.internalError
}
