// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"
)

func TestStoredJSONBytes(t *testing.T) {
	v := map[string]any{"b": 1, "a": "x\xffy", "c": "<&>"}
	got, err := json.Marshal(v, StoredJSON, jsontext.WithIndent("  "))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"a\": \"x�y\",\n  \"b\": 1,\n  \"c\": \"<&>\"\n}"
	if string(got) != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
