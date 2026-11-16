// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package statemachine

import (
	"context"
	"iter"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/lexer"
	"typefox.dev/fastbelt/linking"
	"typefox.dev/fastbelt/parser"
	"typefox.dev/fastbelt/util/service"
	"typefox.dev/fastbelt/workspace"
)

// isBroken reports whether a build step should panic for doc.
func isBroken(doc *core.Document) bool { return strings.Contains(doc.URI.String(), "broken") }

type brokenLexer struct{ lexer.Lexer }

func (l brokenLexer) Exec(ctx context.Context, doc *core.Document) {
	if isBroken(doc) {
		panic("bug in the lexer")
	}
	l.Lexer.Exec(ctx, doc)
}

type brokenParser struct{ parser.Parser }

func (p brokenParser) Parse(ctx context.Context, doc *core.Document) {
	if isBroken(doc) {
		panic("bug in the parser")
	}
	p.Parser.Parse(ctx, doc)
}

type brokenExporter struct{ linking.SymbolExporter }

func (e brokenExporter) ExportSymbols(ctx context.Context, doc *core.Document) core.SymbolContainer {
	if isBroken(doc) {
		panic("bug in the exporter")
	}
	return e.SymbolExporter.ExportSymbols(ctx, doc)
}

type brokenImporter struct{ linking.SymbolImporter }

func (i brokenImporter) ImportSymbols(ctx context.Context, doc *core.Document, all iter.Seq[*core.Document]) core.SymbolContainer {
	if isBroken(doc) {
		panic("bug in the importer")
	}
	return i.SymbolImporter.ImportSymbols(ctx, doc, all)
}

type brokenLocalSymbols struct{ linking.LocalSymbolsProvider }

func (l brokenLocalSymbols) LocalSymbols(ctx context.Context, doc *core.Document) core.LocalSymbols {
	if isBroken(doc) {
		panic("bug in the local symbols")
	}
	return l.LocalSymbolsProvider.LocalSymbols(ctx, doc)
}

type brokenLinker struct{ linking.Linker }

func (l brokenLinker) Link(ctx context.Context, doc *core.Document) {
	if isBroken(doc) {
		panic("bug in the linker")
	}
	l.Linker.Link(ctx, doc)
}

type brokenReferenceDescriptions struct {
	linking.ReferenceDescriptionsProvider
}

func (r brokenReferenceDescriptions) ReferenceDescriptions(ctx context.Context, doc *core.Document) core.ReferenceDescriptions {
	if isBroken(doc) {
		panic("bug in the reference descriptions")
	}
	return r.ReferenceDescriptionsProvider.ReferenceDescriptions(ctx, doc)
}

type brokenValidator struct{ workspace.DocumentValidator }

func (v brokenValidator) Validate(ctx context.Context, doc *core.Document) []*core.Diagnostic {
	if isBroken(doc) {
		panic("bug in the validator")
	}
	return v.DocumentValidator.Validate(ctx, doc)
}

// A panic of a build step for one document becomes an error of that
// document (or is only logged), and the build of every document completes.
func TestBuildStepPanicIsReportedOnItsDocument(t *testing.T) {
	tests := []struct {
		step  string
		setup func(sc *service.Container)
		// want is the error reported on the broken document, "" if the
		// panic is only logged.
		want string
	}{
		{"lexer", func(sc *service.Container) { sc.Put[lexer.Lexer](brokenLexer{NewLexer(sc)}) },
			"An error occurred during lexing: bug in the lexer"},
		{"parser", func(sc *service.Container) { sc.Put[parser.Parser](brokenParser{NewParser(sc)}) },
			"An error occurred during parsing: bug in the parser"},
		{"exporter", func(sc *service.Container) {
			sc.Put[linking.SymbolExporter](brokenExporter{linking.NewDefaultSymbolExporter(sc)})
		}, ""},
		{"importer", func(sc *service.Container) {
			sc.Put[linking.SymbolImporter](brokenImporter{linking.NewDefaultSymbolImporter(sc)})
		}, ""},
		{"local symbols", func(sc *service.Container) {
			sc.Put[linking.LocalSymbolsProvider](brokenLocalSymbols{linking.NewDefaultLocalSymbolsProvider(sc)})
		}, ""},
		{"linker", func(sc *service.Container) { sc.Put[linking.Linker](brokenLinker{linking.NewDefaultLinker(sc)}) }, ""},
		{"reference descriptions", func(sc *service.Container) {
			sc.Put[linking.ReferenceDescriptionsProvider](brokenReferenceDescriptions{linking.NewDefaultReferenceDescriptionsProvider(sc)})
		}, ""},
		{"validator", func(sc *service.Container) {
			sc.Put[workspace.DocumentValidator](brokenValidator{workspace.NewDefaultDocumentValidator(sc)})
		}, "An error occurred during validation: bug in the validator"},
	}
	const text = "statemachine M\nevents go\ninitialState a\nstate a\n  go => a\nend\n"
	for _, tt := range tests {
		t.Run(tt.step, func(t *testing.T) {
			sc := service.NewContainer()
			tt.setup(sc)
			SetupServices(sc)
			sc.Seal()
			good := core.NewDocumentFromString("file:///ws/good.statemachine", "statemachine", text)
			broken := core.NewDocumentFromString("file:///ws/broken.statemachine", "statemachine", text)
			docs := sc.MustGet[workspace.DocumentManager]()
			docs.Set(good)
			docs.Set(broken)

			require.NoError(t, sc.MustGet[workspace.Builder]().Build(context.Background(), []*core.Document{good, broken}))

			for _, doc := range []*core.Document{good, broken} {
				assert.True(t, doc.State().IsComplete(), "%s is not built", doc.URI)
			}
			assert.Empty(t, good.Diagnostics, "the good document has diagnostics")
			if tt.want != "" {
				var messages []string
				for _, d := range broken.Diagnostics {
					messages = append(messages, d.Message)
				}
				assert.Contains(t, messages, tt.want)
			}
		})
	}
}

// A panicking build step listener is logged like a listener error: the
// other listeners are still called and the build completes.
func TestBuildStepListenerPanic(t *testing.T) {
	sc := CreateServices()
	builder := sc.MustGet[workspace.Builder]()
	builder.AddBuildStepListener(core.DocStateValidated, func(context.Context, *core.Document) error {
		panic("bug in a listener")
	})
	called := 0
	builder.AddBuildStepListener(core.DocStateValidated, func(context.Context, *core.Document) error {
		called++
		return nil
	})
	doc := core.NewDocumentFromString("file:///ws/a.statemachine", "statemachine", "statemachine M\n")
	sc.MustGet[workspace.DocumentManager]().Set(doc)

	require.NoError(t, builder.Build(context.Background(), []*core.Document{doc}))
	assert.True(t, doc.State().IsComplete())
	assert.Equal(t, 1, called)
}
