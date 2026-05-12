package data

import "encoding/json"

// normalizeJSONArray ensures a json.RawMessage representing a list is never
// null or empty — both become []. This avoids forcing API consumers to
// distinguish "no value" from "empty list".
func normalizeJSONArray(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("[]")
	}
	return raw
}

// normalizeJSONObject ensures a json.RawMessage representing an object is
// never null or empty — both become {}.
func normalizeJSONObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("{}")
	}
	return raw
}
