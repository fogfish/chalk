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
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fogfish/stream/spool"
	"golang.org/x/term"
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

// ── Helpers ───────────────────────────────────────────────────────────────────

const indentUnit = "    " // 4 spaces per level

func indentStr(level int) string {
	if level <= 0 {
		return ""
	}
	return strings.Repeat(indentUnit, level)
}

// formatWallClock formats a wall-clock offset as "00m 00.0s".
func formatWallClock(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalTenths := int(d.Milliseconds() / 100)
	secs := totalTenths / 10
	tenths := totalTenths % 10
	mins := secs / 60
	secs = secs % 60
	return fmt.Sprintf("%02dm %02d.%ds", mins, secs, tenths)
}

// formatElapsed formats a task duration as "00.0s", or "00.0m" when >= 100 s.
func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < 100*time.Second {
		totalTenths := int(d.Milliseconds() / 100)
		secs := totalTenths / 10
		tenths := totalTenths % 10
		return fmt.Sprintf("%02d.%ds", secs, tenths)
	}
	wholeMinutes := int(d.Minutes())
	tenthsMin := int(d.Seconds()/6) % 10
	return fmt.Sprintf("%02d.%dm", wholeMinutes, tenthsMin)
}

// ── Task entry ────────────────────────────────────────────────────────────────

type taskEntry struct {
	level     int
	label     string
	note      string // optional suffix printed after the label on completion
	startTime time.Time
	anchored  bool // true once a running breadcrumb line has been printed
}

// ── printer interface ─────────────────────────────────────────────────────────

// printer is the rendering strategy used by Reporter.
// All methods except pauseLocked/resumeLocked are called while the Reporter
// mutex is held and no animation is running.
type printer interface {
	// pauseLocked halts any live animation before output is written.
	// The implementation may temporarily release and reacquire the mutex.
	// Caller must hold the Reporter mutex.
	pauseLocked()

	// resumeLocked restarts animation for the current top task (nil = stack empty).
	// Caller must hold the Reporter mutex.
	resumeLocked(top *taskEntry)

	// printRunning emits a "still running" breadcrumb for a parent task whose
	// animation was displaced by a child starting.
	printRunning(t taskEntry)

	printDone(t taskEntry)
	printFailed(t taskEntry, err error)

	// printText emits an informational message under the given indent level.
	printText(level int, text string)

	// printPanic emits a fatal error message and exits with code 1.
	printPanic(err error)
}

// ── Reporter ──────────────────────────────────────────────────────────────────

// Reporter manages progress output using one of two printer strategies:
// ttyPrinter (colours + spinner) when stderr is an interactive terminal, or
// logPrinter (structured slog records) otherwise.
type Reporter struct {
	mu    sync.Mutex
	stack []taskEntry // active task stack (index 0 = outermost)
	p     printer
}

// Init creates a Reporter that writes to stderr, automatically selecting
// the ttyPrinter or logPrinter strategy based on whether stderr is a terminal.
// If NoTTY has been called, logPrinter is always used regardless of the terminal.
func Init() *Reporter {
	flag.Parse()

	if *flagNoTTY {
		noTTY = true
	}

	if *flagNoColor {
		noColor = true
	}

	r := &Reporter{}
	start := time.Now()
	if !noTTY && term.IsTerminal(int(os.Stderr.Fd())) {
		if noColor {
			bwStyles()
		}
		r.p = newTTYPrinter(os.Stderr, start, &r.mu)
	} else {
		r.p = &logPrinter{}
	}

	stdout = r
	return r
}

// Task begins a new task at the nesting level carried by ctx. Use WithLevel to
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

	r.p.pauseLocked()

	// Auto-complete tasks at the same or deeper level.
	for len(r.stack) > 0 && r.stack[len(r.stack)-1].level >= level {
		top := r.stack[len(r.stack)-1]
		r.stack = r.stack[:len(r.stack)-1]
		r.p.printDone(top)
	}

	// Anchor any parent tasks that haven't been printed yet.
	for i := range r.stack {
		if !r.stack[i].anchored {
			r.p.printRunning(r.stack[i])
			r.stack[i].anchored = true
		}
	}

	r.stack = append(r.stack, taskEntry{
		level:     level,
		label:     label,
		startTime: time.Now(),
	})
	r.p.resumeLocked(&r.stack[len(r.stack)-1])
}

