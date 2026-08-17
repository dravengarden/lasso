package cliformat

import "encoding/json"

// jsonMarshal returns indented JSON. Two-space indent matches the
// pre-existing `json.MarshalIndent(v, "", "  ")` call shape across
// the migrated commands.
func jsonMarshal(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
