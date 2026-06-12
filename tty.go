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

// styles holds the lipgloss styles used by a single ttyPrinter instance.
// Each Reporter built with WithColor(false) gets its own black & white
// styles, independent of any other Reporter in the same process.
type styles struct {
	done          lipgloss.Style
	failed        lipgloss.Style
	running       lipgloss.Style
	errorText     lipgloss.Style
	check         lipgloss.Style
	cross         lipgloss.Style
	arrow         lipgloss.Style
	timer         lipgloss.Style
	timerDuration lipgloss.Style
	timerFail     lipgloss.Style
	errTitle      lipgloss.Style
	text          lipgloss.Style
}

func newStyles(color bool) styles {
	if !color {
		return styles{
			done:          lipgloss.NewStyle().Faint(true),
			failed:        lipgloss.NewStyle().Bold(true),
			running:       lipgloss.NewStyle().Bold(true),
			errorText:     lipgloss.NewStyle(),
			check:         lipgloss.NewStyle().Bold(true),
			cross:         lipgloss.NewStyle().Bold(true),
			arrow:         lipgloss.NewStyle().Bold(true),
			timer:         lipgloss.NewStyle(),
			timerDuration: lipgloss.NewStyle().Faint(true),
			timerFail:     lipgloss.NewStyle().Bold(true),
			errTitle:      lipgloss.NewStyle().Bold(true),
			text:          lipgloss.NewStyle().Faint(true),
		}
	}

	return styles{
		done:          lipgloss.NewStyle().Faint(true),
		failed:        lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),
		running:       lipgloss.NewStyle().Bold(true),
		errorText:     lipgloss.NewStyle(),
		check:         lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true),
		cross:         lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),
		arrow:         lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true),
		timer:         lipgloss.NewStyle().Foreground(lipgloss.Color("12")),
		timerDuration: lipgloss.NewStyle().Faint(true),
		timerFail:     lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		errTitle:      lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),
		text:          lipgloss.NewStyle().Faint(true),
	}
}

// ── ttyPrinter ────────────────────────────────────────────────────────────────

// spinner frames (braille dots)
var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ttyPrinter is the strategy for interactive terminals: ANSI colours, bold
// text, and a live braille spinner on the current task.
type ttyPrinter struct {
	out          io.Writer
	style        styles
	programStart time.Time
	mu           *sync.Mutex   // reference to Reporter.mu
	stopCh       chan struct{} // signals spinner goroutine to stop
	doneCh       chan struct{} // closed when spinner goroutine has exited
}

func newTTYPrinter(out io.Writer, start time.Time, mu *sync.Mutex, color bool) *ttyPrinter {
	return &ttyPrinter{out: out, style: newStyles(color), programStart: start, mu: mu}
}

func (p *ttyPrinter) spinPrefix(e Entry) string {
	wallOff := formatWallClock(time.Since(p.programStart))
	return p.style.timer.Render(wallOff) + "        " + indentStr(e.Level)
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

func (p *ttyPrinter) resumeLocked(top *Entry) {
	if top == nil {
		return
	}
	e := *top
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
				prefix := p.spinPrefix(e)
				fmt.Fprintf(p.out, "\r%s%s %s", prefix, spinFrames[frame%len(spinFrames)], e.Label)
				frame++
			case <-stopCh:
				return
			}
		}
	}()
}

func (p *ttyPrinter) Running(e Entry) {
	wallOff := formatWallClock(e.StartTime.Sub(p.programStart))
	fmt.Fprintln(p.out,
		p.style.timer.Render(wallOff)+" "+
			strings.Repeat(" ", 8)+
			indentStr(e.Level)+
			p.style.arrow.Render("▶")+" "+
			p.style.running.Render(e.Label))
}

func (p *ttyPrinter) Done(e Entry) {
	now := time.Now()
	wallOff := formatWallClock(now.Sub(p.programStart))
	duration := formatElapsed(now.Sub(e.StartTime))
	label := e.Label
	if e.Note != "" {
		label += " " + e.Note
	}
	fmt.Fprintln(p.out,
		p.style.timer.Render(wallOff)+" "+
			p.style.timerDuration.Render("("+duration+")")+" "+
			indentStr(e.Level)+
			p.style.check.Render("✓")+" "+
			p.style.done.Render(label))
}

func (p *ttyPrinter) Failed(e Entry, err error) {
	now := time.Now()
	wallOff := formatWallClock(now.Sub(p.programStart))
	duration := formatElapsed(now.Sub(e.StartTime))
	fmt.Fprintln(p.out,
		p.style.timerFail.Render(wallOff)+" "+
			p.style.timerDuration.Render("("+duration+")")+" "+
			indentStr(e.Level)+
			p.style.cross.Render("✗")+" "+
			p.style.failed.Render(e.Label))
	if err != nil {
		pad := indentStr(e.Level + 1)
		cols := 80 - len(pad)
		if cols < 20 {
			cols = 20
		}
		wrapped := lipgloss.NewStyle().Width(cols).Render(err.Error())
		for _, line := range strings.Split(wrapped, "\n") {
			fmt.Fprintln(p.out, pad+p.style.errorText.Render(line))
		}
	}
}

func (p *ttyPrinter) Text(level int, text string) {
	pad := indentStr(level + 1)
	cols := 80 - len(pad)
	if cols < 20 {
		cols = 20
	}
	for _, para := range strings.Split(text, "\n") {
		wrapped := lipgloss.NewStyle().Width(cols).Render(para)
		for _, line := range strings.Split(wrapped, "\n") {
			fmt.Fprintln(p.out, pad+p.style.text.Render(line))
		}
	}
}

func (p *ttyPrinter) Panic(err error) {
	fmt.Fprintln(p.out, "\n"+p.style.errTitle.Render("Error")+"\n")
	fmt.Fprintln(p.out, p.style.errorText.Render(err.Error()))
	os.Exit(1)
}