// Done marks the current (innermost) task as successfully completed.
// An optional note is appended after the task label, e.g. Done("(hits 50)").
func (r *Reporter) Done(suffix ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.stack) == 0 {
		return
	}
	r.p.pauseLocked()
	top := r.stack[len(r.stack)-1]
	if len(suffix) > 0 {
		top.note = suffix[0]
	}
	r.stack = r.stack[:len(r.stack)-1]
	r.p.printDone(top)
	var next *taskEntry
	if len(r.stack) > 0 {
		next = &r.stack[len(r.stack)-1]
	}
	r.p.resumeLocked(next)
}

// Fail marks the current (innermost) task as failed. err is printed beneath
// the task line (ttyPrinter) or included as a structured field (logPrinter).
func (r *Reporter) Fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.stack) == 0 {
		return
	}
	r.p.pauseLocked()
	top := r.stack[len(r.stack)-1]
	r.stack = r.stack[:len(r.stack)-1]
	r.p.printFailed(top, err)
	var next *taskEntry
	if len(r.stack) > 0 {
		next = &r.stack[len(r.stack)-1]
	}
	r.p.resumeLocked(next)
}

// Printf prints a formatted message indented under the current task.
func (r *Reporter) Printf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.p.pauseLocked()
	level := -1
	if len(r.stack) > 0 {
		level = r.stack[len(r.stack)-1].level
	}
	r.p.printText(level, fmt.Sprintf(format, args...))
	var top *taskEntry
	if len(r.stack) > 0 {
		top = &r.stack[len(r.stack)-1]
	}
	r.p.resumeLocked(top)
}

// Quit stops any animation and marks all remaining tasks as done.
func (r *Reporter) Quit() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.p.pauseLocked()
	for len(r.stack) > 0 {
		top := r.stack[len(r.stack)-1]
		r.stack = r.stack[:len(r.stack)-1]
		r.p.printDone(top)
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

var Stdout Proxy

type Proxy struct{}

func (Proxy) Sub(ctx context.Context) context.Context { return Sub(ctx) }
func (Proxy) Task(ctx context.Context, label string, args ...any) {
	if stdout != nil {
		stdout.Task(ctx, label, args...)
	}
}
func (Proxy) Done(suffix ...string) {
	if stdout != nil {
		stdout.Done(suffix...)
	}
}
func (Proxy) Fail(err error) {
	if stdout != nil {
		stdout.Fail(err)
	}
}
func (Proxy) Printf(format string, args ...any) {
	if stdout != nil {
		stdout.Printf(format, args...)
	}
}

// ── Package-level API ─────────────────────────────────────────────────────────

var stdout *Reporter
var noTTY bool
var noColor bool

// NoTTY forces log-mode output (structured slog records, no colour, no spinner)
// even when stderr is an interactive terminal. Must be called before Start.
func NoTTY() { noTTY = true }

// NoColor disables color output in TTY mode, using plain black & white styles.
// Must be called before Start.
func NoColor() { noColor = true }

// Start wires up the default Reporter, parses flags, and runs the provided
// Spooler. It is the main entry-point for CLI tools built on this library.
func Start(f spool.Spooler) {
	Init()

	src, wlk, err := source()
	if err != nil {
		Panic(err)
	}

	dst, err := target()
	if err != nil {
		Panic(err)
	}

	ingress := spool.New(src, dst)
	if err = ingress.ForEach(context.Background(), wlk, f); err != nil {
		Panic(err)
	}

	stdout.Quit()
}

// Task begins a new task at the nesting level carried by ctx. Use WithLevel to
// produce a context for sub-tasks. Any active tasks at the same or deeper level
// are auto-completed first.
func Task(ctx context.Context, label string, args ...any) { stdout.Task(ctx, label, args...) }

// Done marks the current task as successfully completed.
// An optional note is appended after the task label, e.g. Done("(hits 50)").
func Done(suffix ...string) { stdout.Done(suffix...) }

// Fail marks the current task as failed.
func Fail(err error) { stdout.Fail(err) }

// Printf prints a formatted message indented under the current task.
func Printf(format string, args ...any) { stdout.Printf(format, args...) }

// Panic fails all pending tasks and exits with code 1.
func Panic(err error) {
	if stdout != nil {
		stdout.mu.Lock()
		stdout.p.pauseLocked()
		for len(stdout.stack) > 0 {
			top := stdout.stack[len(stdout.stack)-1]
			stdout.stack = stdout.stack[:len(stdout.stack)-1]
			stdout.p.printFailed(top, nil)
		}
		stdout.mu.Unlock()
		stdout.p.printPanic(err)
		return
	}
	fmt.Fprintf(os.Stderr, "\nError\n\n%s\n", err.Error())
	os.Exit(1)
}
