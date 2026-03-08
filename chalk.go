package chalk

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fogfish/stream/spool"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	stylePending  = lipgloss.NewStyle().Faint(true)
	styleRunning  = lipgloss.NewStyle().Bold(true)
	styleDone     = lipgloss.NewStyle().Strikethrough(true).Faint(true)
	styleFailed   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	styleError    = lipgloss.NewStyle().Faint(false)
	styleIndent   = lipgloss.NewStyle().PaddingLeft(4)
	styleCheck    = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	styleCross    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	styleSpinClr  = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styleTimer    = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styleTimerOff = lipgloss.NewStyle().Faint(true)
)

// ── Item states ───────────────────────────────────────────────────────────────

type itemState int8

const (
	statePending itemState = iota
	stateRunning
	stateDone
	stateFailed
)

// ── Data model ────────────────────────────────────────────────────────────────

type subItem struct {
	label     string
	state     itemState
	err       error
	startTime time.Time
	elapsed   time.Duration
}

type taskItem struct {
	label     string
	state     itemState
	err       error
	subs      []subItem
	startTime time.Time
	elapsed   time.Duration
}

// ── BubbleTea messages ────────────────────────────────────────────────────────

type (
	msgTaskStart struct{ label string }
	msgTaskDone  struct{}
	msgTaskFail  struct{ err error }
	msgSubStart  struct{ label string }
	msgSubDone   struct{}
	msgSubFail   struct{ err error }
	msgQuit      struct{}
	msgPanic     struct{ err error }
)

// pollCh returns a Cmd that blocks until the next message arrives on ch.
func pollCh(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

// ── BubbleTea model ───────────────────────────────────────────────────────────

type progressModel struct {
	spinner  spinner.Model
	tasks    []taskItem
	ch       <-chan tea.Msg
	panicErr error
	width    int
}

func newProgressModel(ch <-chan tea.Msg) progressModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styleSpinClr
	return progressModel{spinner: s, ch: ch}
}

func (m progressModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, pollCh(m.ch))
}

// currentTask returns the index of the last task in stateRunning, or -1.
func (m *progressModel) currentTask() int {
	for i := len(m.tasks) - 1; i >= 0; i-- {
		if m.tasks[i].state == stateRunning {
			return i
		}
	}
	return -1
}

// currentSub returns the index of the last subtask in stateRunning for the
// current task, or -1.
func (m *progressModel) currentSub() int {
	t := m.currentTask()
	if t < 0 {
		return -1
	}
	subs := m.tasks[t].subs
	for i := len(subs) - 1; i >= 0; i-- {
		if subs[i].state == stateRunning {
			return i
		}
	}
	return -1
}

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case msgTaskStart:
		m.tasks = append(m.tasks, taskItem{label: msg.label, state: stateRunning, startTime: time.Now()})
		return m, pollCh(m.ch)

	case msgTaskDone:
		if i := m.currentTask(); i >= 0 {
			m.tasks[i].elapsed = time.Since(m.tasks[i].startTime)
			m.tasks[i].state = stateDone
		}
		return m, pollCh(m.ch)

	case msgTaskFail:
		if i := m.currentTask(); i >= 0 {
			m.tasks[i].elapsed = time.Since(m.tasks[i].startTime)
			m.tasks[i].state = stateFailed
			m.tasks[i].err = msg.err
		}
		return m, pollCh(m.ch)

	case msgSubStart:
		if t := m.currentTask(); t >= 0 {
			m.tasks[t].subs = append(m.tasks[t].subs, subItem{label: msg.label, state: stateRunning, startTime: time.Now()})
		}
		return m, pollCh(m.ch)

	case msgSubDone:
		if t := m.currentTask(); t >= 0 {
			if s := m.currentSub(); s >= 0 {
				m.tasks[t].subs[s].elapsed = time.Since(m.tasks[t].subs[s].startTime)
				m.tasks[t].subs[s].state = stateDone
			}
		}
		return m, pollCh(m.ch)

	case msgSubFail:
		if t := m.currentTask(); t >= 0 {
			if s := m.currentSub(); s >= 0 {
				m.tasks[t].subs[s].elapsed = time.Since(m.tasks[t].subs[s].startTime)
				m.tasks[t].subs[s].state = stateFailed
				m.tasks[t].subs[s].err = msg.err
			}
		}
		return m, pollCh(m.ch)

	case msgQuit:
		return m, tea.Quit

	case msgPanic:
		m.panicErr = msg.err
		return m, tea.Quit

	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	return m, nil
}

