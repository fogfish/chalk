//
// Copyright (C) 2026 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/chalk
//

package spool_test

import (
	"os"
	"path/filepath"
	"testing"

	ispool "github.com/fogfish/chalk/rt/internal/spool"
)

func TestSourceLocalDir(t *testing.T) {
	dir := t.TempDir()

	fs, wlk, err := ispool.Source(dir, nil)
	if err != nil {
		t.Fatalf("Source: %v", err)
	}
	if fs == nil || wlk == nil {
		t.Fatalf("expected non-nil filesystem and walker")
	}
}

func TestSourceFileArgs(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(file, []byte("data"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	fs, wlk, err := ispool.Source("", []string{file})
	if err != nil {
		t.Fatalf("Source: %v", err)
	}
	if fs == nil || wlk == nil {
		t.Fatalf("expected non-nil filesystem and walker")
	}
}

func TestTargetLocalDir(t *testing.T) {
	dir := t.TempDir()

	fs, err := ispool.Target(dir, "")
	if err != nil {
		t.Fatalf("Target: %v", err)
	}
	if fs == nil {
		t.Fatalf("expected non-nil filesystem")
	}
}

func TestTargetOutputFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "output.txt")

	fs, err := ispool.Target("", file)
	if err != nil {
		t.Fatalf("Target: %v", err)
	}
	if fs == nil {
		t.Fatalf("expected non-nil filesystem")
	}

	if _, err := os.Stat(file); err != nil {
		t.Fatalf("expected output file to be created: %v", err)
	}
}
