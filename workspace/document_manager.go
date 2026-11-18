// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package workspace

import (
	"cmp"
	"iter"
	"slices"
	"sync"
	"sync/atomic"

	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/util/service"
)

// DocumentManager holds all known documents of a workspace in memory.
// All methods are safe for concurrent use.
type DocumentManager interface {
	// Has reports whether a document with the given URI is managed.
	Has(uri core.URI) bool
	// Get returns the document for uri, or nil if it is not managed.
	Get(uri core.URI) *core.Document
	// Set adds or replaces a document keyed by document.URI.
	Set(document *core.Document)
	// All returns every managed document. The order is implementation-defined;
	// [DefaultDocumentManager] orders the documents by URI.
	All() iter.Seq[*core.Document]
	// Delete removes the document for uri and returns it, or nil if it was not managed.
	Delete(uri core.URI) *core.Document
	// Clear removes every document from the manager.
	Clear()
}

// DefaultDocumentManager is the default implementation of [DocumentManager].
//
// All yields the documents ordered by URI, so that everything built on it,
// such as the default global scope, sees the documents in the same order in
// every run.
type DefaultDocumentManager struct {
	sc        *service.Container
	documents sync.Map
	// version counts the changes; every change increments it after changing
	// documents.
	version atomic.Uint64
	// sorted is the last snapshot of the documents ordered by URI. It is only
	// used while its version is the current one.
	sorted atomic.Pointer[sortedDocuments]
	// mu lets only one caller of All rebuild an outdated snapshot at a time.
	mu sync.Mutex
}

// sortedDocuments is a snapshot of the documents ordered by URI, taken at a
// version of the [DefaultDocumentManager].
type sortedDocuments struct {
	version uint64
	docs    []*core.Document
}

// NewDefaultDocumentManager returns a [DocumentManager] backed by a concurrent map.
func NewDefaultDocumentManager(sc *service.Container) DocumentManager {
	return &DefaultDocumentManager{
		sc: sc,
	}
}

func (d *DefaultDocumentManager) Has(uri core.URI) bool {
	_, exists := d.documents.Load(uri.StringUnencoded())
	return exists
}

func (d *DefaultDocumentManager) Get(uri core.URI) *core.Document {
	value, _ := d.documents.Load(uri.StringUnencoded())
	if doc, ok := value.(*core.Document); ok {
		return doc
	}
	return nil
}

func (d *DefaultDocumentManager) Set(document *core.Document) {
	d.documents.Store(document.URI.StringUnencoded(), document)
	d.version.Add(1)
}

// All yields the documents ordered by URI. It iterates a snapshot taken when
// All is called; changes during the iteration are not reflected.
func (d *DefaultDocumentManager) All() iter.Seq[*core.Document] {
	docs := d.sortedDocuments()
	return func(yield func(*core.Document) bool) {
		for _, doc := range docs {
			if !yield(doc) {
				return
			}
		}
	}
}

func (d *DefaultDocumentManager) Delete(uri core.URI) *core.Document {
	document, loaded := d.documents.LoadAndDelete(uri.StringUnencoded())
	if loaded {
		d.version.Add(1)
	}
	if doc, ok := document.(*core.Document); ok {
		return doc
	}
	return nil
}

func (d *DefaultDocumentManager) Clear() {
	d.documents.Clear()
	d.version.Add(1)
}

// sortedDocuments returns the documents ordered by URI, rebuilding the
// snapshot if a change has made it outdated.
//
// A snapshot is stored with the version read before the documents were read.
// Every change increments the version after changing the documents, so a
// snapshot that may miss a change has an older version and is never used.
func (d *DefaultDocumentManager) sortedDocuments() []*core.Document {
	if s := d.sorted.Load(); s != nil && s.version == d.version.Load() {
		return s.docs
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	version := d.version.Load()
	if s := d.sorted.Load(); s != nil && s.version == version {
		return s.docs
	}
	type entry struct {
		key string
		doc *core.Document
	}
	var entries []entry
	d.documents.Range(func(key, value any) bool {
		if doc, ok := value.(*core.Document); ok {
			entries = append(entries, entry{key.(string), doc})
		}
		return true
	})
	slices.SortFunc(entries, func(a, b entry) int { return cmp.Compare(a.key, b.key) })
	docs := make([]*core.Document, len(entries))
	for i, e := range entries {
		docs[i] = e.doc
	}
	d.sorted.Store(&sortedDocuments{version: version, docs: docs})
	return docs
}
