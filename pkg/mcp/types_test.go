package mcp

import (
	"encoding/json"
	"testing"
)

func TestJSONRPCRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     JSONRPCRequest
		want    string
		wantErr bool
	}{
		{
			name: "full request marshal/unmarshal",
			req: JSONRPCRequest{
				JSONRPC: JSONRPCVersion,
				ID:      int64(1),
				Method:  MethodToolsList,
				Params:  json.RawMessage(`{"name":"test"}`),
			},
			want: `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"name":"test"}}`,
		},
		{
			name: "notification nil id",
			req: JSONRPCRequest{
				JSONRPC: JSONRPCVersion,
				ID:      nil,
				Method:  MethodInitialized,
			},
			want: `{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		},
		{
			name: "string id",
			req: JSONRPCRequest{
				JSONRPC: JSONRPCVersion,
				ID:      "abc",
				Method:  MethodPing,
			},
			want: `{"jsonrpc":"2.0","id":"abc","method":"ping"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("marshal error = %v, wantErr %v", err, tt.wantErr)
			}
			if string(b) != tt.want {
				t.Fatalf("marshal = %s, want %s", b, tt.want)
			}

			var got JSONRPCRequest
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got.JSONRPC != tt.req.JSONRPC {
				t.Fatalf("jsonrpc = %s, want %s", got.JSONRPC, tt.req.JSONRPC)
			}
			if got.Method != tt.req.Method {
				t.Fatalf("method = %s, want %s", got.Method, tt.req.Method)
			}
		})
	}
}

func TestJSONRPCResponse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, resp JSONRPCResponse)
	}{
		{
			name:  "unmarshal with result",
			input: `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`,
			check: func(t *testing.T, resp JSONRPCResponse) {
				if resp.JSONRPC != JSONRPCVersion {
					t.Fatalf("expected jsonrpc %s, got %s", JSONRPCVersion, resp.JSONRPC)
				}
				if resp.ID != float64(1) {
					t.Fatalf("expected id 1, got %v (type %T)", resp.ID, resp.ID)
				}
				if string(resp.Result) != `{"tools":[]}` {
					t.Fatalf("expected result, got %s", resp.Result)
				}
			},
		},
		{
			name:  "response with error object",
			input: `{"jsonrpc":"2.0","id":2,"error":{"code":-32600,"message":"Invalid Request"}}`,
			check: func(t *testing.T, resp JSONRPCResponse) {
				if resp.Error == nil {
					t.Fatal("expected error, got nil")
				}
				if resp.Error.Code != -32600 {
					t.Fatalf("expected code -32600, got %d", resp.Error.Code)
				}
				if resp.Error.Message != "Invalid Request" {
					t.Fatalf("expected message Invalid Request, got %s", resp.Error.Message)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp JSONRPCResponse
			if err := json.Unmarshal([]byte(tt.input), &resp); (err != nil) != tt.wantErr {
				t.Fatalf("unmarshal error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.check != nil {
				tt.check(t, resp)
			}
		})
	}
}

func TestJSONRPCError(t *testing.T) {
	errObj := &JSONRPCError{
		Code:    -32600,
		Message: "Invalid Request",
		Data:    json.RawMessage(`{"detail":"missing method"}`),
	}

	b, err := json.Marshal(errObj)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	want := `{"code":-32600,"message":"Invalid Request","data":{"detail":"missing method"}}`
	if string(b) != want {
		t.Fatalf("marshal = %s, want %s", b, want)
	}

	var got JSONRPCError
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Code != errObj.Code {
		t.Fatalf("code = %d, want %d", got.Code, errObj.Code)
	}
	if got.Message != errObj.Message {
		t.Fatalf("message = %s, want %s", got.Message, errObj.Message)
	}
	if string(got.Data) != string(errObj.Data) {
		t.Fatalf("data = %s, want %s", got.Data, errObj.Data)
	}
}

func TestJSONRPCErrorNilReceiver(t *testing.T) {
	var e *JSONRPCError
	if e.Error() != "" {
		t.Fatalf("expected empty string from nil receiver, got %q", e.Error())
	}
}

func TestJSONRPCRoundtrip(t *testing.T) {
	original := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      float64(42),
		Method:  MethodToolsCall,
		Params:  json.RawMessage(`{"name":"calculator","args":{"a":1,"b":2}}`),
	}

	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got JSONRPCRequest
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.JSONRPC != original.JSONRPC {
		t.Fatalf("jsonrpc = %s, want %s", got.JSONRPC, original.JSONRPC)
	}
	if got.ID != original.ID {
		t.Fatalf("id = %v, want %v", got.ID, original.ID)
	}
	if got.Method != original.Method {
		t.Fatalf("method = %s, want %s", got.Method, original.Method)
	}
	if string(got.Params) != string(original.Params) {
		t.Fatalf("params = %s, want %s", got.Params, original.Params)
	}
}
