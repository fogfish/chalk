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
type logPrinter struct{}

func (p *logPrinter) pauseLocked()              {}
func (p *logPrinter) resumeLocked(_ *taskEntry) {}

// printRunning is called when a parent task's spinner is displaced by a child.
// In log mode this is the right moment to record that the parent is in progress.
func (p *logPrinter) printRunning(t taskEntry) {
	slog.Info("▶ "+t.label, "status", "running")
}

func (p *logPrinter) printDone(t taskEntry) {
	msg := "✔ " + t.label
	if t.note != "" {
		msg += " " + t.note
	}
	slog.Info(msg,
		"status", "done",
		"duration", time.Since(t.startTime).Round(time.Millisecond).String(),
	)
}

func (p *logPrinter) printFailed(t taskEntry, err error) {
	slog.Error("✗ "+t.label,
		"status", "failed",
		"duration", time.Since(t.startTime).Round(time.Millisecond).String(),
		"error", err,
	)
}

func (p *logPrinter) printText(_ int, text string) {
	slog.Info(text)
}

func (p *logPrinter) printPanic(err error) {
	slog.Error("fatal", "error", err.Error())
	os.Exit(1)
}
