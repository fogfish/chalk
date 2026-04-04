//
// Copyright (C) 2026 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/chalk
//

package chalk

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"github.com/goccy/go-yaml"
)

func Recover[T any](key string, def T) T {
	if *cache == "" {
		return def
	}

	hash := sha1.New()
	hash.Write([]byte(key))
	hval := hash.Sum(nil)

	path := filepath.Join(*cache, fmt.Sprintf("%x.yaml", hval))
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return def
	}

	fd, err := os.Open(path)
	if err != nil {
		return def
	}
	defer fd.Close()

	b, err := io.ReadAll(fd)
	if err != nil {
		return def
	}

	var val T
	rv := reflect.ValueOf(&val).Elem()
	if rv.Kind() == reflect.Ptr {
		rv.Set(reflect.New(rv.Type().Elem()))
		if err := yaml.Unmarshal(b, rv.Interface()); err != nil {
			return def
		}
	} else {
		if err := yaml.Unmarshal(b, &val); err != nil {
			return def
		}
	}

	return val
}

func Commit[T any](key string, val T) {
	if *cache == "" {
		return
	}

	if err := os.MkdirAll(*cache, 0755); err != nil && !os.IsExist(err) {
		return
	}

	hash := sha1.New()
	hash.Write([]byte(key))
	hval := hash.Sum(nil)

	path := filepath.Join(*cache, fmt.Sprintf("%x.yaml", hval))
	fd, err := os.Create(path)
	if err != nil {
		return
	}
	defer fd.Close()

	yaml.NewEncoder(fd, yaml.UseLiteralStyleIfMultiline(true)).Encode(val)
}
