package logging

import (
	"encoding/json"
	"strings"
)

type logEvent struct {
	Time      string          `json:"time"`
	ID        string          `json:"id,omitempty"`
	Category  string          `json:"category,omitempty"`
	Direction string          `json:"direction,omitempty"`
	Message   string          `json:"message"`
	Raw       json.RawMessage `json:"raw,omitempty"`
	Method    string          `json:"method,omitempty"`
	Path      string          `json:"path,omitempty"`
	Query     string          `json:"query,omitempty"`
	Host      string          `json:"host,omitempty"`
	Proto     string          `json:"proto,omitempty"`
	UserAgent string          `json:"userAgent,omitempty"`
	Referer   string          `json:"referer,omitempty"`
	Status    int             `json:"status,omitempty"`
	Remote    string          `json:"remote,omitempty"`
	Response  int64           `json:"responseBytes,omitempty"`
	Duration  int64           `json:"durationMs,omitempty"`
}

type logPayload struct {
	LogEvents []logEvent `json:"logevents"`
}

func encodeLogRaw(raw string) json.RawMessage {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	data := []byte(raw)
	if json.Valid(data) {
		return json.RawMessage(data)
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	return encoded
}
