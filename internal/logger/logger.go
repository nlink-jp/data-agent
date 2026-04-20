package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Level represents log severity.
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Entry is a single structured log entry.
type Entry struct {
	Timestamp time.Time         `json:"timestamp"`
	Level     Level             `json:"level"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
}

// Field is a key-value pair for structured logging.
type Field struct {
	Key   string
	Value string
}

// F creates a Field.
func F(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Emitter is a callback for sending log entries to the frontend.
type Emitter func(entry Entry)

// Logger provides structured logging with file output and event emission.
type Logger struct {
	file    *os.File
	emitter Emitter
	level   Level
	mu      sync.Mutex
}

// New creates a Logger that writes to the given directory.
// The emitter callback is optional (can be nil).
func New(logDir string, emitter Emitter) (*Logger, error) {
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	path := filepath.Join(logDir, "data-agent.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return &Logger{
		file:    f,
		emitter: emitter,
		level:   LevelInfo,
	}, nil
}

// SetLevel sets the minimum log level.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// Close closes the log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

func (l *Logger) Debug(msg string, fields ...Field) { l.log(LevelDebug, msg, fields) }
func (l *Logger) Info(msg string, fields ...Field)  { l.log(LevelInfo, msg, fields) }
func (l *Logger) Warn(msg string, fields ...Field)  { l.log(LevelWarn, msg, fields) }
func (l *Logger) Error(msg string, fields ...Field) { l.log(LevelError, msg, fields) }

func (l *Logger) log(level Level, msg string, fields []Field) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.shouldLog(level) {
		return
	}

	entry := Entry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   msg,
	}
	if len(fields) > 0 {
		entry.Fields = make(map[string]string, len(fields))
		for _, f := range fields {
			entry.Fields[f.Key] = f.Value
		}
	}

	// Write to file
	if l.file != nil {
		data, err := json.Marshal(entry)
		if err == nil {
			l.file.Write(data)
			l.file.Write([]byte("\n"))
		}
	}

	// Emit to frontend
	if l.emitter != nil {
		l.emitter(entry)
	}
}

func (l *Logger) shouldLog(level Level) bool {
	return levelOrder(level) >= levelOrder(l.level)
}

func levelOrder(level Level) int {
	switch level {
	case LevelDebug:
		return 0
	case LevelInfo:
		return 1
	case LevelWarn:
		return 2
	case LevelError:
		return 3
	default:
		return 0
	}
}