// formatElapsed renders a duration as MM:SS with a fixed 5-character width.
func formatElapsed(d time.Duration) string {
	total := int(d.Seconds())
	if total < 0 {
		total = 0
	}
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

func renderItem(label string, st itemState, startTime time.Time, elapsed time.Duration, styleSpin func() string) string {
	var timer, icon, text string

	switch st {
	case statePending:
		timer = styleTimerOff.Render("--:--")
		icon = stylePending.Render("○") + " "
		text = stylePending.Render(label)
	case stateRunning:
		timer = styleTimer.Render(formatElapsed(time.Since(startTime)))
		icon = styleSpin() + " "
		text = styleRunning.Render(label)
	case stateDone:
		timer = styleTimerOff.Render(formatElapsed(elapsed))
		icon = styleCheck.Render("✓") + " "
		text = styleDone.Render(label)
	case stateFailed:
		timer = styleCross.Render(formatElapsed(elapsed))
		icon = styleCross.Render("✗") + " "
		text = styleFailed.Render(label)
	}

	return timer + " " + icon + text
}

func (m progressModel) View() string {
	var sb strings.Builder
	sb.WriteByte('\n')

	for _, t := range m.tasks {
		sb.WriteString(renderItem(t.label, t.state, t.startTime, t.elapsed, m.spinner.View) + "\n")
		if t.err != nil {
			sb.WriteString(styleIndent.Render(styleError.Render(t.err.Error())) + "\n")
		}

		for _, s := range t.subs {
			sb.WriteString(styleIndent.Render(renderItem(s.label, s.state, s.startTime, s.elapsed, m.spinner.View)) + "\n")
			if s.err != nil {
				sb.WriteString(styleIndent.Render(styleIndent.Render(styleError.Render(s.err.Error()))) + "\n")
			}
		}
	}

	if m.panicErr != nil {
		w := m.width
		if w <= 0 {
			w = 80
		}
		sb.WriteByte('\n')
		sb.WriteString(styleFailed.Render("Something went wrong") + "\n")
		sb.WriteByte('\n')
		sb.WriteString(styleError.Width(w).Render(m.panicErr.Error()) + "\n")
	}

	return sb.String()
}

// ── Public Reporter API ───────────────────────────────────────────────────────

var stdout *Reporter

func Start(f spool.Spooler) {
	flag.Parse()

	stdout = NewReporter()

	go func() {
		defer stdout.Quit()

		src, wlk, err := source()
		if err != nil {
			Panic(err)
		}

		dst, err := target()
		if err != nil {
			Panic(err)
		}

		ingress := spool.New(src, dst)
		err = ingress.ForEach(context.Background(), wlk, f)
		if err != nil {
			Panic(err)
		}
	}()

	if err := stdout.Run(); err != nil {
		os.Exit(1) //nolint
	}
}

// Task begins a new top-level task entry.
func Task(label string) { stdout.Task(label) }

// Done marks the current top-level task as completed (strikethrough).
func Done() { stdout.Done() }

// Fail marks the current top-level task as failed.
func Fail(err error) { stdout.Fail(err) }

// SubTask begins a new sub-item under the current top-level task.
func SubTask(label string) { stdout.SubTask(label) }

// SubDone marks the current subtask as completed (strikethrough).
func SubDone() { stdout.SubDone() }

// SubFail marks the current subtask as failed.
func SubFail(err error) { stdout.SubFail(err) }

func Panic(err error) {
	if stdout != nil {
		stdout.ch <- msgPanic{err: err}
		<-stdout.done
	} else {
		fmt.Fprintf(os.Stderr, "%s\n\n", styleFailed.Render("Something went wrong"))
		fmt.Fprintf(os.Stderr, "%s\n", styleError.Render(err.Error()))
	}
	os.Exit(1)
}

// Reporter drives the BubbleTea progress UI. Call Run() in the main goroutine
// and send progress events from a background goroutine.
type Reporter struct {
	ch   chan tea.Msg
	p    *tea.Program
	done chan struct{}
}

// NewReporter creates and wires a Reporter with a running BubbleTea program.
func NewReporter() *Reporter {
	ch := make(chan tea.Msg, 64)
	m := newProgressModel(ch)
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	return &Reporter{ch: ch, p: p, done: make(chan struct{})}
}

// Task begins a new top-level task entry.
func (r *Reporter) Task(label string) { r.ch <- msgTaskStart{label: label} }

// Done marks the current top-level task as completed (strikethrough).
func (r *Reporter) Done() { r.ch <- msgTaskDone{} }

// Fail marks the current top-level task as failed.
func (r *Reporter) Fail(err error) { r.ch <- msgTaskFail{err: err} }

// SubTask begins a new sub-item under the current top-level task.
func (r *Reporter) SubTask(label string) { r.ch <- msgSubStart{label: label} }

// SubDone marks the current subtask as completed (strikethrough).
func (r *Reporter) SubDone() { r.ch <- msgSubDone{} }

// SubFail marks the current subtask as failed.
func (r *Reporter) SubFail(err error) { r.ch <- msgSubFail{err: err} }

// Quit tears down the BubbleTea program. Call this after all work is done.
func (r *Reporter) Quit() { r.ch <- msgQuit{} }

// Run starts the BubbleTea event loop (blocking). Call this from main.
func (r *Reporter) Run() error {
	finalModel, err := r.p.Run()
	close(r.done)
	if err != nil {
		return err
	}
	if m, ok := finalModel.(progressModel); ok && m.panicErr != nil {
		return m.panicErr
	}
	return nil
}
