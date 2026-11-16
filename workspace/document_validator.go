// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package workspace

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"

	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/util/service"
)

// DocumentValidator validates a document's AST and collects diagnostics.
type DocumentValidator interface {
	// Validate returns diagnostics for doc. ctx may be used to cancel a
	// long-running validation, in which case the result may be incomplete.
	Validate(ctx context.Context, doc *core.Document) []*core.Diagnostic
}

// DefaultDocumentValidator is the default implementation of [DocumentValidator].
type DefaultDocumentValidator struct {
	sc *service.Container
}

// NewDefaultDocumentValidator returns a [DocumentValidator] that includes
// lexer, parser, and linker diagnostics before running AST validators.
func NewDefaultDocumentValidator(sc *service.Container) DocumentValidator {
	return &DefaultDocumentValidator{sc: sc}
}

func (s *DefaultDocumentValidator) Validate(ctx context.Context, doc *core.Document) []*core.Diagnostic {
	if doc.Root == nil {
		return nil
	}

	source := documentLanguageID(doc)

	lexerErrors := CreateLexerDiagnostics(doc, source)
	parserErrors := CreateParserDiagnostics(doc, source)
	linkerErrors := CreateLinkerDiagnostics(doc, source)

	capacity := len(lexerErrors) + len(parserErrors) + len(linkerErrors) + 8
	diagnostics := make([]*core.Diagnostic, 0, capacity)
	diagnostics = append(diagnostics, lexerErrors...)
	diagnostics = append(diagnostics, parserErrors...)
	diagnostics = append(diagnostics, linkerErrors...)

	accept := func(d *core.Diagnostic) {
		if d.Source == "" {
			d.Source = source
		}
		diagnostics = append(diagnostics, d)
	}
	// A panic of a validator (a bug in a check, or in a scope provider it
	// resolves a reference with) does not end the validation: it is logged
	// and reported as an error on the node, and the nodes after it are still
	// validated, like Langium reports a failing check. Only a panic pays for
	// the recovery: validateNodes is restarted after the failing node.
	for skip := 0; skip >= 0; {
		skip = s.validateNodes(ctx, doc, skip, accept)
	}
	return diagnostics
}

// validateNodes runs the validators of the nodes of doc, skipping the first
// skip of them. It returns -1 when it is done, or, after a validator
// panicked, the number of validators to skip to continue after it.
func (s *DefaultDocumentValidator) validateNodes(ctx context.Context, doc *core.Document, skip int, accept core.ValidationAcceptor) (next int) {
	i := 0
	// current is the node whose validator runs, nil while walking the tree.
	var current core.AstNode
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		node := current
		next = i + 1
		if node == nil {
			// The walk itself failed: report it on the root and stop, as
			// a restart would fail at the same place.
			node = doc.Root
			next = -1
		}
		log.Printf("panic in the validation of a %T in %s: %v\n%s", node, documentURI(doc), r, debug.Stack())
		accept(core.NewDiagnostic(core.SeverityError, fmt.Sprintf("An error occurred during validation: %v", r), node))
	}()
	for node := range core.AllNodes(doc.Root) {
		if ctx.Err() != nil {
			break
		}
		validator, ok := node.(core.Validator)
		if !ok {
			continue
		}
		if i >= skip {
			current = node
			validator.Validate(ctx, s.sc, accept)
			current = nil
		}
		i++
	}
	return -1
}

func documentURI(doc *core.Document) string {
	if doc.URI != nil {
		return doc.URI.String()
	}
	return "<no uri>"
}

func documentLanguageID(doc *core.Document) string {
	if doc.TextDoc != nil {
		return doc.TextDoc.LanguageID()
	}
	return ""
}
