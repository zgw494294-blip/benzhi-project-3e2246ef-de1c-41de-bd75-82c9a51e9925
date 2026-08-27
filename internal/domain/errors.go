package domain

import "fmt"

type ErrorKind string

const (
	KindValidation ErrorKind = "validation"
	KindConflict   ErrorKind = "conflict"
	KindNotFound   ErrorKind = "not_found"
	KindForbidden  ErrorKind = "forbidden"
)

type BusinessError struct {
	Kind    ErrorKind
	Code    string
	Message string
	Details map[string]any
}

func (e *BusinessError) Error() string { return e.Message }

func NewError(kind ErrorKind, code, message string) *BusinessError {
	return &BusinessError{Kind: kind, Code: code, Message: message}
}

func Validation(code, format string, args ...any) error {
	return NewError(KindValidation, code, fmt.Sprintf(format, args...))
}

func Conflict(code, format string, args ...any) error {
	return NewError(KindConflict, code, fmt.Sprintf(format, args...))
}

func NotFound(entity, id string) error {
	return NewError(KindNotFound, "not_found", fmt.Sprintf("未找到%s %s", entity, id))
}

func Forbidden(code, message string) error {
	return NewError(KindForbidden, code, message)
}
