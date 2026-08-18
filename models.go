package main

import (
	"encoding/json"
	"io"
)

type modelPayload struct {
	Data   []Model         `json:"data"`
	Models []Model         `json:"models"`
	Error  json.RawMessage `json:"error"`
}

func decodeModelPayload(reader io.Reader) (ModelResponse, error) {
	var payload modelPayload
	decoder := json.NewDecoder(io.LimitReader(reader, 4*1024*1024))
	if err := decoder.Decode(&payload); err != nil {
		return ModelResponse{}, err
	}
	models := payload.Data
	if len(models) == 0 {
		models = payload.Models
	}
	message := ""
	if len(payload.Error) > 0 {
		var structured struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(payload.Error, &structured) == nil {
			message = structured.Message
		}
		if message == "" {
			var text string
			if json.Unmarshal(payload.Error, &text) == nil {
				message = text
			}
		}
	}
	return ModelResponse{Models: models, Message: message}, nil
}
