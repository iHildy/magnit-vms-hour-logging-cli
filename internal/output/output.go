package output

import (
	"encoding/json"
	"fmt"
	"io"
)

type ErrorPayload struct {
	OK      bool   `json:"ok"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func Write(w io.Writer, asJSON bool, human string, payload any) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}
	_, err := fmt.Fprintln(w, human)
	return err
}

func WriteJSON(w io.Writer, payload any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func NewErrorPayload(code, message string, details any) ErrorPayload {
	return ErrorPayload{
		OK:      false,
		Code:    code,
		Message: message,
		Details: details,
	}
}
