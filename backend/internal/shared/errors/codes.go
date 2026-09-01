package errors

type ErrorCode string

const (
	CodeNotFound           ErrorCode = "NOT_FOUND"
	CodeInvalidRange       ErrorCode = "INVALID_RANGE"
	CodeFutureDate         ErrorCode = "FUTURE_DATE"
	CodeBeforeInitialDate  ErrorCode = "BEFORE_INITIAL_DATE"
	CodeAlreadyIngested    ErrorCode = "ALREADY_INGESTED"
	CodeDeviceInactive     ErrorCode = "DEVICE_INACTIVE"
	CodeInvalidMeasuringID ErrorCode = "INVALID_MEASURING_POINT_ID"
)
