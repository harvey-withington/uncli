package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
)

// JSON-RPC 2.0 over POST /rpc, dispatched by reflection to a listed set of
// methods (BRUV's transport/http/rpc.go pattern). Params are positional,
// as the Wails bridge takes them, so the frontend's HTTP backend calls the
// same names with the same arguments.

type rpcRequest struct {
	JSONRPC string            `json:"jsonrpc"`
	Method  string            `json:"method"`
	Params  []json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage   `json:"id,omitempty"`
}

type rpcResponse struct {
	Result any
	Error  *rpcError
	ID     json.RawMessage
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error codes: JSON-RPC's own, and one for a method that ran and failed
// (its message is the app's, meant for the user).
const (
	errParse          = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602
	errInternal       = -32603
	errApp            = -32000
)

// maxRPCBody caps a request: a message with pasted images still fits.
const maxRPCBody = 16 << 20

var (
	errorType   = reflect.TypeOf((*error)(nil)).Elem()
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
)

// dispatch calls the named method on target, which must be one of names.
// Each request gets its own target (it carries the device), so methods
// are looked up per call; the list is what keeps the rest unreachable.
func dispatch(ctx context.Context, target any, names map[string]bool, method string, raw []json.RawMessage) (any, *rpcError) {
	if !names[method] {
		return nil, &rpcError{Code: errMethodNotFound, Message: "this server doesn't offer " + method}
	}
	fn := reflect.ValueOf(target).MethodByName(method)
	if !fn.IsValid() {
		return nil, &rpcError{Code: errMethodNotFound, Message: "this server doesn't offer " + method}
	}
	ft := fn.Type()
	first := 0
	if ft.NumIn() > 0 && ft.In(0) == contextType {
		first = 1
	}
	if want := ft.NumIn() - first; len(raw) != want {
		return nil, &rpcError{Code: errInvalidParams, Message: fmt.Sprintf("%s takes %d arguments, got %d", method, want, len(raw))}
	}
	args := make([]reflect.Value, ft.NumIn())
	if first == 1 {
		args[0] = reflect.ValueOf(ctx)
	}
	for i, r := range raw {
		p := reflect.New(ft.In(i + first))
		if err := json.Unmarshal(r, p.Interface()); err != nil {
			return nil, &rpcError{Code: errInvalidParams, Message: fmt.Sprintf("%s argument %d: %v", method, i+1, err)}
		}
		args[i+first] = p.Elem()
	}
	var out []reflect.Value
	var panicked *rpcError
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = &rpcError{Code: errInternal, Message: fmt.Sprintf("%s failed: %v", method, r)}
			}
		}()
		out = fn.Call(args)
	}()
	if panicked != nil {
		return nil, panicked
	}
	if n := len(out); n > 0 && ft.Out(n-1) == errorType {
		if e := out[n-1]; !e.IsNil() {
			return nil, &rpcError{Code: errApp, Message: e.Interface().(error).Error()}
		}
		out = out[:n-1]
	}
	switch len(out) {
	case 0:
		return nil, nil
	case 1:
		return out[0].Interface(), nil
	}
	res := make([]any, len(out))
	for i, v := range out {
		res[i] = v.Interface()
	}
	return res, nil
}

// serveRPC handles one JSON-RPC request for an authenticated device.
func serveRPC(w http.ResponseWriter, r *http.Request, target any, names map[string]bool) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		writeRPC(w, rpcResponse{Error: &rpcError{Code: errInvalidRequest, Message: "use POST"}})
		return
	}
	var req rpcRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRPCBody)).Decode(&req); err != nil {
		writeRPC(w, rpcResponse{Error: &rpcError{Code: errParse, Message: "invalid JSON: " + err.Error()}})
		return
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		writeRPC(w, rpcResponse{ID: req.ID, Error: &rpcError{Code: errInvalidRequest, Message: `a request needs "jsonrpc": "2.0" and a method`}})
		return
	}
	res, rerr := dispatch(r.Context(), target, names, req.Method, req.Params)
	writeRPC(w, rpcResponse{ID: req.ID, Result: res, Error: rerr})
}

// writeRPC sends a result (always present on success, even when null) or
// an error, never both.
func writeRPC(w http.ResponseWriter, resp rpcResponse) {
	id := resp.ID
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	if resp.Error != nil {
		_ = json.NewEncoder(w).Encode(struct {
			JSONRPC string          `json:"jsonrpc"`
			Error   *rpcError       `json:"error"`
			ID      json.RawMessage `json:"id"`
		}{"2.0", resp.Error, id})
		return
	}
	_ = json.NewEncoder(w).Encode(struct {
		JSONRPC string          `json:"jsonrpc"`
		Result  any             `json:"result"`
		ID      json.RawMessage `json:"id"`
	}{"2.0", resp.Result, id})
}
