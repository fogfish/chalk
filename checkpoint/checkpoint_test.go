//
// Copyright (C) 2026 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/chalk
//

package checkpoint_test

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/fogfish/chalk/checkpoint"
)

type state struct {
	Offset int    `yaml:"offset"`
	Name   string `yaml:"name"`
}

func TestRecoverCommitRoundTrip(t *testing.T) {
	orig := checkpoint.Default()
	defer checkpoint.SetDefault(orig)

	dir := t.TempDir()
	checkpoint.SetDefault(checkpoint.New(dir))

	got := checkpoint.Recover("job", state{})
	if got != (state{}) {
		t.Fatalf("expected zero value before Commit, got %+v", got)
	}

	want := state{Offset: 42, Name: "alice"}
	if err := checkpoint.Commit("job", want); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	got = checkpoint.Recover("job", state{})
	if got != want {
		t.Fatalf("Recover: got %+v, want %+v", got, want)
	}
}

func TestDefaultCacheIsNoop(t *testing.T) {
	orig := checkpoint.Default()
	defer checkpoint.SetDefault(orig)

	checkpoint.SetDefault(checkpoint.New(""))
	def := state{Offset: 7}

	got := checkpoint.Recover("job", def)
	if got != def {
		t.Fatalf("Recover: got %+v, want %+v", got, def)
	}

	if err := checkpoint.Commit("job", state{Offset: 1}); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if _, err := os.Stat(""); err == nil {
		t.Fatalf("expected no file to be written for empty cache dir")
	}
}

func TestRecoverWarnsOnDecodeError(t *testing.T) {
	orig := checkpoint.Default()
	defer checkpoint.SetDefault(orig)

	dir := t.TempDir()

	var warnings []string
	checkpoint.SetDefault(checkpoint.New(dir, checkpoint.WithWarn(func(format string, args ...any) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
	})))

	hash := sha1.New()
	hash.Write([]byte("job"))
	path := filepath.Join(dir, fmt.Sprintf("%x.yaml", hash.Sum(nil)))
	if err := os.WriteFile(path, []byte("- not\n- a\n- struct\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	def := state{Offset: 7}
	got := checkpoint.Recover("job", def)
	if got != def {
		t.Fatalf("Recover: got %+v, want %+v", got, def)
	}

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
}
