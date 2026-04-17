package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/senguoyun-guosheng/graphmind/internal/model"
)

const maxJSONStdinBytes int64 = 10 << 20

func hasPipedStdin(stdin *os.File) (bool, error) {
	stat, err := stdin.Stat()
	if err != nil {
		return false, fmt.Errorf("inspect stdin: %w", err)
	}
	return stat.Mode()&os.ModeCharDevice == 0, nil
}

func readOptionalStdinBytes(stdin *os.File, maxBytes int64) ([]byte, error) {
	hasInput, err := hasPipedStdin(stdin)
	if err != nil {
		return nil, err
	}
	if !hasInput {
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

func readOptionalIDsFromJSONLStdin(stdin *os.File, maxIDs int) ([]string, error) {
	hasInput, err := hasPipedStdin(stdin)
	if err != nil {
		return nil, err
	}
	if !hasInput {
		return nil, nil
	}

	maxLineBytes := int(maxJSONStdinBytes)
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	ids := make([]string, 0)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var payload struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(line, &payload); err != nil {
			continue
		}
		if payload.ID == "" {
			continue
		}

		ids = append(ids, payload.ID)
		if len(ids) > maxIDs {
			return nil, fmt.Errorf("%w: too many IDs (max %d)", model.ErrInvalidInput, maxIDs)
		}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, fmt.Errorf("%w: JSONL line exceeds %d MB limit", model.ErrInvalidInput, maxLineBytes/(1<<20))
		}
		return nil, fmt.Errorf("%w: read stdin: %s", model.ErrInvalidInput, err)
	}

	return ids, nil
}
