// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package input

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

// foreignJSON reads session transcripts written by other tools: duplicate
// keys, invalid UTF-8 and any letter case in field names are accepted, and
// the last duplicate wins.
var foreignJSON = json.JoinOptions(
	jsontext.AllowDuplicateNames(true),
	jsontext.AllowInvalidUTF8(true),
	json.MatchCaseInsensitiveNames(true),
)
