package chalk

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/fogfish/stream/spool"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	styleDone          = lipgloss.NewStyle().Faint(true)
	styleFailed        = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	styleRunning       = lipgloss.NewStyle().Bold(true)
	styleError         = lipgloss.NewStyle()
	styleCheck         = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	styleCross         = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	styleArrow         = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	styleTimer         = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styleTimerDuration = lipgloss.NewStyle().Faint(true)
	styleTimerFail     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleErrTitle      = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
)

const indentUnit = "    " // 4 spaces per level

func indentStr(level int) string {
	if level <= 0 {
		return ""
	}
	return strings.Repeat(indentUnit, level)
}

func formatElapsed(d time.Duration) string {
	total := int(d.Seconds())
	if total < 0 {
		total = 0
	}
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

// ── Task entry ────────────────────────────────────────────────────────────────

type taskEntry struct {
	level     int
	label     string
	startTime time.Time
	anchored  bool // true once a running breadcrumb line has been printed
}

// ── Reporter ──────────────────────────────────────────────────────────────────

// spinner frames (braille dots)
var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Reporter manages classical terminal output with a live spinner for the active
// task and a scrolling history of completed / failed tasks above it.
type Reporter struct {
	mu           sync.Mutex
	stack        []taskEntry   // active task stack (index 0 = outermost)
	stopCh       chan struct{} // signals spinner goroutine to stop
	doneCh       chan struct{} // closed when spinner goroutine has exited
	out          io.Writer
	programStart time.Time // wall-clock anchor for the left-column offset
}

// NewReporter creates a Reporter that writes to stderr.
func NewReporter() *Reporter {
	return &Reporter{out: os.Stderr, programStart: time.Now()}
}

// spinDesc builds the spinner description line for task t.
// Safe to call without the mutex — uses only the t copy and time.Since.
func (r *Reporter) spinDesc(t taskEntry) string {
	wallOff := formatElapsed(t.startTime.Sub(r.programStart))
	taskElapsed := formatElapsed(time.Since(t.startTime))
	return indentStr(t.level) + wallOff + " (" + taskElapsed + ") " + t.label
}

// startSpinnerLocked starts a background goroutine that renders a braille
// spinner on the current line. We own every byte so clearing is reliable.
// Caller must hold r.mu.
func (r *Reporter) startSpinnerLocked() {
	if len(r.stack) == 0 {
		return
	}
	t := r.stack[len(r.stack)-1]

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	r.stopCh = stopCh
	r.doneCh = doneCh

	go func() {
		defer close(doneCh)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		frame := 0
		for {
			select {
			case <-ticker.C:
				desc := r.spinDesc(t)
				fmt.Fprintf(r.out, "\r%s %s", spinFrames[frame%len(spinFrames)], desc)
				frame++
			case <-stopCh:
				return
			}
		}
	}()
}

// stopSpinnerLocked stops the spinner goroutine and erases the spinner line.
// It temporarily releases r.mu while waiting for the goroutine to exit, then
// reacquires it. Caller must hold r.mu.
func (r *Reporter) stopSpinnerLocked() {
	if r.stopCh == nil {
		return
	}
	stopCh := r.stopCh
	doneCh := r.doneCh
	r.stopCh = nil
	r.doneCh = nil

	close(stopCh)
	r.mu.Unlock()
	<-doneCh
	// \r moves to column 0; \033[2K erases the entire line.
	// Because we wrote every byte ourselves (no progressbar buffering),
	// the cursor is guaranteed to be on the spinner line.
	fmt.Fprint(r.out, "\r\033[2K")
	r.mu.Lock()
}

// printRunningLocked prints a static "still running" breadcrumb for a parent
// task whose spinner was displaced by a child. Called at most once per task.
func (r *Reporter) printRunningLocked(t taskEntry) {
	wallOff := formatElapsed(t.startTime.Sub(r.programStart))
	fmt.Fprintln(r.out,
		indentStr(t.level)+
			styleTimer.Render(wallOff)+" "+
			styleArrow.Render("▶")+" "+
			styleRunning.Render(t.label))
}

func (r *Reporter) printDoneLocked(t taskEntry) {
	now := time.Now()
	wallOff := formatElapsed(now.Sub(r.programStart))
	duration := formatElapsed(now.Sub(t.startTime))
	fmt.Fprintln(r.out,
		indentStr(t.level)+
			styleTimer.Render(wallOff)+" "+
			styleTimerDuration.Render("("+duration+")")+" "+
			styleCheck.Render("✓")+" "+
			styleDone.Render(t.label))
}

func (r *Reporter) printFailedLocked(t taskEntry, err error) {
	now := time.Now()
	wallOff := formatElapsed(now.Sub(r.programStart))
	duration := formatElapsed(now.Sub(t.startTime))
	fmt.Fprintln(r.out,
		indentStr(t.level)+
			styleTimerFail.Render(wallOff)+" "+
			styleTimerDuration.Render("("+duration+")")+" "+
			styleCross.Render("✗")+" "+
			styleFailed.Render(t.label))
	if err != nil {
		pad := indentStr(t.level + 1)
		cols := 80 - len(pad)
		if cols < 20 {
			cols = 20
		}
		wrapped := lipgloss.NewStyle().Width(cols).Render(err.Error())
		for _, line := range strings.Split(wrapped, "\n") {
			fmt.Fprintln(r.out, pad+styleError.Render(line))
		}
	}
}

// Task begins a new task at the given nesting level. Any currently active tasks
// at the same or deeper level are automatically completed before the new task
// starts, which simplifies error handling — callers do not need to guarantee a
// matching Done/Fail on every code path.
func (r *Reporter) Task(level int, label string, args ...any) {
	if len(args) > 0 {
		label = fmt.Sprintf(label, args...)
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stopSpinnerLocked()

	// Auto-complete tasks at the same or deeper level.
	for len(r.stack) > 0 && r.stack[len(r.stack)-1].level >= level {
		top := r.stack[len(r.stack)-1]
		r.stack = r.stack[:len(r.stack)-1]
		r.printDoneLocked(top)
	}

	// Anchor any parent tasks that haven't been printed yet: emit a static
	// "running" breadcrumb so the parent line is never silently erased.
	for i := range r.stack {
		if !r.stack[i].anchored {
			r.printRunningLocked(r.stack[i])
			r.stack[i].anchored = true
		}
	}

	r.stack = append(r.stack, taskEntry{
		level:     level,
		label:     label,
		startTime: time.Now(),
	})
	r.startSpinnerLocked()
}

// Done marks the current (innermost) task as successfully completed.
func (r *Reporter) Done() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.stack) == 0 {
		return
	}
	r.stopSpinnerLocked()
	top := r.stack[len(r.stack)-1]
	r.stack = r.stack[:len(r.stack)-1]
	r.printDoneLocked(top)
	r.startSpinnerLocked()
}

