package mcp

import (
	"encoding/json"
	"testing"
)

func TestJSONRPCRequestMarshal(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      int64(1),
		Method:  MethodToolsList,
		Params:  json.RawMessage(`{"name":"test"}`),
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("expected non-empty JSON")
	}
}

func TestJSONRPCResponseUnmarshal(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`
	var resp JSONRPCResponse
	if err := json.Unmarshal([]byte(input), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.JSONRPC != "2.0" {
		t.Fatalf("expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}
}
