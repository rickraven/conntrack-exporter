package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Level is a log severity level.
type Level int

const (
	Debug Level = iota
	Info
	Warn
	Error
)

func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return Debug, nil
	case "info":
		return Info, nil
	case "warn", "warning":
		return Warn, nil
	case "error":
		return Error, nil
	default:
		return Info, fmt.Errorf("unknown log level %q", s)
	}
}

// Format is a log output format.
type Format string

const (
	Logfmt Format = "logfmt"
	JSON   Format = "json"
)

func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "logfmt":
		return Logfmt, nil
	case "json":
		return JSON, nil
	default:
		return Logfmt, fmt.Errorf("unknown log format %q", s)
	}
}

// Logger is a tiny structured logger used by this project to avoid pulling in
// a heavy logging dependency for a small exporter.
//
// All methods are safe for concurrent use.
type Logger struct {
	mu     sync.Mutex
	out    io.Writer
	level  Level
	format Format
}

func New(out io.Writer, level Level, format Format) *Logger {
	if out == nil {
		out = os.Stderr
	}
	return &Logger{out: out, level: level, format: format}
}

func (l *Logger) Debug(msg string, kv ...any) { l.log(Debug, msg, kv...) }
func (l *Logger) Info(msg string, kv ...any)  { l.log(Info, msg, kv...) }
func (l *Logger) Warn(msg string, kv ...any)  { l.log(Warn, msg, kv...) }
func (l *Logger) Error(msg string, kv ...any) { l.log(Error, msg, kv...) }

func (l *Logger) log(lvl Level, msg string, kv ...any) {
	if lvl < l.level {
		return
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	levelStr := levelString(lvl)

	l.mu.Lock()
	defer l.mu.Unlock()

	switch l.format {
	case JSON:
		m := map[string]any{
			"ts":    now,
			"level": levelStr,
			"msg":   msg,
		}
		addKV(m, kv...)
		b, _ := json.Marshal(m)
		_, _ = l.out.Write(append(b, '\n'))
	default:
		// logfmt-ish, not a full logfmt implementation, but adequate here.
		sb := strings.Builder{}
		sb.WriteString("ts=")
		sb.WriteString(escapeLogfmt(now))
		sb.WriteString(" level=")
		sb.WriteString(escapeLogfmt(levelStr))
		sb.WriteString(" msg=")
		sb.WriteString(escapeLogfmt(msg))
		for i := 0; i+1 < len(kv); i += 2 {
			k, ok := kv[i].(string)
			if !ok {
				continue
			}
			sb.WriteString(" ")
			sb.WriteString(k)
			sb.WriteString("=")
			sb.WriteString(escapeLogfmt(fmt.Sprint(kv[i+1])))
		}
		sb.WriteString("\n")
		_, _ = l.out.Write([]byte(sb.String()))
	}
}

func levelString(lvl Level) string {
	switch lvl {
	case Debug:
		return "debug"
	case Info:
		return "info"
	case Warn:
		return "warn"
	case Error:
		return "error"
	default:
		return "info"
	}
}

func addKV(m map[string]any, kv ...any) {
	for i := 0; i+1 < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			continue
		}
		m[k] = kv[i+1]
	}
}

func escapeLogfmt(s string) string {
	// Quote if contains spaces or special chars; keep it simple.
	if s == "" {
		return `""`
	}
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '"', '=':
			return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
		}
	}
	return s
}

