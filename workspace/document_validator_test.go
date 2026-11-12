// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package workspace

import (
	"context"
	"testing"
	"unique"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/textdoc"
	"typefox.dev/fastbelt/util/service"
)

var childrenField = unique.Make("children")

// validatorTestRoot is a root node with child nodes.
// If brokenWalk is set, walking past its first child panics.
type validatorTestRoot struct {
	core.AstNodeBase
	children   []core.AstNode
	brokenWalk bool
}

func (n *validatorTestRoot) ForEachNode(fn func(core.AstNode, unique.Handle[string], int)) {
	for i, child := range n.children {
		if n.brokenWalk && i == 1 {
			panic("broken walk")
		}
		fn(child, childrenField, i)
	}
}

// validatorTestNode reports a warning, or panics if panics is set.
type validatorTestNode struct {
	core.AstNodeBase
	panics bool
}

func (n *validatorTestNode) Validate(_ context.Context, _ *service.Container, accept core.ValidationAcceptor) {
	if n.panics {
		panic("bug in a check")
	}
	accept(core.NewDiagnostic(core.SeverityWarning, "checked", n))
}

// A panic of a node's validator becomes an error on that node, and the other
// nodes are still validated, also after consecutive panics and when the first
// or the last node panics.
func TestValidatorPanicBecomesDiagnostic(t *testing.T) {
	root := &validatorTestRoot{}
	panics := []bool{true, false, true, true, false, true}
	for i, p := range panics {
		child := &validatorTestNode{panics: p}
		child.SetTextRange(core.NewTextRange(i*2, i*2+1))
		child.SetContainer(root, childrenField, i)
		root.children = append(root.children, child)
	}
	doc := core.NewDocument(textdoc.NewFile("file:///test.txt", "test", 1, "a b c d e f"))
	doc.Root = root

	diagnostics := NewDefaultDocumentValidator(service.NewContainer()).Validate(context.Background(), doc)

	require.Len(t, diagnostics, len(panics))
	for i, p := range panics {
		d := diagnostics[i]
		assert.Equal(t, core.NewTextRange(i*2, i*2+1), d.Range, "node %d", i)
		assert.Equal(t, "test", d.Source, "node %d", i)
		if p {
			assert.Equal(t, core.SeverityError, d.Severity, "node %d", i)
			assert.Equal(t, "An error occurred during validation: bug in a check", d.Message, "node %d", i)
		} else {
			assert.Equal(t, "checked", d.Message, "node %d", i)
		}
	}
}

// A panic while walking the tree (not in a validator) is reported on the
// root, and the validation stops.
func TestValidationWalkPanicBecomesDiagnostic(t *testing.T) {
	root := &validatorTestRoot{brokenWalk: true}
	root.SetTextRange(core.NewTextRange(0, 3))
	for i := range 2 {
		child := &validatorTestNode{}
		child.SetTextRange(core.NewTextRange(i*2, i*2+1))
		child.SetContainer(root, childrenField, i)
		root.children = append(root.children, child)
	}
	doc := core.NewDocument(textdoc.NewFile("file:///test.txt", "test", 1, "a b"))
	doc.Root = root

	diagnostics := NewDefaultDocumentValidator(service.NewContainer()).Validate(context.Background(), doc)

	require.Len(t, diagnostics, 2)
	assert.Equal(t, "checked", diagnostics[0].Message)
	assert.Equal(t, "An error occurred during validation: broken walk", diagnostics[1].Message)
	assert.Equal(t, core.NewTextRange(0, 3), diagnostics[1].Range)
}
