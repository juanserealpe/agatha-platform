package errors

type ErrorCode string

const (
	CodeNotFound     ErrorCode = "NOT_FOUND"
	CodeInvalidInput ErrorCode = "INVALID_INPUT"
	CodeFutureDate   ErrorCode = "FUTURE_DATE"
)
