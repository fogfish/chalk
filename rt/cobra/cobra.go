//
// Copyright (C) 2026 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/chalk
//

// Package cobra integrates chalk with github.com/spf13/cobra (or any other
// flag library exposing a pflag.FlagSet-shaped API) without importing
// cobra/pflag directly.
package cobra

import (
	"github.com/fogfish/chalk"
	"github.com/fogfish/chalk/checkpoint"
	ispool "github.com/fogfish/chalk/rt/internal/spool"
	"github.com/fogfish/opts"
	"github.com/fogfish/stream/spool"
)

// FlagSet is the subset of (*pflag.FlagSet) used by BindFlags to register
// flags. *pflag.FlagSet (e.g. cmd.PersistentFlags()) and stdlib
// *flag.FlagSet both satisfy this interface.
type FlagSet interface {
	BoolVar(p *bool, name string, value bool, usage string)
	StringVar(p *string, name string, value string, usage string)
}

// Flags holds chalk's standard CLI flags, populated once fs (passed to
// BindFlags) has been parsed.
type Flags struct {
	NoTTY, NoColor, Silent          bool
	InputDir, OutputDir, OutputFile string
	Extension, Cache                string
}

// BindFlags registers chalk's standard flags on fs (e.g.
// cmd.PersistentFlags()) and returns a *Flags populated once fs is parsed:
//
//	f := chalkcobra.BindFlags(cmd.PersistentFlags())
//	// ... cmd.Execute() parses flags ...
//	r := f.Init()
func BindFlags(fs FlagSet) *Flags {
	f := &Flags{}
	fs.BoolVar(&f.NoTTY, "no-tty", false, "disable TTY output (structured log mode)")
	fs.BoolVar(&f.NoColor, "no-color", false, "disable color output in TTY mode")
	fs.BoolVar(&f.Silent, "silent", false, "disable all output")
	fs.StringVar(&f.InputDir, "input", "", "input directory")
	fs.StringVar(&f.OutputDir, "output", "", "output directory")
	fs.StringVar(&f.OutputFile, "out", "", "output file")
	fs.StringVar(&f.Extension, "extension", "", "override file extension for output files")
	fs.StringVar(&f.Cache, "cache", "", "directory to cache checkpoints")
	return f
}

// Init builds and installs a Reporter from the parsed flag values, and
// wires the checkpoint cache as checkpoint.Default(), used by
// Recover/Commit. Call from cmd's RunE/PersistentPreRunE, after cobra has
// parsed flags.
func (f *Flags) Init() *chalk.Reporter {
	var copts []chalk.Option
	if f.Silent {
		copts = append(copts, chalk.WithSilent())
	}
	if f.NoTTY {
		copts = append(copts, chalk.WithTTY(false))
	}
	if f.NoColor {
		copts = append(copts, chalk.WithColor(false))
	}

	r := chalk.New(copts...)
	chalk.SetDefault(r)

	checkpoint.SetDefault(checkpoint.New(f.Cache, checkpoint.WithWarn(func(format string, args ...any) {
		chalk.Printf("unable to recover state: "+format, args...)
	})))

	return r
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

// Spool resolves the input/output file systems and walker from f, for use
// with github.com/fogfish/stream/spool. args are the positional file
// arguments (e.g. cmd.Flags().Args()).
func (f *Flags) Spool(args []string) (src spool.FileSystem, wlk spool.Walker, dst spool.FileSystem, err error) {
	src, wlk, err = ispool.Source(f.InputDir, args)
	if err != nil {
		return nil, nil, nil, err
	}

	dst, err = ispool.Target(f.OutputDir, f.OutputFile)
	if err != nil {
		return nil, nil, nil, err
	}

	return src, wlk, dst, nil
}

// SpoolOptions returns the github.com/fogfish/stream/spool options derived
// from f (currently: -extension), for use with spool.New.
func (f *Flags) SpoolOptions() []opts.Option[spool.Spool] {
	if f.Extension == "" {
		return nil
	}
	return []opts.Option[spool.Spool]{spool.WithFileExt(f.Extension)}
}
