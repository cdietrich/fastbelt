// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package statemachine

import (
	"reflect"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"typefox.dev/fastbelt"
	"typefox.dev/fastbelt/workspace"
)

func rootSymbols(t *testing.T) fastbelt.SymbolContainer {
	t.Helper()
	content, _ := generateStatemachineContent(0)
	srv := CreateServices()
	doc := fastbelt.NewDocumentFromString("file:///workspace/statemachine_0.statemachine", "statemachine", content)
	require.NoError(t, srv.MustGet[workspace.Builder]().Build(t.Context(), []*fastbelt.Document{doc}))
	return doc.LocalSymbols.For(doc.Root)
}

func TestSymbolContainerAllYieldsEverySliceInOrder(t *testing.T) {
	container := rootSymbols(t)
	sliceContainer, ok := container.(fastbelt.SymbolSliceContainer)
	require.True(t, ok)
	var expected []*fastbelt.SymbolDescription
	for _, typ := range []reflect.Type{TypeFor_State, TypeFor_Event, TypeFor_Command} {
		slice, ok := sliceContainer.ForTypeSlice(typ)
		require.True(t, ok)
		require.NotEmpty(t, slice)
		expected = append(expected, slice...)
	}
	assert.Equal(t, expected, slices.Collect(container.All()))
}

func TestSymbolContainerAllStopsWhenTheLoopBreaks(t *testing.T) {
	container := rootSymbols(t)
	count := 0
	for range container.All() {
		count++
		if count == 2 {
			break
		}
	}
	assert.Equal(t, 2, count)
}

// TestSymbolContainerAllAllocationsDoNotGrowWithTheSlices guards against
// chaining one iterator per slice again: that allocated about four objects per
// slice of the container on every call (13 here, for three slices). Now a loop
// allocates the returned iterator and the loop's own closure and state only.
func TestSymbolContainerAllAllocationsDoNotGrowWithTheSlices(t *testing.T) {
	container := rootSymbols(t)
	allocs := testing.AllocsPerRun(100, func() {
		for range container.All() {
		}
	})
	assert.LessOrEqual(t, allocs, 3.0)
}
