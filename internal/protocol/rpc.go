// Package protocol defines the JSON-RPC 2.0 communication protocol between CLI and daemon.
package protocol

import (
	"encoding/json"
	"fmt"
)

const JSONRPCVersion = "2.0"

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      *int            `json:"id,omitempty"` // nil for notifications
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
	ID      *int            `json:"id"`
}

// Error represents a JSON-RPC 2.0 error.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

// Standard JSON-RPC 2.0 error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// Application-specific error codes
const (
	ServiceNotFound = -32000
	ServiceError    = -32001
)

// NewRequest creates a new JSON-RPC request.
func NewRequest(method string, params any, id int) (*Request, error) {
	req := &Request{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		ID:      &id,
	}

	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
		req.Params = data
	}

	return req, nil
}

// NewNotification creates a new JSON-RPC notification (no ID, no response expected).
func NewNotification(method string, params any) (*Request, error) {
	req := &Request{
		JSONRPC: JSONRPCVersion,
		Method:  method,
	}

	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
		req.Params = data
	}

	return req, nil
}

// NewResponse creates a successful JSON-RPC response.
func NewResponse(result any, id int) (*Response, error) {
	resp := &Response{
		JSONRPC: JSONRPCVersion,
		ID:      &id,
	}

	if result != nil {
		data, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal result: %w", err)
		}
		resp.Result = data
	}

	return resp, nil
}

// NewErrorResponse creates an error JSON-RPC response.
func NewErrorResponse(code int, message string, id *int) *Response {
	return &Response{
		JSONRPC: JSONRPCVersion,
		Error: &Error{
			Code:    code,
			Message: message,
		},
		ID: id,
	}
}

// Method names
const (
	MethodUp      = "up"
	MethodDown    = "down"
	MethodStatus  = "status"
	MethodRestart = "restart"
	MethodLogs    = "logs"
	MethodLog     = "log" // Server-sent log notification
)

// UpParams represents parameters for the "up" method.
type UpParams struct {
	Services []string `json:"services,omitempty"`
}

// DownParams represents parameters for the "down" method.
type DownParams struct {
	Services []string `json:"services,omitempty"`
}

// RestartParams represents parameters for the "restart" method.
type RestartParams struct {
	Services []string `json:"services,omitempty"`
}

// LogsParams represents parameters for the "logs" method.
type LogsParams struct {
	Services []string `json:"services,omitempty"`
	Follow   bool     `json:"follow,omitempty"`
	Lines    int      `json:"lines,omitempty"`
}

// ServiceStatus represents the status of a single service.
type ServiceStatus struct {
	Name      string `json:"name"`
	State     string `json:"state"`
	PID       int    `json:"pid,omitempty"`
	Restarts  int    `json:"restarts"`
	StartedAt string `json:"started_at,omitempty"`
	ExitCode  int    `json:"exit_code,omitempty"`
}

// StatusResult represents the result of a "status" request.
type StatusResult struct {
	Services []ServiceStatus `json:"services"`
}

// UpResult represents the result of an "up" request.
type UpResult struct {
	Started []string `json:"started,omitempty"`
	Failed  []string `json:"failed,omitempty"`
}

// DownResult represents the result of a "down" request.
type DownResult struct {
	Stopped []string `json:"stopped,omitempty"`
}

// RestartResult represents the result of a "restart" request.
type RestartResult struct {
	Restarted []string `json:"restarted,omitempty"`
	Failed    []string `json:"failed,omitempty"`
}

// LogEntry represents a single log entry sent as a notification.
type LogEntry struct {
	Service   string `json:"service"`
	Line      string `json:"line"`
	Timestamp string `json:"timestamp"`
	Stream    string `json:"stream"` // "stdout" or "stderr"
}

// ParseParams unmarshals request params into the given struct.
func (r *Request) ParseParams(v any) error {
	if r.Params == nil {
		return nil
	}
	return json.Unmarshal(r.Params, v)
}

// ParseResult unmarshals response result into the given struct.
func (r *Response) ParseResult(v any) error {
	if r.Result == nil {
		return nil
	}
	return json.Unmarshal(r.Result, v)
}
