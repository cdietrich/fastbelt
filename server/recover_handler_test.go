// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package server_test

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/exp/jsonrpc2"
	"typefox.dev/fastbelt/server"
)

// A panic while handling a request becomes an internal error reply; a
// handler that does not panic is unaffected.
func TestRecoverHandler(t *testing.T) {
	handler := server.RecoverHandler(jsonrpc2.HandlerFunc(func(_ context.Context, req *jsonrpc2.Request) (any, error) {
		if req.Method == "textDocument/hover" {
			panic("bug in a handler")
		}
		return "ok", nil
	}))
	req, err := jsonrpc2.NewCall(jsonrpc2.Int64ID(1), "textDocument/hover", nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := handler.Handle(context.Background(), req)
	if result != nil || !errors.Is(err, jsonrpc2.ErrInternal) {
		t.Fatalf("result %v, error %v", result, err)
	}
	if want := `JSON RPC internal error: "textDocument/hover" panicked: bug in a handler`; err.Error() != want {
		t.Errorf("error %q, want %q", err, want)
	}
	req, _ = jsonrpc2.NewCall(jsonrpc2.Int64ID(2), "textDocument/definition", nil)
	if result, err := handler.Handle(context.Background(), req); result != "ok" || err != nil {
		t.Errorf("result %v, error %v", result, err)
	}
}
