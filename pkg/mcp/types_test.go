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
			if tt.name == "full request marshal/unmarshal" {
				// encoding/json unmarshals numbers into float64 when the target type is any.
				if got.ID != float64(1) {
					t.Fatalf("id = %v (type %T), want float64(1)", got.ID, got.ID)
				}
				if string(got.Params) != `{"name":"test"}` {
					t.Fatalf("params = %s, want {\"name\":\"test\"}", got.Params)
				}
			}
			if tt.name == "notification nil id" {
				if got.ID != nil {
					t.Fatalf("id = %v, want nil", got.ID)
				}
				if got.Params != nil {
					t.Fatalf("params = %s, want nil", got.Params)
				}
			}
			if tt.name == "string id" {
				if got.ID != "abc" {
					t.Fatalf("id = %v, want abc", got.ID)
				}
				if got.Params != nil {
					t.Fatalf("params = %s, want nil", got.Params)
				}
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
	tests := []struct {
		name    string
		errObj  *JSONRPCError
		want    string
		wantErr bool
		check   func(t *testing.T, got JSONRPCError)
	}{
		{
			name: "standard marshal/unmarshal",
			errObj: &JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    json.RawMessage(`{"detail":"missing method"}`),
			},
			want: `{"code":-32600,"message":"Invalid Request","data":{"detail":"missing method"}}`,
			check: func(t *testing.T, got JSONRPCError) {
				if got.Code != -32600 {
					t.Fatalf("code = %d, want -32600", got.Code)
				}
				if got.Message != "Invalid Request" {
					t.Fatalf("message = %s, want Invalid Request", got.Message)
				}
				if string(got.Data) != `{"detail":"missing method"}` {
					t.Fatalf("data = %s, want {\"detail\":\"missing method\"}", got.Data)
				}
			},
		},
		{
			name:    "nil receiver Error() does not panic",
			errObj:  nil,
			wantErr: true,
			check: func(t *testing.T, got JSONRPCError) {
				var e *JSONRPCError
				if e.Error() != "" {
					t.Fatalf("expected empty string from nil receiver, got %q", e.Error())
				}
			},
		},
		{
			name: "nil Data omitempty",
			errObj: &JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
			},
			want: `{"code":-32601,"message":"Method not found"}`,
			check: func(t *testing.T, got JSONRPCError) {
				if got.Data != nil {
					t.Fatalf("data = %s, want nil", got.Data)
				}
			},
		},
		{
			name: "boundary code 0",
			errObj: &JSONRPCError{
				Code:    0,
				Message: "zero code",
			},
			want: `{"code":0,"message":"zero code"}`,
			check: func(t *testing.T, got JSONRPCError) {
				if got.Code != 0 {
					t.Fatalf("code = %d, want 0", got.Code)
				}
			},
		},
		{
			name: "boundary code -32700",
			errObj: &JSONRPCError{
				Code:    -32700,
				Message: "Parse error",
			},
			want: `{"code":-32700,"message":"Parse error"}`,
			check: func(t *testing.T, got JSONRPCError) {
				if got.Code != -32700 {
					t.Fatalf("code = %d, want -32700", got.Code)
				}
			},
		},
		{
			name: "boundary code 32000",
			errObj: &JSONRPCError{
				Code:    32000,
				Message: "Server error",
			},
			want: `{"code":32000,"message":"Server error"}`,
			check: func(t *testing.T, got JSONRPCError) {
				if got.Code != 32000 {
					t.Fatalf("code = %d, want 32000", got.Code)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.errObj == nil {
				if tt.check != nil {
					tt.check(t, JSONRPCError{})
				}
				return
			}

			b, err := json.Marshal(tt.errObj)
			if (err != nil) != tt.wantErr {
				t.Fatalf("marshal error = %v, wantErr %v", err, tt.wantErr)
			}
			if string(b) != tt.want {
				t.Fatalf("marshal = %s, want %s", b, tt.want)
			}

			var got JSONRPCError
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
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

func TestRequestIDString(t *testing.T) {
	tests := []struct {
		name string
		id   RequestID
		want string
	}{
		{"nil", RequestID{Value: nil}, "null"},
		{"int", RequestID{Value: 42}, "42"},
		{"float64", RequestID{Value: float64(42)}, "42"},
		{"string", RequestID{Value: "abc"}, "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRequestIDEqual(t *testing.T) {
	tests := []struct {
		name string
		a    RequestID
		b    RequestID
		want bool
	}{
		{"both nil", RequestID{Value: nil}, RequestID{Value: nil}, true},
		{"a nil", RequestID{Value: nil}, RequestID{Value: 1}, false},
		{"b nil", RequestID{Value: 1}, RequestID{Value: nil}, false},
		{"same int", RequestID{Value: 1}, RequestID{Value: 1}, true},
		{"int vs float64", RequestID{Value: 1}, RequestID{Value: float64(1)}, true},
		{"same string", RequestID{Value: "abc"}, RequestID{Value: "abc"}, true},
		{"different string", RequestID{Value: "abc"}, RequestID{Value: "def"}, false},
		{"different numbers", RequestID{Value: 1}, RequestID{Value: 2}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equal(tt.b); got != tt.want {
				t.Fatalf("Equal(%v, %v) = %v, want %v", tt.a.Value, tt.b.Value, got, tt.want)
			}
		})
	}
}
