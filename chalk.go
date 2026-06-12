//
// Copyright (C) 2026 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/chalk
//

package chalk

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ── Context level ───────────────────────────────────────────────────────────

type contextLevelKey struct{}

// Sub returns a child context with the task nesting level incremented by one.
// Pass the returned context to Task to start a nested sub-task.
func Sub(ctx context.Context) context.Context {
	level, _ := ctx.Value(contextLevelKey{}).(int)
	return context.WithValue(ctx, contextLevelKey{}, level+1)
}

func levelFromContext(ctx context.Context) int {
	level, _ := ctx.Value(contextLevelKey{}).(int)
	return level
}

// ── Reporter ──────────────────────────────────────────────────────────────────

// Reporter manages progress output using a Printer strategy: ttyPrinter
// (colours + spinner), logPrinter (structured slog records), silentPrinter,
// or any user-supplied Printer.
type Reporter struct {
	mu    sync.Mutex
	stack []taskEntry // active task stack (index 0 = outermost)
	p     Printer
}

// pause halts any live animation before output is written. Caller must
// hold r.mu.
func (r *Reporter) pause() {
	if a, ok := r.p.(animator); ok {
		a.pauseLocked()
	}
}

// resume restarts animation for the current top task (nil = stack empty).
// Caller must hold r.mu.
func (r *Reporter) resume(top *taskEntry) {
	a, ok := r.p.(animator)
	if !ok {
		return
	}
	if top == nil {
		a.resumeLocked(nil)
		return
	}
	a.resumeLocked(&top.Entry)
}

// Task begins a new task at the nesting level carried by ctx. Use Sub to
// produce a context for sub-tasks. Any currently active tasks at the same or
// deeper level are automatically completed before the new task starts, which
// simplifies error handling — callers do not need to guarantee a matching
// Done/Fail on every code path.
func (r *Reporter) Task(ctx context.Context, label string, args ...any) {
	level := levelFromContext(ctx)
	if len(args) > 0 {
		label = fmt.Sprintf(label, args...)
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.pause()

	// Auto-complete tasks at the same or deeper level.
	for len(r.stack) > 0 && r.stack[len(r.stack)-1].Level >= level {
		top := r.stack[len(r.stack)-1]
		r.stack = r.stack[:len(r.stack)-1]
		r.p.Done(top.Entry)
	}

	// Anchor any parent tasks that haven't been printed yet.
	for i := range r.stack {
		if !r.stack[i].anchored {
			r.p.Running(r.stack[i].Entry)
			r.stack[i].anchored = true
		}
	}

	r.stack = append(r.stack, taskEntry{
		Entry: Entry{
			Level:     level,
			Label:     label,
			StartTime: time.Now(),
		},
	})
	r.resume(&r.stack[len(r.stack)-1])
}

// Done marks the current (innermost) task as successfully completed.
// An optional note is appended after the task label, e.g. Done("(hits 50)").
func (r *Reporter) Done(suffix ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.stack) == 0 {
		return
	}
	r.pause()
	top := r.stack[len(r.stack)-1]
	if len(suffix) > 0 {
		top.Note = suffix[0]
	}
	r.stack = r.stack[:len(r.stack)-1]
	r.p.Done(top.Entry)
	var next *taskEntry
	if len(r.stack) > 0 {
		next = &r.stack[len(r.stack)-1]
	}
	r.resume(next)
}

// Fail marks the current (innermost) task as failed. err is printed beneath
// the task line (ttyPrinter) or included as a structured field (logPrinter).
func (r *Reporter) Fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.stack) == 0 {
		return
	}
	r.pause()
	top := r.stack[len(r.stack)-1]
	r.stack = r.stack[:len(r.stack)-1]
	r.p.Failed(top.Entry, err)
	var next *taskEntry
	if len(r.stack) > 0 {
		next = &r.stack[len(r.stack)-1]
	}
	r.resume(next)
}

// Printf prints a formatted message indented under the current task.
func (r *Reporter) Printf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.pause()
	level := -1
	if len(r.stack) > 0 {
		level = r.stack[len(r.stack)-1].Level
	}
	r.p.Text(level, fmt.Sprintf(format, args...))
	var top *taskEntry
	if len(r.stack) > 0 {
		top = &r.stack[len(r.stack)-1]
	}
	r.resume(top)
}

// Quit stops any animation and marks all remaining tasks as done.
func (r *Reporter) Quit() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.pause()
	for len(r.stack) > 0 {
		top := r.stack[len(r.stack)-1]
		r.stack = r.stack[:len(r.stack)-1]
		r.p.Done(top.Entry)
	}
}

// Sub returns a child context with the task nesting level incremented by one.
// Pass the returned context to Task to start a nested sub-task.
func (r *Reporter) Sub(ctx context.Context) context.Context { return Sub(ctx) }

// ── Stdio interface ───────────────────────────────────────────────────────────

// Stdio is the progress-reporting interface implemented by *Reporter.
// Accept Stdio in your own APIs to decouple callers from this package:
// any value whose method set matches Stdio satisfies the interface without
// importing chalk.
type Stdio interface {
	Sub(ctx context.Context) context.Context
	Task(ctx context.Context, label string, args ...any)
	Done(suffix ...string)
	Fail(err error)
	Printf(format string, args ...any)
}

// ── Default reporter ─────────────────────────────────────────────────────────

// def is the package-level Reporter used by Task/Done/Fail/Printf/Panic and
// by Stdout. It is safe to use as-is (logPrinter) without any setup —
// runners (rt/cli, rt/cobra, ...) call SetDefault to install a Reporter
// configured for their environment.
var def *Reporter = &Reporter{p: newLogPrinter(nil)}

// Default returns the package-level Reporter used by Task/Done/Fail/Printf.
func Default() *Reporter { return def }

// SetDefault replaces the package-level Reporter used by Task/Done/Fail/
// Printf/Panic and by Stdout.
func SetDefault(r *Reporter) { def = r }

var Stdout Proxy

type Proxy struct{}

func (Proxy) Sub(ctx context.Context) context.Context             { return Sub(ctx) }
func (Proxy) Task(ctx context.Context, label string, args ...any) { def.Task(ctx, label, args...) }
func (Proxy) Done(suffix ...string)                               { def.Done(suffix...) }
func (Proxy) Fail(err error)                                      { def.Fail(err) }
func (Proxy) Printf(format string, args ...any)                   { def.Printf(format, args...) }

// ── Package-level API ─────────────────────────────────────────────────────────

// Task begins a new task at the nesting level carried by ctx. Use Sub to
// produce a context for sub-tasks. Any active tasks at the same or deeper
// level are auto-completed first.
func Task(ctx context.Context, label string, args ...any) {
	def.Task(ctx, label, args...)
}

// Done marks the current task as successfully completed.
// An optional note is appended after the task label, e.g. Done("(hits 50)").
func Done(suffix ...string) {
	def.Done(suffix...)
}

// Fail marks the current task as failed.
func Fail(err error) {
	def.Fail(err)
}

// Printf prints a formatted message indented under the current task.
func Printf(format string, args ...any) {
	def.Printf(format, args...)
}

// Panic fails all pending tasks and exits with code 1.
func Panic(err error) {
	def.mu.Lock()
	def.pause()
	for len(def.stack) > 0 {
		top := def.stack[len(def.stack)-1]
		def.stack = def.stack[:len(def.stack)-1]
		def.p.Failed(top.Entry, nil)
	}
	def.mu.Unlock()
	def.p.Panic(err)
}
