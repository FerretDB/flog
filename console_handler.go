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
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"sync"

	"golang.org/x/term"
)

// timeLayout is the format of date time used by the console handler.
const timeLayout = "2006-01-02T15:04:05.000Z0700"

// NewConsoleHandlerOpts represents [NewConsoleHandler] options.
type NewConsoleHandlerOpts struct {
	Level        slog.Leveler
	RemoveTime   bool
	RemoveLevel  bool
	RemoveSource bool
}

// consoleHandler is a [slog.Handler] that writes logs to the console.
// The format is intended to be more human-readable than [slog.TextHandler]'s logfmt.
// The format is not stable.
//
// See https://golang.org/s/slog-handler-guide.
type consoleHandler struct {
	opts *NewConsoleHandlerOpts
	esc  *term.EscapeCodes

	ga        []groupOrAttrs
	testAttrs map[string]any

	m   *sync.Mutex
	out io.Writer
}

// NewConsoleHandler creates a new console handler.
//
// If out is a valid tty, the consoleHandler will send colorized messages.
// If NO_COLOR environment variable is set colorized messages are disabled.
func NewConsoleHandler(out io.Writer, opts *NewConsoleHandlerOpts) *consoleHandler {
	ch := &consoleHandler{
		opts:      cmp.Or(opts, new(NewConsoleHandlerOpts)),
		testAttrs: nil,
		m:         new(sync.Mutex),
		out:       out,
	}

	if os.Getenv("NO_COLOR") == "" {
		f, _ := out.(*os.File)
		if f != nil && term.IsTerminal(int(f.Fd())) {
			ch.esc = term.NewTerminal(f, "").Escape
		}
	}

	return ch
}

// Enabled implements [slog.Handler].
func (ch *consoleHandler) Enabled(_ context.Context, l slog.Level) bool {
	minLevel := slog.LevelInfo
	if ch.opts.Level != nil {
		minLevel = ch.opts.Level.Level()
	}

	return l >= minLevel
}

// Handle implements [slog.Handler].
func (ch *consoleHandler) Handle(ctx context.Context, r slog.Record) error {
	var buf bytes.Buffer

	if !ch.opts.RemoveTime && !r.Time.IsZero() {
		t := r.Time.Format(timeLayout)
		buf.WriteString(t)
		buf.WriteRune('\t')

		if ch.testAttrs != nil {
			ch.testAttrs[slog.TimeKey] = t
		}
	}

	if !ch.opts.RemoveLevel {
		buf.WriteString(ch.colorizedLevel(r.Level))
		buf.WriteRune('\t')

		if ch.testAttrs != nil {
			ch.testAttrs[slog.LevelKey] = r.Level.String()
		}
	}

	if !ch.opts.RemoveSource {
		f, _ := runtime.CallersFrames([]uintptr{r.PC}).Next()
		if f.File != "" {
			s := ch.shortPath(f.File) + ":" + strconv.Itoa(f.Line)
			buf.WriteString(s)
			buf.WriteRune('\t')

			if ch.testAttrs != nil {
				ch.testAttrs[slog.SourceKey] = s
			}
		}
	}

	if r.Message != "" {
		buf.WriteString(r.Message)

		if ch.testAttrs != nil {
			ch.testAttrs[slog.MessageKey] = r.Message
		}
	}

	if m := attrs(r, ch.ga); len(m) > 0 {
		buf.WriteRune('\t')

		var b bytes.Buffer
		encoder := json.NewEncoder(&b)
		encoder.SetEscapeHTML(false)

		if err := encoder.Encode(m); err != nil {
			return err
		}

		buf.Write(bytes.TrimSuffix(b.Bytes(), []byte{'\n'}))

		if ch.testAttrs != nil {
			maps.Copy(ch.testAttrs, m)
		}
	}

	buf.WriteRune('\n')

	ch.m.Lock()
	defer ch.m.Unlock()

	_, err := buf.WriteTo(ch.out)
	return err
}

// WithAttrs implements [slog.Handler].
func (ch *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return ch
	}

	return &consoleHandler{
		opts:      ch.opts,
		ga:        append(slices.Clone(ch.ga), groupOrAttrs{attrs: attrs}),
		m:         ch.m,
		out:       ch.out,
		esc:       ch.esc,
		testAttrs: ch.testAttrs,
	}
}

// WithGroup implements [slog.Handler].
func (ch *consoleHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return ch
	}

	return &consoleHandler{
		opts:      ch.opts,
		ga:        append(slices.Clone(ch.ga), groupOrAttrs{group: name}),
		m:         ch.m,
		out:       ch.out,
		esc:       ch.esc,
		testAttrs: ch.testAttrs,
	}
}

// colorizedLevel returns colorized string representation of l [slog.Level].
// If ch is unable to print colorized messages, non-colorized string is returned.
func (ch *consoleHandler) colorizedLevel(l slog.Level) string {
	if ch.esc == nil {
		return l.String()
	}

	format := "%s%s%s"

	switch {
	case l < slog.LevelInfo:
		return fmt.Sprintf(format, ch.esc.Blue, l, ch.esc.Reset)
	case l < slog.LevelWarn:
		return fmt.Sprintf(format, ch.esc.Green, l, ch.esc.Reset)
	case l < slog.LevelError:
		return fmt.Sprintf(format, ch.esc.Yellow, l, ch.esc.Reset)
	default:
		return fmt.Sprintf(format, ch.esc.Red, l, ch.esc.Reset)
	}
}

// shortPath returns shorter path for the given path.
func (ch *consoleHandler) shortPath(path string) string {
	if path == "" {
		panic("empty path")
	}

	dir := filepath.Base(filepath.Dir(path))
	if dir == "/" {
		dir = ""
	} else {
		dir += "/"
	}

	return dir + filepath.Base(path)
}

// check interfaces
var (
	_ slog.Handler = (*consoleHandler)(nil)
)
