// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"strings"
	"testing"
)

func TestForeignJSONLenient(t *testing.T) {
	var ev struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	in := []byte("{\"TYPE\":\"a\",\"name\":\"x\",\"name\":\"y\\u0000\xff\"}")
	if err := json.Unmarshal(in, &ev, foreignJSON); err != nil {
		t.Fatal(err)
	}
	if ev.Type != "a" || !strings.HasPrefix(ev.Name, "y") {
		t.Fatalf("got %+v", ev)
	}
}

func TestStreamJSONDuplicateKeys(t *testing.T) {
	var got []string
	err := StreamJSON(strings.NewReader("{\"a\":1,\"a\":2}\n"), func(raw jsontext.Value) error {
		got = append(got, string(raw))
		return nil
	})
	if err != nil || len(got) != 1 {
		t.Fatalf("err=%v got=%v", err, got)
	}
}
