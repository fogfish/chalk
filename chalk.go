package chalk

import (
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	stylePending = lipgloss.NewStyle().Faint(true)
	styleRunning = lipgloss.NewStyle().Bold(true)
	styleDone    = lipgloss.NewStyle().Strikethrough(true).Faint(true)
	styleFailed  = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	styleIndent  = lipgloss.NewStyle().PaddingLeft(4)
	styleCheck   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	styleCross   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	styleSpinClr = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
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
	label string
	state itemState
}

type taskItem struct {
	label string
	state itemState
	subs  []subItem
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
)

// pollCh returns a Cmd that blocks until the next message arrives on ch.
func pollCh(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

// ── BubbleTea model ───────────────────────────────────────────────────────────

type progressModel struct {
	spinner spinner.Model
	tasks   []taskItem
	ch      <-chan tea.Msg
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
		m.tasks = append(m.tasks, taskItem{label: msg.label, state: stateRunning})
		return m, pollCh(m.ch)

	case msgTaskDone:
		if i := m.currentTask(); i >= 0 {
			m.tasks[i].state = stateDone
		}
		return m, pollCh(m.ch)

	case msgTaskFail:
		if i := m.currentTask(); i >= 0 {
			m.tasks[i].state = stateFailed
		}
		return m, pollCh(m.ch)

	case msgSubStart:
		if t := m.currentTask(); t >= 0 {
			m.tasks[t].subs = append(m.tasks[t].subs, subItem{label: msg.label, state: stateRunning})
		}
		return m, pollCh(m.ch)

	case msgSubDone:
		if t := m.currentTask(); t >= 0 {
			if s := m.currentSub(); s >= 0 {
				m.tasks[t].subs[s].state = stateDone
			}
		}
		return m, pollCh(m.ch)

	case msgSubFail:
		if t := m.currentTask(); t >= 0 {
			if s := m.currentSub(); s >= 0 {
				m.tasks[t].subs[s].state = stateFailed
			}
		}
		return m, pollCh(m.ch)

	case msgQuit:
		return m, tea.Quit

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	return m, nil
}

func renderItem(_, label string, st itemState, styleSpin func() string) (prefix, text string) {
	switch st {
	case stateRunning:
		return styleSpin() + " ", styleRunning.Render(label)
	case stateDone:
		return styleCheck.Render("✓") + " ", styleDone.Render(label)
	case stateFailed:
		return styleCross.Render("✗") + " ", styleFailed.Render(label)
	default:
		return stylePending.Render("○") + " ", stylePending.Render(label)
	}
}

func (m progressModel) View() string {
	var sb strings.Builder
	sb.WriteByte('\n')

	for _, t := range m.tasks {
		p, l := renderItem("", t.label, t.state, m.spinner.View)
		sb.WriteString(p + l + "\n")

		for _, s := range t.subs {
			sp, sl := renderItem("", s.label, s.state, m.spinner.View)
			sb.WriteString(styleIndent.Render(sp+sl) + "\n")
		}
	}

	return sb.String()
}

// ── Public Reporter API ───────────────────────────────────────────────────────

var stdout *Reporter

func Start(f func()) {
	stdout = NewReporter()

	go func() {
		defer stdout.Quit()
		f()
	}()

	if err := stdout.Run(); err != nil {
		panic(err)
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

// Reporter drives the BubbleTea progress UI. Call Run() in the main goroutine
// and send progress events from a background goroutine.
type Reporter struct {
	ch chan tea.Msg
	p  *tea.Program
}

// NewReporter creates and wires a Reporter with a running BubbleTea program.
func NewReporter() *Reporter {
	ch := make(chan tea.Msg, 64)
	m := newProgressModel(ch)
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	return &Reporter{ch: ch, p: p}
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
	_, err := r.p.Run()
	return err
}
