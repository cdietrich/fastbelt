// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package parallel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A panic on a worker goroutine is raised again on the calling goroutine.
func TestForEachRaisesWorkerPanicOnCaller(t *testing.T) {
	if maxWorkers < 2 {
		t.Skip("needs GOMAXPROCS >= 2")
	}
	elements := make([]int, 1000)
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		ForEach(elements, func(_ int, i int) {
			if i == 500 {
				panic("bug in an action")
			}
		})
	}()
	require.IsType(t, &WorkerPanic{}, recovered)
	assert.Equal(t, "bug in an action", recovered.(*WorkerPanic).Value)
	assert.NotEmpty(t, recovered.(*WorkerPanic).Stack)
}
