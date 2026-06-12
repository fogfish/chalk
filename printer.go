//
// Copyright (C) 2026 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/chalk
//

package chalk

import "time"

// Entry describes a task at a point in its lifecycle. It is passed to
// Printer methods by value.
type Entry struct {
	Level     int       // nesting level (0 = top-level)
	Label     string    // formatted task label
	Note      string    // optional Done() suffix
	StartTime time.Time // when Task() was called for this entry
}

// taskEntry is the internal stack representation: an Entry plus bookkeeping
// of whether it has already been anchored (printed as "running") because a
// child task displaced it.
type taskEntry struct {
	Entry
	anchored bool
}

// Printer renders task lifecycle events. Implement this interface to ship
// progress to a custom sink (database, websocket, OpenTelemetry span, ...).
// All methods are called while the Reporter's internal mutex is held —
// implementations must not call back into the Reporter.
type Printer interface {
	// Running is called for a parent task that remains active while a
	// child task starts — i.e. the parent becomes "displaced" and needs
	// its own visible record.
	Running(e Entry)

	// Done is called when a task completes successfully.
	Done(e Entry)

	// Failed is called when a task completes with an error.
	Failed(e Entry, err error)

	// Text emits an informational message (Printf) at the given indent
	// level. level == -1 means "no active task".
	Text(level int, text string)

	// Panic is called after all pending tasks have been failed.
	// Implementations typically print and os.Exit(1).
	Panic(err error)
}

// animator is implemented only by printers that render a live, animated
// "current task" line and need to suspend/resume it around other output.
// Printers that don't implement this are treated as fully static.
type animator interface {
	pauseLocked()
	resumeLocked(top *Entry)
}

// multiPrinter fans every event out to a list of printers, in order.
type multiPrinter []Printer

// MultiPrinter returns a Printer that forwards every event to each of
// printers, in order.
func MultiPrinter(printers ...Printer) Printer {
	return multiPrinter(printers)
}

func (m multiPrinter) Running(e Entry) {
	for _, p := range m {
		p.Running(e)
	}
}

func (m multiPrinter) Done(e Entry) {
	for _, p := range m {
		p.Done(e)
	}
}

func (m multiPrinter) Failed(e Entry, err error) {
	for _, p := range m {
		p.Failed(e, err)
	}
}

func (m multiPrinter) Text(level int, text string) {
	for _, p := range m {
		p.Text(level, text)
	}
}

func (m multiPrinter) Panic(err error) {
	for _, p := range m {
		p.Panic(err)
	}
}
