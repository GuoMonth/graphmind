package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/senguoyun-guosheng/graphmind/internal/model"
)

const maxJSONStdinBytes int64 = 10 << 20

func readOptionalStdinBytes(stdin *os.File, maxBytes int64) ([]byte, error) {
	stat, err := stdin.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect stdin: %w", err)
	}
	if stat.Mode()&os.ModeCharDevice != 0 {
		return nil, nil
	}
	return readBoundedStdinBytes(stdin, maxBytes)
}

func readBoundedStdinBytes(reader io.Reader, maxBytes int64) ([]byte, error) {
	input, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read stdin: %s", model.ErrInvalidInput, err)
	}
	if int64(len(input)) > maxBytes {
		return nil, fmt.Errorf("%w: stdin exceeds %d MB limit", model.ErrInvalidInput, maxBytes/(1<<20))
	}
	return input, nil
}

func readOptionalJSONObjectFromStdin(stdin *os.File) (payload map[string]any, hasInput bool, err error) {
	input, err := readOptionalStdinBytes(stdin, maxJSONStdinBytes)
	if err != nil {
		return nil, false, err
	}
	if len(input) == 0 {
		return nil, false, nil
	}

	payload = make(map[string]any) // JSON input is untyped
	if err := json.Unmarshal(input, &payload); err != nil {
		return nil, false, fmt.Errorf("%w: invalid JSON: %s", model.ErrInvalidInput, err)
	}
	return payload, true, nil
}
