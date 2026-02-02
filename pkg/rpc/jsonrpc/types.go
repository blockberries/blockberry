// Package jsonrpc provides a JSON-RPC 2.0 implementation for the RPC server.
package jsonrpc

import (
	"encoding/json"
)

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
	ID      interface{}     `json:"id"`
}

// Error represents a JSON-RPC 2.0 error.
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Message
}

// Standard JSON-RPC 2.0 error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// Standard errors.
var (
	ErrParseError     = &Error{Code: CodeParseError, Message: "Parse error"}
	ErrInvalidRequest = &Error{Code: CodeInvalidRequest, Message: "Invalid Request"}
	ErrMethodNotFound = &Error{Code: CodeMethodNotFound, Message: "Method not found"}
	ErrInvalidParams  = &Error{Code: CodeInvalidParams, Message: "Invalid params"}
	ErrInternalError  = &Error{Code: CodeInternalError, Message: "Internal error"}
)

// NewResponse creates a successful response.
func NewResponse(id interface{}, result interface{}) (*Response, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return &Response{
		JSONRPC: "2.0",
		Result:  data,
		ID:      id,
	}, nil
}

// NewErrorResponse creates an error response.
func NewErrorResponse(id interface{}, err *Error) *Response {
	return &Response{
		JSONRPC: "2.0",
		Error:   err,
		ID:      id,
	}
}

// NewErrorWithData creates an error with additional data.
func NewErrorWithData(code int, message string, data interface{}) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// BatchRequest is a batch of JSON-RPC requests.
type BatchRequest []Request

// BatchResponse is a batch of JSON-RPC responses.
type BatchResponse []Response
