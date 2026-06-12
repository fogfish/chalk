//
// Copyright (C) 2026 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/chalk
//

// Package spool resolves chalk's standard input/output routing
// (local directory, S3 bucket, explicit file list, stdin/stdout) to
// github.com/fogfish/stream/spool primitives. It is shared by rt/cli and
// rt/cobra.
package spool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fogfish/stream"
	"github.com/fogfish/stream/lfs"
	"github.com/fogfish/stream/spool"
	"github.com/fogfish/stream/stdio"
)

const s3pfx = "s3://"

// Source resolves the input file system and walker.
//
//   - inputDir starting with "s3://" -> S3 bucket, walk from "/"
//   - inputDir (local path)          -> local directory, walk from "/"
//   - len(args) != 0                 -> current directory, walk listed files
//   - otherwise                      -> stdin, walked as a single "STDIN" entry
func Source(inputDir string, args []string) (spool.FileSystem, spool.Walker, error) {
	if inputDir != "" && strings.HasPrefix(inputDir, s3pfx) {
		r, err := stream.NewFS(inputDir[len(s3pfx):])
		if err != nil {
			return nil, nil, err
		}
		return r, spool.WalkDir("/"), nil
	}

	if inputDir != "" {
		dir, err := filepath.Abs(inputDir)
		if err != nil {
			return nil, nil, err
		}
		r, err := lfs.New(dir)
		if err != nil {
			return nil, nil, err
		}
		return r, spool.WalkDir("/"), nil
	}

	if len(args) != 0 {
		dir, err := filepath.Abs(".")
		if err != nil {
			return nil, nil, err
		}
		r, err := lfs.New(dir)
		if err != nil {
			return nil, nil, err
		}
		return r, spool.WalkFiles(args), nil
	}

	fi, err := os.Stdin.Stat()
	if err == nil || ((fi.Mode() & os.ModeCharDevice) == 0) {
		r, err := stdio.New(os.Stdin, nil)
		if err != nil {
			return nil, nil, err
		}
		return r, spool.WalkFiles([]string{"STDIN"}), nil
	}

	return nil, nil, fmt.Errorf("no input")
}

// Target resolves the output file system.
//
//   - outputDir starting with "s3://" -> S3 bucket
//   - outputDir (local path)          -> local directory
//   - outputFile                      -> single file
//   - otherwise                       -> stdout
func Target(outputDir, outputFile string) (spool.FileSystem, error) {
	if outputDir != "" && strings.HasPrefix(outputDir, s3pfx) {
		w, err := stream.NewFS(outputDir[len(s3pfx):])
		if err != nil {
			return nil, err
		}
		return w, nil
	}

	if outputDir != "" {
		dir, err := filepath.Abs(outputDir)
		if err != nil {
			return nil, err
		}
		w, err := lfs.New(dir)
		if err != nil {
			return nil, err
		}
		return w, nil
	}

	if outputFile != "" {
		file, err := os.Create(outputFile)
		if err != nil {
			return nil, err
		}
		return stdio.New(nil, file)
	}

	return stdio.New(nil, os.Stdout)
}
