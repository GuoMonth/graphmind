package cli

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/senguoyun-guosheng/graphmind/internal/model"
)

func TestReadBoundedStdinBytesReturnsInput(t *testing.T) {
	input := []byte(`{"type":"event","title":"Sprint planning"}`)

	got, err := readBoundedStdinBytes(bytes.NewReader(input), maxJSONStdinBytes)
	if err != nil {
		t.Fatalf("read bounded stdin: %v", err)
	}
	if !bytes.Equal(got, input) {
		t.Fatalf("stdin = %q, want %q", string(got), string(input))
	}
}

func TestReadBoundedStdinBytesRejectsOversizedInput(t *testing.T) {
	input := strings.Repeat("x", int(maxJSONStdinBytes)+1)

	_, err := readBoundedStdinBytes(strings.NewReader(input), maxJSONStdinBytes)
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("err = %v, want invalid input", err)
	}
	if !strings.Contains(err.Error(), "stdin exceeds 10 MB limit") {
		t.Fatalf("err = %v, want size limit error", err)
	}
}

func TestReadOptionalStdinBytesSkipsCharacterDevice(t *testing.T) {
	file, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open dev null: %v", err)
	}
	defer file.Close()

	got, err := readOptionalStdinBytes(file, maxJSONStdinBytes)
	if err != nil {
		t.Fatalf("read optional stdin: %v", err)
	}
	if got != nil {
		t.Fatalf("stdin = %q, want nil", string(got))
	}
}

func TestReadOptionalJSONObjectFromStdinParsesJSON(t *testing.T) {
	file := mustTempInputFile(t, `{"type":"event","title":"Sprint planning"}`)

	got, hasInput, err := readOptionalJSONObjectFromStdin(file)
	if err != nil {
		t.Fatalf("read optional json stdin: %v", err)
	}
	if !hasInput {
		t.Fatalf("expected stdin payload")
	}
	if got["type"] != "event" {
		t.Fatalf("type = %v, want event", got["type"])
	}
	if got["title"] != "Sprint planning" {
		t.Fatalf("title = %v, want Sprint planning", got["title"])
	}
}

func TestReadOptionalJSONObjectFromStdinRejectsInvalidJSON(t *testing.T) {
	file := mustTempInputFile(t, `{"type":"event"`)

	_, _, err := readOptionalJSONObjectFromStdin(file)
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("err = %v, want invalid input", err)
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("err = %v, want invalid JSON", err)
	}
}

func TestReadOptionalJSONObjectFromStdinRejectsOversizedInput(t *testing.T) {
	file := mustTempInputFile(t, strings.Repeat("x", int(maxJSONStdinBytes)+1))

	_, _, err := readOptionalJSONObjectFromStdin(file)
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("err = %v, want invalid input", err)
	}
	if !strings.Contains(err.Error(), "stdin exceeds 10 MB limit") {
		t.Fatalf("err = %v, want size limit error", err)
	}
}

func mustTempInputFile(t *testing.T, content string) *os.File {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "stdin-*")
	if err != nil {
		t.Fatalf("create temp input file: %v", err)
	}
	if _, err := file.WriteString(content); err != nil {
		t.Fatalf("write temp input file: %v", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatalf("seek temp input file: %v", err)
	}
	t.Cleanup(func() {
		_ = file.Close()
	})
	return file
}
