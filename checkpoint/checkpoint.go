//
// Copyright (C) 2026 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/chalk
//

// Package checkpoint provides a small file-based cache for resumable,
// task-oriented CLIs: Recover loads a previously Commit-ed value by key,
// or returns the supplied default if no cache is configured or the value
// is missing/unreadable.
package checkpoint

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"github.com/goccy/go-yaml"
)

// Cache is a directory-backed, SHA1-keyed YAML cache. The zero value and a
// nil *Cache are both safe to use: Recover returns the default value and
// Commit is a no-op.
type Cache struct {
	dir  string
	warn func(format string, args ...any)
}

// Option configures a Cache.
type Option func(*Cache)

// WithWarn registers a callback invoked when Recover cannot read or decode
// a cached value, so the caller can surface this to the user (e.g. via
// chalk.Printf).
func WithWarn(f func(format string, args ...any)) Option {
	return func(c *Cache) { c.warn = f }
}

// New returns a Cache rooted at dir. If dir == "", the returned Cache is a
// no-op: Recover always returns the default value, Commit always returns
// nil.
func New(dir string, opts ...Option) *Cache {
	c := &Cache{dir: dir}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// defaultCache is the package-level Cache used by Recover/Commit. It is a
// no-op until SetDefault installs a Cache configured for the environment
// (e.g. rt/cli, rt/cobra wire it up from the -cache flag).
var defaultCache *Cache = New("")

// Default returns the package-level Cache used by Recover/Commit.
func Default() *Cache { return defaultCache }

// SetDefault replaces the package-level Cache used by Recover/Commit.
func SetDefault(c *Cache) { defaultCache = c }

func (c *Cache) path(key string) string {
	hash := sha1.New()
	hash.Write([]byte(key))
	return filepath.Join(c.dir, fmt.Sprintf("%x.yaml", hash.Sum(nil)))
}

func (c *Cache) warnf(format string, args ...any) {
	if c.warn != nil {
		c.warn(format, args...)
	}
}

// Recover loads the value previously stored under key by Commit, using the
// default Cache (see Default/SetDefault). It returns def if no cache
// directory is configured, the key is unknown, or the cached value cannot
// be read or decoded (in the latter cases, the optional WithWarn callback
// is invoked).
func Recover[T any](key string, def T) T {
	return recover_(defaultCache, key, def)
}

// Commit persists val under key in the default Cache (see
// Default/SetDefault). It is a no-op if no cache directory is configured.
func Commit[T any](key string, val T) error {
	return commit_(defaultCache, key, val)
}

func recover_[T any](c *Cache, key string, def T) T {
	if c == nil || c.dir == "" {
		return def
	}

	path := c.path(key)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return def
	}

	fd, err := os.Open(path)
	if err != nil {
		c.warnf("checkpoint %q: %s", key, err)
		return def
	}
	defer fd.Close()

	b, err := io.ReadAll(fd)
	if err != nil {
		c.warnf("checkpoint %q: %s", key, err)
		return def
	}

	var val T
	rv := reflect.ValueOf(&val).Elem()
	if rv.Kind() == reflect.Ptr {
		rv.Set(reflect.New(rv.Type().Elem()))
		if err := yaml.Unmarshal(b, rv.Interface()); err != nil {
			c.warnf("checkpoint %q: %s", key, err)
			return def
		}
	} else {
		if err := yaml.Unmarshal(b, &val); err != nil {
			c.warnf("checkpoint %q: %s", key, err)
			return def
		}
	}

	return val
}

// commit_ persists val under key in c. It is a no-op if c is nil or no
// cache directory is configured.
func commit_[T any](c *Cache, key string, val T) error {
	if c == nil || c.dir == "" {
		return nil
	}

	if err := os.MkdirAll(c.dir, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	fd, err := os.Create(c.path(key))
	if err != nil {
		return err
	}
	defer fd.Close()

	return yaml.NewEncoder(fd, yaml.UseLiteralStyleIfMultiline(true)).Encode(val)
}
