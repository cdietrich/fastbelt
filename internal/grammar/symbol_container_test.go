// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package grammar

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	core "typefox.dev/fastbelt"
)

// mergedTypeContainer returns a container in which ForType(AbstractRule)
// merges two slices: the AbstractTokenRules and the AbstractRules.
func mergedTypeContainer() (*FastbeltSymbolContainer, []*core.SymbolDescription) {
	a, b, c := &core.SymbolDescription{Name: "a"}, &core.SymbolDescription{Name: "b"}, &core.SymbolDescription{Name: "c"}
	sc := &FastbeltSymbolContainer{
		AbstractTokenRules: []*core.SymbolDescription{a, b},
		AbstractRules:      []*core.SymbolDescription{c},
	}
	return sc, []*core.SymbolDescription{a, b, c}
}

func TestForTypeOfMergedTypeYieldsTheSlicesInOrder(t *testing.T) {
	sc, expected := mergedTypeContainer()
	assert.Equal(t, expected, slices.Collect(sc.ForType(TypeFor_AbstractRule)))
}

func TestForTypeOfMergedTypeStopsWhenTheLoopBreaks(t *testing.T) {
	sc, _ := mergedTypeContainer()
	count := 0
	for range sc.ForType(TypeFor_AbstractRule) {
		count++
		break
	}
	assert.Equal(t, 1, count)
}

// TestForTypeOfMergedTypeAllocationsDoNotGrowWithTheSlices guards against
// chaining one iterator per slice again (10 allocations here, for two slices).
func TestForTypeOfMergedTypeAllocationsDoNotGrowWithTheSlices(t *testing.T) {
	sc, _ := mergedTypeContainer()
	allocs := testing.AllocsPerRun(100, func() {
		for range sc.ForType(TypeFor_AbstractRule) {
		}
	})
	assert.LessOrEqual(t, allocs, 3.0)
}
