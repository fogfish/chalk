//
// Copyright (C) 2026 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/chalk
//

// Package cli is a stdlib flag-based runner for chalk-based, task-oriented
// CLIs. It wires Reporter construction, the standard -I/-O/-o/-extension
// I/O flags, and a checkpoint cache (-cache) on top of
// github.com/fogfish/stream/spool.
package cli

import (
	"context"
	"flag"

	"github.com/fogfish/chalk"
	"github.com/fogfish/chalk/checkpoint"
	ispool "github.com/fogfish/chalk/rt/internal/spool"
	"github.com/fogfish/opts"
	"github.com/fogfish/stream/spool"
)

var (
	dirI        = flag.String("I", "", "input directory")
	dirO        = flag.String("O", "", "output directory")
	fileO       = flag.String("o", "", "output file")
	extOverride = flag.String("extension", "", "override file extension for output files")
	cacheDir    = flag.String("cache", "", "directory to cache checkpoints")
	flagNoTTY   = flag.Bool("no-tty", false, "disable TTY output (structured log mode)")
	flagNoColor = flag.Bool("no-color", false, "disable color output in TTY mode")
	flagSilent  = flag.Bool("silent", false, "disable all output")
)

// Init parses flags, builds a Reporter via chalk.New and installs it as
// chalk.Default(). It also wires the -cache checkpoint cache as
// checkpoint.Default(), used by Recover/Commit. Call this directly if you
// only need the reporter (e.g. you're not using Start).
func Init() *chalk.Reporter {
	flag.Parse()

	var copts []chalk.Option
	if *flagSilent {
		copts = append(copts, chalk.WithSilent())
	}
	if *flagNoTTY {
		copts = append(copts, chalk.WithTTY(false))
	}
	if *flagNoColor {
		copts = append(copts, chalk.WithColor(false))
	}

	r := chalk.New(copts...)
	chalk.SetDefault(r)

	checkpoint.SetDefault(checkpoint.New(*cacheDir, checkpoint.WithWarn(func(format string, args ...any) {
		chalk.Printf("unable to recover state: "+format, args...)
	})))

	return r
}

// Start parses flags, wires source/target via spool, runs f over every
// input, and calls Reporter.Quit().
func Start(f spool.Spooler) {
	r := Init()

	src, wlk, err := ispool.Source(*dirI, flag.CommandLine.Args())
	if err != nil {
		chalk.Panic(err)
	}

	dst, err := ispool.Target(*dirO, *fileO)
	if err != nil {
		chalk.Panic(err)
	}

	var sopts []opts.Option[spool.Spool]
	if *extOverride != "" {
		sopts = append(sopts, spool.WithFileExt(*extOverride))
	}
	ingress := spool.New(src, dst, sopts...)

	if err := ingress.ForEach(context.Background(), wlk, f); err != nil {
		chalk.Panic(err)
	}

	r.Quit()
}

// Recover loads a previously Commit-ed value for key from the -cache
// directory, or returns def if no cache is configured or the value is
// missing/unreadable. Init must have been called first.
func Recover[T any](key string, def T) T {
	return checkpoint.Recover(key, def)
}

// Commit persists val under key in the -cache directory. It is a no-op if
// -cache was not set. Init must have been called first.
func Commit[T any](key string, val T) error {
	return checkpoint.Commit(key, val)
}
