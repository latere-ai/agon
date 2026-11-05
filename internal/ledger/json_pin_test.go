// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package ledger

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestRecordWireBytes pins the attacks.jsonl line: a nil round_introduced is
// omitted, invalid UTF-8 from model text becomes U+FFFD, and HTML characters
// are written as is.
func TestRecordWireBytes(t *testing.T) {
	s := freshSession(t)
	r := Record{TS: time.Unix(0, 0).UTC(), AttackID: "a1", Aspect: "x", Claim: "a\xff<b>", Status: StatusOpen}
	if err := Append(s, r); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(s.Path("attacks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	b := string(raw)
	if strings.Contains(b, "round_introduced") {
		t.Fatalf("nil pointer must be omitted: %s", b)
	}
	if !strings.Contains(b, `"claim":"a`+"�"+`<b>"`) {
		t.Fatalf("claim bytes: %s", b)
	}
}
