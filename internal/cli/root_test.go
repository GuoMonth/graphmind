package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/senguoyun-guosheng/graphmind/internal/model"
)

func TestNewJSONEncoderDoesNotEscapeAngleBrackets(t *testing.T) {
	var out bytes.Buffer
	env := model.Envelope{
		OK:        true,
		Summary:   "Listed 0 nodes.",
		NextSteps: []string{"gm cat <id>  — inspect a specific node"},
	}

	if err := newJSONEncoder(&out).Encode(env); err != nil {
		t.Fatalf("encode json: %v", err)
	}

	got := out.String()
	if strings.Contains(got, `\u003c`) || strings.Contains(got, `\u003e`) {
		t.Fatalf("expected angle brackets to remain unescaped, got %q", got)
	}
	if !strings.Contains(got, `"gm cat <id>  — inspect a specific node"`) {
		t.Fatalf("expected literal angle brackets in output, got %q", got)
	}
}

func TestNewJSONEncoderPreservesUserUnicodeText(t *testing.T) {
	var out bytes.Buffer
	env := model.Envelope{
		OK:        true,
		Data:      map[string]string{"title": "中文节点", "description": "用户输入：上海会议"},
		Summary:   "Retrieved 1 node.",
		NextSteps: []string{"gm cat <id>  — inspect the node"},
	}

	if err := newJSONEncoder(&out).Encode(env); err != nil {
		t.Fatalf("encode json: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, `"summary":"Retrieved 1 node."`) {
		t.Fatalf("expected English summary in output, got %q", got)
	}
	if !strings.Contains(got, `"gm cat <id>  — inspect the node"`) {
		t.Fatalf("expected English next step in output, got %q", got)
	}
	if !strings.Contains(got, `"title":"中文节点"`) {
		t.Fatalf("expected Chinese user content in output, got %q", got)
	}
	if !strings.Contains(got, `"description":"用户输入：上海会议"`) {
		t.Fatalf("expected Chinese user description in output, got %q", got)
	}
}
