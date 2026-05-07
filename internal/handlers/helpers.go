package handlers

import "encoding/json"

func marshalPayload(v any) (json.RawMessage, error) {
	return json.Marshal(v)
}
