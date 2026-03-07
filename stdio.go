package chalk

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fogfish/stream"
	"github.com/fogfish/stream/lfs"
	"github.com/fogfish/stream/spool"
	"github.com/fogfish/stream/stdio"
)

var (
	dirI  = flag.String("I", "", "input directory")
	dirO  = flag.String("O", "", "output directory")
	fileO = flag.String("o", "", "output file")
)

func source() (spool.FileSystem, spool.Walker, error) {
	const s3pfx = "s3://"
	if *dirI != "" && strings.HasPrefix(*dirI, "s3://") {
		r, err := stream.NewFS((*dirI)[len(s3pfx):])
		if err != nil {
			return nil, nil, err
		}
		return r, spool.WalkDir("/"), nil
	}

	if *dirI != "" {
		dir, err := filepath.Abs(*dirI)
		if err != nil {
			return nil, nil, err
		}
		r, err := lfs.New(dir)
		if err != nil {
			return nil, nil, err
		}
		return r, spool.WalkDir("/"), nil
	}

	if len(flag.CommandLine.Args()) != 0 {
		dir, err := filepath.Abs(".")
		if err != nil {
			return nil, nil, err
		}
		r, err := lfs.New(dir)
		if err != nil {
			return nil, nil, err
		}
		return r, spool.WalkFiles(flag.CommandLine.Args()), nil
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

func target() (spool.FileSystem, error) {
	const s3pfx = "s3://"
	if *dirO != "" && strings.HasPrefix(*dirO, "s3://") {
		w, err := stream.NewFS((*dirO)[len(s3pfx):])
		if err != nil {
			return nil, err
		}
		return w, nil
	}

	if *dirO != "" {
		dir, err := filepath.Abs(*dirO)
		if err != nil {
			return nil, err
		}
		w, err := lfs.New(dir)
		if err != nil {
			return nil, err
		}
		return w, nil
	}

	if *fileO != "" {
		file, err := os.Create(*fileO)
		if err != nil {
			return nil, err
		}
		return stdio.New(nil, file)
	}

	return stdio.New(nil, os.Stdout)
}
