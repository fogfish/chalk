//
// Copyright (C) 2026 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/chalk
//

package chalk

import (
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/charmbracelet/x/term"
)

// config holds the resolved configuration built from Options.
type config struct {
	out     io.Writer
	tty     *bool // nil = auto-detect
	color   bool
	silent  bool
	printer Printer
	logger  *slog.Logger
}

// Option configures a Reporter built by New.
type Option func(*config)

// WithOutput sets the writer used by the TTY/log printers. Defaults to
// os.Stderr.
func WithOutput(w io.Writer) Option {
	return func(c *config) { c.out = w }
}

// WithTTY forces TTY (spinner + colours) mode on or off, skipping
// auto-detection of the output stream.
func WithTTY(enabled bool) Option {
	return func(c *config) { c.tty = &enabled }
}

// WithColor enables or disables ANSI colour in TTY mode. Defaults to true.
func WithColor(enabled bool) Option {
	return func(c *config) { c.color = enabled }
}

// WithSilent disables all output, including errors.
func WithSilent() Option {
	return func(c *config) { c.silent = true }
}

// WithPrinter overrides the printer selection entirely with p.
func WithPrinter(p Printer) Option {
	return func(c *config) { c.printer = p }
}

// WithLogger sets the slog.Logger used by the log printer. Defaults to
// slog.Default().
func WithLogger(l *slog.Logger) Option {
	return func(c *config) { c.logger = l }
}

// New builds a Reporter. With no options it auto-detects: ttyPrinter on
// os.Stderr if it's an interactive terminal, logPrinter otherwise.
func New(opts ...Option) *Reporter {
	cfg := config{
		out:   os.Stderr,
		color: true,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	r := &Reporter{}

	switch {
	case cfg.printer != nil:
		r.p = cfg.printer
	case cfg.silent:
		r.p = &silentPrinter{}
	case cfg.tty != nil && *cfg.tty:
		r.p = newTTYPrinter(cfg.out, time.Now(), &r.mu, cfg.color)
	case cfg.tty != nil && !*cfg.tty:
		r.p = newLogPrinter(cfg.logger)
	case isTerminal(cfg.out):
		r.p = newTTYPrinter(cfg.out, time.Now(), &r.mu, cfg.color)
	default:
		r.p = newLogPrinter(cfg.logger)
	}

	return r
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(f.Fd())
}
