package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestFilterModelsForClient(t *testing.T) {
	models := []Model{
		{ID: "gpt-5.6-sol"},
		{ID: "claude-sonnet-4-5"},
		{ID: "gemini-2.5-pro"},
		{ID: "grok-4"},
		{ID: "gpt-5.6-sol"},
	}
	cases := map[string][]string{
		"claude":   {"claude-sonnet-4-5"},
		"codex":    {"gpt-5.6-sol"},
		"gemini":   {"gemini-2.5-pro"},
		"grok":     {"grok-4"},
		"opencode": {"claude-sonnet-4-5", "gemini-2.5-pro", "gpt-5.6-sol", "grok-4"},
		"openclaw": {"claude-sonnet-4-5", "gemini-2.5-pro", "gpt-5.6-sol", "grok-4"},
		"hermes":   {"claude-sonnet-4-5", "gemini-2.5-pro", "gpt-5.6-sol", "grok-4"},
	}
	for clientID, expected := range cases {
		filtered := filterModelsForClient(clientID, models)
		if len(filtered) != len(expected) {
			t.Fatalf("%s: got %#v, want %#v", clientID, filtered, expected)
		}
		for index, model := range filtered {
			if model.ID != expected[index] {
				t.Fatalf("%s: model %d = %q, want %q", clientID, index, model.ID, expected[index])
			}
		}
	}
}

func TestDecodeDashboardModelsSupportsNameLists(t *testing.T) {
	models, err := decodeDashboardModels(json.RawMessage(`["gpt-5.6-sol","claude-sonnet-4-5","gpt-5.6-sol"]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0].ID != "claude-sonnet-4-5" || models[1].ID != "gpt-5.6-sol" {
		t.Fatalf("unexpected models: %#v", models)
	}
}

func TestSolveAltchaChallengeProducesExpectedPayload(t *testing.T) {
	const salt = "test-salt?expires=9999999999"
	const number = 42
	sum := sha256.Sum256([]byte(salt + "42"))
	challenge := altchaChallenge{
		Algorithm: "SHA-256",
		Challenge: hex.EncodeToString(sum[:]),
		MaxNumber: 100,
		Salt:      salt,
		Signature: "signature",
	}
	encoded, err := solveAltchaChallenge(challenge)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	payload := altchaPayload{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Number != number || payload.Salt != salt || payload.Challenge != challenge.Challenge || payload.Signature != challenge.Signature {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestRefreshAccountStateLoadsUsernameAndFormatsBalance(t *testing.T) {
	app := NewApp()
	app.account = dashboardSession{
		AccessToken: "dashboard-session",
		Username:    "before-refresh",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	app.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer dashboard-session" && request.URL.Path == "/api/user/self" {
			t.Fatalf("missing dashboard authorization: %q", request.Header.Get("Authorization"))
		}
		var body string
		switch request.URL.Path {
		case "/api/user/self":
			body = `{"success":true,"data":{"username":"alice","quota":1250000}}`
		case "/api/status":
			if request.Header.Get("Authorization") != "" {
				t.Fatalf("public status request leaked authorization: %q", request.Header.Get("Authorization"))
			}
			body = `{"success":true,"data":{"quota_per_unit":500000,"quota_display_type":"CNY","usd_exchange_rate":7.3}}`
		default:
			t.Fatalf("unexpected dashboard request: %s", request.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}

	state, err := app.RefreshAccountState()
	if err != nil {
		t.Fatal(err)
	}
	if state.Username != "alice" || state.Quota != 1250000 || state.Balance != "¥18.25" {
		t.Fatalf("unexpected account state: %#v", state)
	}
}

func TestFormatAccountBalanceSupportsTokenDisplay(t *testing.T) {
	if got := formatAccountBalance(1250000, dashboardAccountStatus{QuotaDisplayType: "TOKENS"}); got != "1,250,000 额度" {
		t.Fatalf("balance = %q", got)
	}
}
