package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriterWritesJSON(t *testing.T) {
	var out bytes.Buffer
	writer, err := New(Options{Format: "json", Out: &out})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	value := []string{"ark_123"}
	if err := writer.Write(value, Table{}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	var got []string
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}

	if len(got) != 1 || got[0] != "ark_123" {
		t.Fatalf("JSON output = %v, want %v", got, value)
	}
}

func TestWriterWritesTable(t *testing.T) {
	var out bytes.Buffer
	writer, err := New(Options{Format: "table", Out: &out})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	table := Table{
		Headers: []string{"ID", "NAME"},
		Rows:    [][]string{{"ark_123", "investigate-deploy"}},
	}
	if err := writer.Write(nil, table); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	for _, value := range []string{"ID", "NAME", "ark_123", "investigate-deploy"} {
		if !strings.Contains(out.String(), value) {
			t.Fatalf("table output = %q, want %q", out.String(), value)
		}
	}
}

func TestNewRejectsUnsupportedFormat(t *testing.T) {
	_, err := New(Options{Format: "yaml", Out: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("New() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "unsupported output format") {
		t.Fatalf("New() error = %q, want unsupported format error", err)
	}
}
