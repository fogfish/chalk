//
// Copyright (C) 2026 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/chalk
//

package chalk

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
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
	styleText          = lipgloss.NewStyle().Faint(true)
)

// bwStyles overrides the package-level style vars with plain black & white styles.
func bwStyles() {
	styleDone = lipgloss.NewStyle().Faint(true)
	styleFailed = lipgloss.NewStyle().Bold(true)
	styleRunning = lipgloss.NewStyle().Bold(true)
	styleError = lipgloss.NewStyle()
	styleCheck = lipgloss.NewStyle().Bold(true)
	styleCross = lipgloss.NewStyle().Bold(true)
	styleArrow = lipgloss.NewStyle().Bold(true)
	styleTimer = lipgloss.NewStyle()
	styleTimerDuration = lipgloss.NewStyle().Faint(true)
	styleTimerFail = lipgloss.NewStyle().Bold(true)
	styleErrTitle = lipgloss.NewStyle().Bold(true)
	styleText = lipgloss.NewStyle().Faint(true)
}

// ── ttyPrinter ────────────────────────────────────────────────────────────────

// spinner frames (braille dots)
var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ttyPrinter is the strategy for interactive terminals: ANSI colours, bold
// text, and a live braille spinner on the current task.
type ttyPrinter struct {
	out          io.Writer
	programStart time.Time
	mu           *sync.Mutex   // reference to Reporter.mu
	stopCh       chan struct{} // signals spinner goroutine to stop
	doneCh       chan struct{} // closed when spinner goroutine has exited
}

func newTTYPrinter(out io.Writer, start time.Time, mu *sync.Mutex) *ttyPrinter {
	return &ttyPrinter{out: out, programStart: start, mu: mu}
}

func (p *ttyPrinter) spinPrefix(t taskEntry) string {
	wallOff := formatWallClock(time.Since(p.programStart))
	return styleTimer.Render(wallOff) + "        " + indentStr(t.level)
}

func (p *ttyPrinter) pauseLocked() {
	if p.stopCh == nil {
		return
	}
	stopCh := p.stopCh
	doneCh := p.doneCh
	p.stopCh = nil
	p.doneCh = nil

	close(stopCh)
	p.mu.Unlock()
	<-doneCh
	// \r moves to column 0; \033[2K erases the entire line.
	fmt.Fprint(p.out, "\r\033[2K")
	p.mu.Lock()
}

func (p *ttyPrinter) resumeLocked(top *taskEntry) {
	if top == nil {
		return
	}
	t := *top
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	p.stopCh = stopCh
	p.doneCh = doneCh

	go func() {
		defer close(doneCh)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		frame := 0
		for {
			select {
			case <-ticker.C:
				prefix := p.spinPrefix(t)
				fmt.Fprintf(p.out, "\r%s%s %s", prefix, spinFrames[frame%len(spinFrames)], t.label)
				frame++
			case <-stopCh:
				return
			}
		}
	}()
}

func (p *ttyPrinter) printRunning(t taskEntry) {
	wallOff := formatWallClock(t.startTime.Sub(p.programStart))
	fmt.Fprintln(p.out,
		styleTimer.Render(wallOff)+" "+
			indentStr(t.level)+
			styleArrow.Render("▶")+" "+
			styleRunning.Render(t.label))
}

func (p *ttyPrinter) printDone(t taskEntry) {
	now := time.Now()
	wallOff := formatWallClock(now.Sub(p.programStart))
	duration := formatElapsed(now.Sub(t.startTime))
	label := t.label
	if t.note != "" {
		label += " " + t.note
	}
	fmt.Fprintln(p.out,
		styleTimer.Render(wallOff)+" "+
			styleTimerDuration.Render("("+duration+")")+" "+
			indentStr(t.level)+
			styleCheck.Render("✓")+" "+
			styleDone.Render(label))
}

func (p *ttyPrinter) printFailed(t taskEntry, err error) {
	now := time.Now()
	wallOff := formatWallClock(now.Sub(p.programStart))
	duration := formatElapsed(now.Sub(t.startTime))
	fmt.Fprintln(p.out,
		styleTimerFail.Render(wallOff)+" "+
			styleTimerDuration.Render("("+duration+")")+" "+
			indentStr(t.level)+
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
			fmt.Fprintln(p.out, pad+styleError.Render(line))
		}
	}
}

func (p *ttyPrinter) printText(level int, text string) {
	pad := indentStr(level + 1)
	cols := 80 - len(pad)
	if cols < 20 {
		cols = 20
	}
	for _, para := range strings.Split(text, "\n") {
		wrapped := lipgloss.NewStyle().Width(cols).Render(para)
		for _, line := range strings.Split(wrapped, "\n") {
			fmt.Fprintln(p.out, pad+styleText.Render(line))
		}
	}
}

func (p *ttyPrinter) printPanic(err error) {
	fmt.Fprintln(p.out, "\n"+styleErrTitle.Render("Error")+"\n")
	fmt.Fprintln(p.out, styleError.Render(err.Error()))
	os.Exit(1)
}
