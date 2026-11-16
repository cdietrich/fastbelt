// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package statemachine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"typefox.dev/fastbelt"
	"typefox.dev/fastbelt/textdoc"
	"typefox.dev/fastbelt/workspace"
)

// A panic in a write task does not end the process: the write releases the
// lock, and the next write builds the workspace.
func TestPanicInWriteReleasesLock(t *testing.T) {
	sc := CreateLspServices(nil)
	lock := sc.MustGet[workspace.Lock]()
	workspace.UpdateInLock(context.Background(), sc, func(context.Context) ([]textdoc.Handle, []fastbelt.URI, func()) {
		panic("bug in a write")
	})

	file := textdoc.NewFile("file:///ws/a.statemachine", "statemachine", 1, "statemachine A\n")
	workspace.UpdateInLock(context.Background(), sc, func(context.Context) ([]textdoc.Handle, []fastbelt.URI, func()) {
		return []textdoc.Handle{file}, nil, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	uri := fastbelt.ParseURI("file:///ws/a.statemachine")
	require.NoError(t, lock.ReadAt(ctx, fastbelt.DocStateValidated, []fastbelt.URI{uri}, func(context.Context) {
		assert.True(t, sc.MustGet[workspace.DocumentManager]().Get(uri).State().IsComplete())
	}))
}
