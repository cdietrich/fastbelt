// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package statemachine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"typefox.dev/fastbelt/server"
	"typefox.dev/lsp"
)

// The connection counts as initialized once the server handled the
// initialize request, before the initialize participants run.
func TestConnectionInitializedByInitialize(t *testing.T) {
	sc := CreateLspServices(nil)
	connection := sc.MustGet[*server.Connection]()
	assert.False(t, connection.Initialized())
	_, err := sc.MustGet[lsp.Server]().Initialize(context.Background(), &lsp.ParamInitialize{})
	require.NoError(t, err)
	assert.True(t, connection.Initialized())
}
