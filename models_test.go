package main

import (
	"strings"
	"testing"
)

func TestDecodeModelPayloadSupportsOpenAIAndGeminiShapes(t *testing.T) {
	openAI, err := decodeModelPayload(strings.NewReader(`{"data":[{"id":"gpt-test","owned_by":"test"}]}`))
	if err != nil || len(openAI.Models) != 1 || openAI.Models[0].ID != "gpt-test" {
		t.Fatalf("unexpected OpenAI model payload: %#v, %v", openAI, err)
	}
	gemini, err := decodeModelPayload(strings.NewReader(`{"models":[{"id":"gemini-test"}]}`))
	if err != nil || len(gemini.Models) != 1 || gemini.Models[0].ID != "gemini-test" {
		t.Fatalf("unexpected Gemini model payload: %#v, %v", gemini, err)
	}
}

func TestDecodeModelPayloadExtractsErrorMessage(t *testing.T) {
	response, err := decodeModelPayload(strings.NewReader(`{"error":{"message":"invalid key"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if response.Message != "invalid key" {
		t.Fatalf("unexpected error message: %q", response.Message)
	}
}
