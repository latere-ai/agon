// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

// StoredJSON is the option set for documents written to the session
// directory: map keys are sorted so repeated writes are byte-identical, and
// strings that carry model or process output with invalid UTF-8 are written
// with U+FFFD instead of failing the write.
var StoredJSON = json.JoinOptions(
	json.Deterministic(true),
	jsontext.AllowInvalidUTF8(true),
)
