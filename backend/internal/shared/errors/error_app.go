package errors

import "fmt"

type AppError struct {
	Code    ErrorCode
	Message string
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) ErrorWithCode() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}
