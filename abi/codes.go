package abi

import "fmt"

// ResultCode represents the result of transaction processing.
// Code 0 indicates success; all other codes indicate errors.
// Codes 1-99 are reserved for framework use.
// Codes 100-999 are available for application-specific errors.
type ResultCode uint32

const (
	// CodeOK indicates the operation succeeded.
	CodeOK ResultCode = 0

	// Framework error codes (1-99)

	// CodeUnknownError indicates an unspecified error.
	CodeUnknownError ResultCode = 1

	// CodeInvalidTx indicates the transaction is malformed or invalid.
	CodeInvalidTx ResultCode = 2

	// CodeInsufficientFunds indicates the sender lacks required funds.
	CodeInsufficientFunds ResultCode = 3

	// CodeInvalidNonce indicates the nonce is invalid (too low or too high).
	CodeInvalidNonce ResultCode = 4

	// CodeInsufficientGas indicates the gas limit is too low.
	CodeInsufficientGas ResultCode = 5

	// CodeNotAuthorized indicates the operation is not permitted.
	CodeNotAuthorized ResultCode = 6

	// CodeInvalidState indicates the state is invalid for this operation.
	CodeInvalidState ResultCode = 7

	// CodeTimeout indicates the operation timed out.
	CodeTimeout ResultCode = 8

	// CodeNotFound indicates the requested resource was not found.
	CodeNotFound ResultCode = 9

	// CodeAlreadyExists indicates the resource already exists.
	CodeAlreadyExists ResultCode = 10

	// Application error codes start at 100
	CodeAppErrorStart ResultCode = 100
)

// IsOK returns true if the code indicates success.
func (c ResultCode) IsOK() bool {
	return c == CodeOK
}

// IsError returns true if the code indicates an error.
func (c ResultCode) IsError() bool {
	return c != CodeOK
}

// IsAppError returns true if this is an application-specific error code.
func (c ResultCode) IsAppError() bool {
	return c >= CodeAppErrorStart
}

// String returns a human-readable description of the code.
func (c ResultCode) String() string {
	switch c {
	case CodeOK:
		return "OK"
	case CodeUnknownError:
		return "UnknownError"
	case CodeInvalidTx:
		return "InvalidTx"
	case CodeInsufficientFunds:
		return "InsufficientFunds"
	case CodeInvalidNonce:
		return "InvalidNonce"
	case CodeInsufficientGas:
		return "InsufficientGas"
	case CodeNotAuthorized:
		return "NotAuthorized"
	case CodeInvalidState:
		return "InvalidState"
	case CodeTimeout:
		return "Timeout"
	case CodeNotFound:
		return "NotFound"
	case CodeAlreadyExists:
		return "AlreadyExists"
	default:
		if c.IsAppError() {
			return fmt.Sprintf("AppError(%d)", c)
		}
		return fmt.Sprintf("Unknown(%d)", c)
	}
}
