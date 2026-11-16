// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package server

import (
	"context"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"typefox.dev/fastbelt/util/service"
	"typefox.dev/lsp"
)

// SlogHandler is a [slog.Handler] that sends log messages to the LSP client
// via the connection (window/logMessage). A record is sent as its message
// followed by its attributes as key=value pairs, e.g. "built documents=42";
// the attributes of groups are qualified with the group names, e.g.
// "build.documents=42".
//
// The handler does not make itself the process-wide default logger; call
// [SetDefaultLogger] from the server's main to send the output of the
// [slog] and [log] packages to the client.
type SlogHandler struct {
	sc    *service.Container
	level slog.Leveler
	// prefix qualifies the keys of attributes: the open groups, each
	// followed by a dot.
	prefix string
	// attrs holds the attributes of WithAttrs, formatted as " key=value".
	attrs []byte
}

// SlogHandlerOptions are the options of a [SlogHandler].
type SlogHandlerOptions struct {
	// Level is the minimum level of the records the handler sends to the
	// client. If nil, it is [slog.LevelInfo].
	Level slog.Leveler
}

// NewSlogHandler creates a new slog handler that writes records of level
// [slog.LevelInfo] and above to the LSP connection.
func NewSlogHandler(sc *service.Container) slog.Handler {
	return NewSlogHandlerWithOptions(sc, SlogHandlerOptions{})
}

// NewSlogHandlerWithOptions creates a new slog handler that writes to the
// LSP connection, configured by options.
func NewSlogHandlerWithOptions(sc *service.Container, options SlogHandlerOptions) slog.Handler {
	level := options.Level
	if level == nil {
		level = slog.LevelInfo
	}
	return &SlogHandler{sc: sc, level: level}
}

// SetDefaultLogger makes the [slog.Handler] registered in sc, by default a
// [SlogHandler], the handler of the process-wide default logger, so that the
// output of the [slog] and [log] packages is sent to the LSP client. Call it
// from the server's main, after the container is sealed; output before the
// client is connected is dropped.
func SetDefaultLogger(sc *service.Container) {
	slog.SetDefault(slog.New(sc.MustGet[slog.Handler]()))
}

func (h *SlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *SlogHandler) Handle(ctx context.Context, record slog.Record) error {
	var lspLevel lsp.MessageType
	switch {
	case record.Level < slog.LevelInfo:
		lspLevel = lsp.Log
	case record.Level < slog.LevelWarn:
		lspLevel = lsp.Info
	case record.Level < slog.LevelError:
		lspLevel = lsp.Warning
	default:
		lspLevel = lsp.Error
	}
	return h.write(ctx, lspLevel, h.message(record))
}

// message returns the text sent for record: its message and attributes.
func (h *SlogHandler) message(record slog.Record) string {
	msg := append([]byte(record.Message), h.attrs...)
	record.Attrs(func(attr slog.Attr) bool {
		msg = appendAttr(msg, h.prefix, attr)
		return true
	})
	return string(msg)
}

// WithAttrs returns a handler that adds attrs to every record it sends.
func (h *SlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	clone := *h
	clone.attrs = slices.Clip(h.attrs)
	for _, attr := range attrs {
		clone.attrs = appendAttr(clone.attrs, h.prefix, attr)
	}
	return &clone
}

// WithGroup returns a handler that qualifies the keys of the attributes
// added after it with name.
func (h *SlogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.prefix = h.prefix + name + "."
	return &clone
}

// appendAttr appends attr as " key=value" to buf, the key qualified with
// prefix; a group appends its attributes. Empty attributes and groups are
// left out, as the slog handler rules require.
func appendAttr(buf []byte, prefix string, attr slog.Attr) []byte {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return buf
	}
	if attr.Value.Kind() == slog.KindGroup {
		if attr.Key != "" {
			prefix += attr.Key + "."
		}
		for _, member := range attr.Value.Group() {
			buf = appendAttr(buf, prefix, member)
		}
		return buf
	}
	buf = append(buf, ' ')
	buf = append(buf, prefix...)
	buf = append(buf, attr.Key...)
	buf = append(buf, '=')
	value := attr.Value.String()
	if needsQuoting(value) {
		return strconv.AppendQuote(buf, value)
	}
	return append(buf, value...)
}

// needsQuoting reports whether value must be quoted to be read back as one
// value: if it is empty or contains spaces, '=', '"' or unprintable runes.
func needsQuoting(value string) bool {
	return value == "" || strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || r == '=' || r == '"' || !unicode.IsPrint(r)
	}) >= 0
}

func (h *SlogHandler) write(ctx context.Context, level lsp.MessageType, msg string) error {
	conn, err := h.sc.Get[*Connection]()
	if err != nil {
		return err
	}
	if conn.Value == nil {
		return nil // Connection not established; skip logging
	}
	lspConn := lsp.ClientDispatcher(conn.Value)
	return lspConn.LogMessage(ctx, &lsp.LogMessageParams{
		Type:    level,
		Message: msg,
	})
}
