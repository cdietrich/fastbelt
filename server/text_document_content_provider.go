// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package server

import (
	"context"
	"fmt"
	"slices"

	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/util/service"
	"typefox.dev/fastbelt/workspace"
	"typefox.dev/lsp"
)

// TextDocumentContentProvider provides the content of documents whose URI
// schemes the client cannot read itself, e.g. built-in libraries that only
// exist in the language server (LSP 3.18 `workspace/textDocumentContent`).
// The client shows such documents read-only, for example when
// go-to-definition targets them.
//
// No provider is registered by default; register one to announce the
// capability for its [TextDocumentContentProvider.Schemes].
type TextDocumentContentProvider interface {
	// Schemes returns the URI schemes the provider serves.
	Schemes() []string
	HandleTextDocumentContentRequest(ctx context.Context, params *lsp.TextDocumentContentParams) (*lsp.TextDocumentContentResult, error)
}

// DefaultTextDocumentContentProvider serves the text of the documents in the
// [workspace.DocumentManager] whose URI has one of its schemes.
type DefaultTextDocumentContentProvider struct {
	sc      *service.Container
	schemes []string
}

var _ TextDocumentContentProvider = (*DefaultTextDocumentContentProvider)(nil)
var _ DocumentStateRequirements = (*DefaultTextDocumentContentProvider)(nil)

// NewDefaultTextDocumentContentProvider creates a provider for the given
// URI schemes, e.g. "builtin".
func NewDefaultTextDocumentContentProvider(sc *service.Container, schemes ...string) TextDocumentContentProvider {
	return &DefaultTextDocumentContentProvider{sc: sc, schemes: schemes}
}

func (p *DefaultTextDocumentContentProvider) Schemes() []string {
	return p.schemes
}

// RequiredState returns no state: the text of a document is available as
// soon as the document is.
func (p *DefaultTextDocumentContentProvider) RequiredState() (core.DocumentState, bool) {
	return 0, false
}

func (p *DefaultTextDocumentContentProvider) HandleTextDocumentContentRequest(_ context.Context, params *lsp.TextDocumentContentParams) (*lsp.TextDocumentContentResult, error) {
	uri := core.ParseURI(string(params.URI))
	if !slices.Contains(p.schemes, uri.Scheme()) {
		return nil, fmt.Errorf("the scheme of %s is not served", params.URI)
	}
	doc := p.sc.MustGet[workspace.DocumentManager]().Get(uri)
	if doc == nil {
		return nil, fmt.Errorf("no document %s", params.URI)
	}
	return &lsp.TextDocumentContentResult{Text: doc.TextDoc.Text(nil)}, nil
}
