//
// Copyright (C) 2026 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/chalk
//

package chalk_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fogfish/chalk"
)

type event struct {
	kind  string
	level int
	label string
	note  string
	err   error
}

type fakePrinter struct {
	events []event
}

func (p *fakePrinter) Running(e chalk.Entry) {
	p.events = append(p.events, event{kind: "running", level: e.Level, label: e.Label})
}

func (p *fakePrinter) Done(e chalk.Entry) {
	p.events = append(p.events, event{kind: "done", level: e.Level, label: e.Label, note: e.Note})
}

func (p *fakePrinter) Failed(e chalk.Entry, err error) {
	p.events = append(p.events, event{kind: "failed", level: e.Level, label: e.Label, err: err})
}

func (p *fakePrinter) Text(level int, text string) {
	p.events = append(p.events, event{kind: "text", level: level, label: text})
}

func (p *fakePrinter) Panic(err error) {
	p.events = append(p.events, event{kind: "panic", err: err})
}

func TestTaskDone(t *testing.T) {
	p := &fakePrinter{}
	r := chalk.New(chalk.WithPrinter(p))
	ctx := context.Background()

	r.Task(ctx, "Configure")
	r.Done()

	want := []event{{kind: "done", level: 0, label: "Configure"}}
	assertEvents(t, p.events, want)
}

func TestDoneSuffix(t *testing.T) {
	p := &fakePrinter{}
	r := chalk.New(chalk.WithPrinter(p))
	ctx := context.Background()

	r.Task(ctx, "Embedding %d chunks", 3)
	r.Done("(3 vectors)")

	want := []event{{kind: "done", level: 0, label: "Embedding 3 chunks", note: "(3 vectors)"}}
	assertEvents(t, p.events, want)
}

func TestFail(t *testing.T) {
	p := &fakePrinter{}
	r := chalk.New(chalk.WithPrinter(p))
	ctx := context.Background()
	boom := errors.New("boom")

	r.Task(ctx, "Reading")
	r.Fail(boom)

	want := []event{{kind: "failed", level: 0, label: "Reading", err: boom}}
	assertEvents(t, p.events, want)
}

func TestSubTaskAnchorsParent(t *testing.T) {
	p := &fakePrinter{}
	r := chalk.New(chalk.WithPrinter(p))
	ctx := context.Background()

	r.Task(ctx, "Outer")
	sub := r.Sub(ctx)
	r.Task(sub, "Inner 1")
	r.Done("(note)")
	r.Task(sub, "Inner 2")
	r.Done()
	r.Done()

	want := []event{
		{kind: "running", level: 0, label: "Outer"},
		{kind: "done", level: 1, label: "Inner 1", note: "(note)"},
		{kind: "done", level: 1, label: "Inner 2"},
		{kind: "done", level: 0, label: "Outer"},
	}
	assertEvents(t, p.events, want)
}

func TestTaskAtSameLevelAutoCompletesPrevious(t *testing.T) {
	p := &fakePrinter{}
	r := chalk.New(chalk.WithPrinter(p))
	ctx := context.Background()

	r.Task(ctx, "A")
	r.Task(ctx, "B") // A never gets an explicit Done/Fail
	r.Done()

	want := []event{
		{kind: "done", level: 0, label: "A"},
		{kind: "done", level: 0, label: "B"},
	}
	assertEvents(t, p.events, want)
}

func TestPrintf(t *testing.T) {
	p := &fakePrinter{}
	r := chalk.New(chalk.WithPrinter(p))
	ctx := context.Background()

	r.Printf("top level note")
	r.Task(ctx, "A")
	r.Printf("nested note %d", 42)
	r.Done()

	want := []event{
		{kind: "text", level: -1, label: "top level note"},
		{kind: "text", level: 0, label: "nested note 42"},
		{kind: "done", level: 0, label: "A"},
	}
	assertEvents(t, p.events, want)
}

func TestQuitDrainsStack(t *testing.T) {
	p := &fakePrinter{}
	r := chalk.New(chalk.WithPrinter(p))
	ctx := context.Background()

	r.Task(ctx, "Outer")
	r.Task(r.Sub(ctx), "Inner")
	r.Quit()

	want := []event{
		{kind: "running", level: 0, label: "Outer"},
		{kind: "done", level: 1, label: "Inner"},
		{kind: "done", level: 0, label: "Outer"},
	}
	assertEvents(t, p.events, want)
}

func TestDoneFailOnEmptyStackAreNoops(t *testing.T) {
	p := &fakePrinter{}
	r := chalk.New(chalk.WithPrinter(p))

	r.Done()
	r.Fail(errors.New("boom"))

	if len(p.events) != 0 {
		t.Fatalf("expected no events, got %v", p.events)
	}
}

func TestMultiPrinter(t *testing.T) {
	a := &fakePrinter{}
	b := &fakePrinter{}
	r := chalk.New(chalk.WithPrinter(chalk.MultiPrinter(a, b)))
	ctx := context.Background()

	r.Task(ctx, "A")
	r.Done()

	want := []event{{kind: "done", level: 0, label: "A"}}
	assertEvents(t, a.events, want)
	assertEvents(t, b.events, want)
}

func TestDefaultAndPanic(t *testing.T) {
	orig := chalk.Default()
	defer chalk.SetDefault(orig)

	p := &fakePrinter{}
	chalk.SetDefault(chalk.New(chalk.WithPrinter(p)))

	ctx := context.Background()
	chalk.Task(ctx, "Outer")
	chalk.Task(chalk.Sub(ctx), "Inner")
	chalk.Panic(errors.New("fatal"))

	want := []event{
		{kind: "running", level: 0, label: "Outer"},
		{kind: "failed", level: 1, label: "Inner"},
		{kind: "failed", level: 0, label: "Outer"},
		{kind: "panic", err: errors.New("fatal")},
	}
	assertEvents(t, p.events, want)
}

func assertEvents(t *testing.T, got, want []event) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("event count mismatch: got %d %v, want %d %v", len(got), got, len(want), want)
	}

	for i := range want {
		g, w := got[i], want[i]
		if g.kind != w.kind || g.level != w.level || g.label != w.label || g.note != w.note {
			t.Fatalf("event %d mismatch: got %+v, want %+v", i, g, w)
		}
		if (g.err == nil) != (w.err == nil) {
			t.Fatalf("event %d error mismatch: got %v, want %v", i, g.err, w.err)
		}
		if g.err != nil && w.err != nil && g.err.Error() != w.err.Error() {
			t.Fatalf("event %d error mismatch: got %v, want %v", i, g.err, w.err)
		}
	}
}
