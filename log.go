//
// Copyright (C) 2026 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/chalk
//

package chalk

import (
	"log/slog"
	"os"
	"time"
)

// ── logPrinter ────────────────────────────────────────────────────────────────

// logPrinter is the strategy for non-interactive environments (pipes, CI).
// It emits structured slog records — no colour, no spinner.
type logPrinter struct {
	log *slog.Logger
}

func newLogPrinter(log *slog.Logger) *logPrinter {
	if log == nil {
		log = slog.Default()
	}
	return &logPrinter{log: log}
}

// Running is called when a parent task's spinner is displaced by a child.
// In log mode this is the right moment to record that the parent is in progress.
func (p *logPrinter) Running(e Entry) {
	p.log.Info("▶ "+e.Label, "status", "running")
}

func (p *logPrinter) Done(e Entry) {
	msg := "✔ " + e.Label
	if e.Note != "" {
		msg += " " + e.Note
	}
	p.log.Info(msg,
		"status", "done",
		"duration", time.Since(e.StartTime).Round(time.Millisecond).String(),
	)
}

func (p *logPrinter) Failed(e Entry, err error) {
	p.log.Error("✗ "+e.Label,
		"status", "failed",
		"duration", time.Since(e.StartTime).Round(time.Millisecond).String(),
		"error", err,
	)
}

func (p *logPrinter) Text(_ int, text string) {
	p.log.Info(text)
}

func (p *logPrinter) Panic(err error) {
	p.log.Error("fatal", "error", err.Error())
	os.Exit(1)
}

// ── silentPrinter ─────────────────────────────────────────────────────────────

// silentPrinter discards all output.
type silentPrinter struct{}

func (p *silentPrinter) Running(e Entry)           {}
func (p *silentPrinter) Done(e Entry)              {}
func (p *silentPrinter) Failed(e Entry, err error) {}
func (p *silentPrinter) Text(_ int, text string)   {}
func (p *silentPrinter) Panic(err error)           { os.Exit(1) }
