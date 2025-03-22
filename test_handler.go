// Copyright 2021 FerretDB Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package flog

import (
	"io"
	"log/slog"
	"strings"
)

// TestingLog is a subset of testing.TB.
// It is used to avoid importing the testing package.
type TestingLog interface {
	Helper()
	Log(args ...any)
}

// testingOutput provides [io.Writer] for testing.TB.
type testingOutput struct {
	tl TestingLog
}

// TestingOutput returns a new [io.Writer] for testing.TB.
//
// It should not be used once https://github.com/golang/go/issues/59928 is resolved (probably in Go 1.25).
func TestingOutput(tl TestingLog) io.Writer {
	return &testingOutput{tl: tl}
}

// Write implements [io.Writer].
func (to *testingOutput) Write(p []byte) (int, error) {
	to.tl.Helper()

	to.tl.Log(strings.TrimSuffix(string(p), "\n"))
	return len(p), nil
}

// TestingLogger returns a slog test logger for the given level (which might be dynamic).
func TestingLogger(tl TestingLog, level slog.Leveler) *slog.Logger {
	h := NewConsoleHandler(TestingOutput(tl), &NewConsoleHandlerOpts{
		Level:        level,
		RemoveTime:   true,
		RemoveSource: true,
	})

	return slog.New(h)
}

// check interfaces
var (
	_ io.Writer = (*testingOutput)(nil)
)