// Fail marks the current (innermost) task as failed. err is printed as an
// indented paragraph beneath the task line.
func (r *Reporter) Fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.stack) == 0 {
		return
	}
	r.stopSpinnerLocked()
	top := r.stack[len(r.stack)-1]
	r.stack = r.stack[:len(r.stack)-1]
	r.printFailedLocked(top, err)
	r.startSpinnerLocked()
}

// Quit stops the spinner and marks all remaining tasks as done. Call this when
// all work has been completed.
func (r *Reporter) Quit() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stopSpinnerLocked()
	for len(r.stack) > 0 {
		top := r.stack[len(r.stack)-1]
		r.stack = r.stack[:len(r.stack)-1]
		r.printDoneLocked(top)
	}
}

// ── Package-level API ─────────────────────────────────────────────────────────

var stdout *Reporter

// Start wires up the default Reporter, parses flags, and runs the provided
// Spooler. It is the main entry-point for CLI tools built on this library.
func Start(f spool.Spooler) {
	flag.Parse()

	stdout = NewReporter()

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

// Task begins a new task at the given nesting level. Any active tasks at the
// same or deeper level are auto-completed first.
func Task(level int, label string, args ...any) { stdout.Task(level, label, args...) }

// Done marks the current task as successfully completed.
func Done() { stdout.Done() }

// Fail marks the current task as failed.
func Fail(err error) { stdout.Fail(err) }

// Panic prints a fatal error, fails all pending tasks, and exits with code 1.
func Panic(err error) {
	if stdout != nil {
		stdout.mu.Lock()
		stdout.stopSpinnerLocked()
		for len(stdout.stack) > 0 {
			top := stdout.stack[len(stdout.stack)-1]
			stdout.stack = stdout.stack[:len(stdout.stack)-1]
			stdout.printFailedLocked(top, nil)
		}
		stdout.mu.Unlock()
	}

	fmt.Fprintln(os.Stderr, "\n"+styleErrTitle.Render("Error")+"\n")
	fmt.Fprintln(os.Stderr, styleError.Render(err.Error()))
	os.Exit(1)
}
