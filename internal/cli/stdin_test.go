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

func TestReadOptionalIDsFromJSONLStdinParsesIDs(t *testing.T) {
	file := mustTempInputFile(t, strings.Join([]string{
		`{"id":"019abc","title":"First"}`,
		`{"title":"missing id"}`,
		`not-json`,
		`{"id":"019def"}`,
		"",
	}, "\n"))

	got, err := readOptionalIDsFromJSONLStdin(file, 10)
	if err != nil {
		t.Fatalf("read optional JSONL ids: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ids = %v, want 2 entries", got)
	}
	if got[0] != "019abc" || got[1] != "019def" {
		t.Fatalf("ids = %v, want [019abc 019def]", got)
	}
}

func TestReadOptionalIDsFromJSONLStdinRejectsOversizedLine(t *testing.T) {
	file := mustTempInputFile(t, strings.Repeat("x", int(maxJSONStdinBytes)+1))

	_, err := readOptionalIDsFromJSONLStdin(file, 10)
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("err = %v, want invalid input", err)
	}
	if !strings.Contains(err.Error(), "JSONL line exceeds 10 MB limit") {
		t.Fatalf("err = %v, want JSONL line limit error", err)
	}
}

func TestReadOptionalIDsFromJSONLStdinRejectsTooManyIDs(t *testing.T) {
	file := mustTempInputFile(t, `{"id":"019abc"}`+"\n"+`{"id":"019def"}`)

	_, err := readOptionalIDsFromJSONLStdin(file, 1)
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("err = %v, want invalid input", err)
	}
	if !strings.Contains(err.Error(), "too many IDs") {
		t.Fatalf("err = %v, want too many IDs", err)
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
