package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewAndClose(t *testing.T) {
	dir := t.TempDir()
	l, err := New(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	logPath := filepath.Join(dir, "data-agent.log")
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("log file not created: %v", err)
	}
}

func TestLogLevels(t *testing.T) {
	dir := t.TempDir()
	l, err := New(dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	l.SetLevel(LevelWarn)
	l.Debug("should be filtered")
	l.Info("should be filtered")
	l.Warn("warning message")
	l.Error("error message")
	l.Close()

	data, err := os.ReadFile(filepath.Join(dir, "data-agent.log"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if strings.Contains(content, "should be filtered") {
		t.Error("debug/info messages should be filtered at warn level")
	}
	if !strings.Contains(content, "warning message") {
		t.Error("warn message should be logged")
	}
	if !strings.Contains(content, "error message") {
		t.Error("error message should be logged")
	}
}

func TestStructuredFields(t *testing.T) {
	dir := t.TempDir()
	l, err := New(dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	l.Info("test message", F("key1", "value1"), F("key2", "value2"))
	l.Close()

	data, err := os.ReadFile(filepath.Join(dir, "data-agent.log"))
	if err != nil {
		t.Fatal(err)
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("parse log entry: %v", err)
	}

	if entry.Message != "test message" {
		t.Errorf("message = %q, want %q", entry.Message, "test message")
	}
	if entry.Level != LevelInfo {
		t.Errorf("level = %q, want %q", entry.Level, LevelInfo)
	}
	if entry.Fields["key1"] != "value1" {
		t.Errorf("field key1 = %q, want %q", entry.Fields["key1"], "value1")
	}
}

func TestEmitter(t *testing.T) {
	dir := t.TempDir()

	var emitted []Entry
	emitter := func(entry Entry) {
		emitted = append(emitted, entry)
	}

	l, err := New(dir, emitter)
	if err != nil {
		t.Fatal(err)
	}

	l.Info("emitted message")
	l.Warn("emitted warning")
	l.Close()

	if len(emitted) != 2 {
		t.Fatalf("emitted %d entries, want 2", len(emitted))
	}
	if emitted[0].Message != "emitted message" {
		t.Errorf("emitted[0].Message = %q, want %q", emitted[0].Message, "emitted message")
	}
	if emitted[1].Level != LevelWarn {
		t.Errorf("emitted[1].Level = %q, want %q", emitted[1].Level, LevelWarn)
	}
}

func TestNoFieldsOmitted(t *testing.T) {
	dir := t.TempDir()
	l, err := New(dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	l.Info("no fields")
	l.Close()

	data, err := os.ReadFile(filepath.Join(dir, "data-agent.log"))
	if err != nil {
		t.Fatal(err)
	}

	// Fields should be omitted from JSON when empty
	if strings.Contains(string(data), `"fields"`) {
		t.Error("empty fields should be omitted from JSON")
	}
}
