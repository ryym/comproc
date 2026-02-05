package protocol

import (
	"encoding/json"
	"testing"
)

func TestNewRequest(t *testing.T) {
	params := UpParams{Services: []string{"api", "db"}}
	req, err := NewRequest(MethodUp, params, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.JSONRPC != JSONRPCVersion {
		t.Errorf("expected jsonrpc %q, got %q", JSONRPCVersion, req.JSONRPC)
	}
	if req.Method != MethodUp {
		t.Errorf("expected method %q, got %q", MethodUp, req.Method)
	}
	if req.ID == nil || *req.ID != 1 {
		t.Errorf("expected id 1, got %v", req.ID)
	}

	// Check params can be parsed back
	var parsed UpParams
	if err := req.ParseParams(&parsed); err != nil {
		t.Fatalf("failed to parse params: %v", err)
	}
	if len(parsed.Services) != 2 || parsed.Services[0] != "api" {
		t.Errorf("expected services [api, db], got %v", parsed.Services)
	}
}

func TestNewNotification(t *testing.T) {
	entry := LogEntry{
		Service: "api",
		Line:    "Server started",
		Stream:  "stdout",
	}
	req, err := NewNotification(MethodLog, entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.ID != nil {
		t.Errorf("expected nil id for notification, got %v", req.ID)
	}
	if req.Method != MethodLog {
		t.Errorf("expected method %q, got %q", MethodLog, req.Method)
	}
}

func TestNewResponse(t *testing.T) {
	result := StatusResult{
		Services: []ServiceStatus{
			{Name: "api", State: "running", PID: 1234},
		},
	}
	resp, err := NewResponse(result, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.JSONRPC != JSONRPCVersion {
		t.Errorf("expected jsonrpc %q, got %q", JSONRPCVersion, resp.JSONRPC)
	}
	if resp.ID == nil || *resp.ID != 1 {
		t.Errorf("expected id 1, got %v", resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("expected nil error, got %v", resp.Error)
	}

	var parsed StatusResult
	if err := resp.ParseResult(&parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if len(parsed.Services) != 1 || parsed.Services[0].Name != "api" {
		t.Errorf("expected service api, got %v", parsed.Services)
	}
}

func TestNewErrorResponse(t *testing.T) {
	id := 1
	resp := NewErrorResponse(ServiceNotFound, "service not found: api", &id)

	if resp.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if resp.Error.Code != ServiceNotFound {
		t.Errorf("expected code %d, got %d", ServiceNotFound, resp.Error.Code)
	}
	if resp.Error.Message != "service not found: api" {
		t.Errorf("expected message 'service not found: api', got %q", resp.Error.Message)
	}
	if resp.Result != nil {
		t.Errorf("expected nil result, got %v", resp.Result)
	}
}

func TestRequest_JSONMarshaling(t *testing.T) {
	params := UpParams{Services: []string{"api"}}
	req, _ := NewRequest(MethodUp, params, 1)

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var parsed Request
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if parsed.Method != MethodUp {
		t.Errorf("expected method %q, got %q", MethodUp, parsed.Method)
	}
	if parsed.ID == nil || *parsed.ID != 1 {
		t.Errorf("expected id 1, got %v", parsed.ID)
	}
}

func TestResponse_JSONMarshaling(t *testing.T) {
	result := UpResult{Started: []string{"api", "db"}}
	resp, _ := NewResponse(result, 1)

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed Response
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if parsed.JSONRPC != JSONRPCVersion {
		t.Errorf("expected jsonrpc %q, got %q", JSONRPCVersion, parsed.JSONRPC)
	}
}

func TestError_ErrorInterface(t *testing.T) {
	err := &Error{
		Code:    ServiceNotFound,
		Message: "service not found",
	}

	expected := "RPC error -32000: service not found"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestParseParams_NilParams(t *testing.T) {
	req := &Request{
		JSONRPC: JSONRPCVersion,
		Method:  MethodStatus,
	}

	var params UpParams
	if err := req.ParseParams(&params); err != nil {
		t.Errorf("unexpected error for nil params: %v", err)
	}
}

func TestParseResult_NilResult(t *testing.T) {
	id := 1
	resp := &Response{
		JSONRPC: JSONRPCVersion,
		ID:      &id,
	}

	var result StatusResult
	if err := resp.ParseResult(&result); err != nil {
		t.Errorf("unexpected error for nil result: %v", err)
	}
}
