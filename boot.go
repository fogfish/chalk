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
	"os"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/fogfish/opts"
	"github.com/fogfish/stream/spool"
)

const ctxkey = "io.console.chalkboard"

var stdout *Reporter
var noTTY bool
var noColor bool
var silent bool

// NoTTY forces log-mode output (structured slog records, no colour, no spinner)
// even when stderr is an interactive terminal. Must be called before Start.
func NoTTY() { noTTY = true }

// NoColor disables color output in TTY mode, using plain black & white styles.
// Must be called before Start.
func NoColor() { noColor = true }

// Silent disables all output, including errors. Must be called before Start.
func Silent() { silent = true }

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

	if *flagSilent {
		silent = true
	}

	r := &Reporter{}
	start := time.Now()
	switch {
	case silent:
		r.p = &silentPrinter{}
	case noTTY:
		r.p = &logPrinter{}
	case term.IsTerminal(os.Stderr.Fd()):
		if noColor {
			bwStyles()
		}
		r.p = newTTYPrinter(os.Stderr, start, &r.mu)
	default:
		r.p = &logPrinter{}
	}

	stdout = r
	return r
}

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

	var opts []opts.Option[spool.Spool]
	if extOverride != nil && *extOverride != "" {
		opts = append(opts, spool.WithFileExt(*extOverride))
	}
	ingress := spool.New(src, dst, opts...)

	///lint:ignore SA1029: We use string keys to allow zero-dep discovery
	ctx := context.WithValue(context.Background(), ctxkey, stdout)

	if err = ingress.ForEach(ctx, wlk, f); err != nil {
		Panic(err)
	}

	stdout.Quit()
}
