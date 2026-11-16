// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package server

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"typefox.dev/fastbelt/util/service"
)

func record(msg string, attrs ...slog.Attr) slog.Record {
	r := slog.NewRecord(time.Time{}, slog.LevelInfo, msg, 0)
	r.AddAttrs(attrs...)
	return r
}

// A record is sent with its attributes, those of WithAttrs first, and the
// keys of groups are qualified with the group names.
func TestSlogHandlerSendsAttributes(t *testing.T) {
	h := NewSlogHandler(service.NewContainer()).(*SlogHandler)
	assert.Equal(t, "built documents=42 in=3ms", h.message(record("built", slog.Int("documents", 42), slog.Duration("in", 3*time.Millisecond))))
	assert.Equal(t, `failed file="a b.txt" empty=""`, h.message(record("failed", slog.String("file", "a b.txt"), slog.String("empty", ""))))

	withAttrs := h.WithAttrs([]slog.Attr{slog.String("server", "x")}).(*SlogHandler)
	assert.Equal(t, "built server=x documents=1", withAttrs.message(record("built", slog.Int("documents", 1))))
	// WithAttrs does not change the handler it was called on.
	assert.Equal(t, "built", h.message(record("built")))

	grouped := withAttrs.WithGroup("build").WithAttrs([]slog.Attr{slog.Int("round", 2)}).(*SlogHandler)
	assert.Equal(t, "built server=x build.round=2 build.documents=1 build.phase.name=link",
		grouped.message(record("built", slog.Int("documents", 1), slog.Group("phase", slog.String("name", "link")), slog.Group("empty"), slog.Attr{})))
}

// By default, records below Info are not sent; the level is configurable.
func TestSlogHandlerLevel(t *testing.T) {
	ctx := context.Background()
	h := NewSlogHandler(service.NewContainer())
	assert.False(t, h.Enabled(ctx, slog.LevelDebug))
	assert.True(t, h.Enabled(ctx, slog.LevelInfo))
	debug := NewSlogHandlerWithOptions(service.NewContainer(), SlogHandlerOptions{Level: slog.LevelDebug})
	assert.True(t, debug.Enabled(ctx, slog.LevelDebug))
}

// Initializing a server does not change the process-wide default logger.
func TestSlogHandlerIsNotTheDefaultLogger(t *testing.T) {
	h := NewSlogHandler(service.NewContainer())
	_, participates := h.(InitializeParticipant)
	assert.False(t, participates)
}
