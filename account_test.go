package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

func TestFetchAccountTokensPaginates(t *testing.T) {
	app := NewApp()
	var requestedPages []string
	app.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/token/" {
			return nil, fmt.Errorf("unexpected path: %s", request.URL.Path)
		}
		page := request.URL.Query().Get("p")
		requestedPages = append(requestedPages, page)
		items := make([]dashboardToken, 0, 100)
		if page == "1" {
			for id := 1000; id > 900; id-- {
				items = append(items, dashboardToken{ID: id, Name: fmt.Sprintf("key-%d", id)})
			}
		} else if page == "2" {
			items = append(items, dashboardToken{ID: 900, Name: "key-900"})
		} else {
			return nil, fmt.Errorf("unexpected page: %s", page)
		}
		return testDashboardResponse(request, map[string]any{
			"page":      page,
			"page_size": 100,
			"total":     101,
			"items":     items,
		}), nil
	})}

	tokens, err := app.fetchAccountTokens("dashboard-session")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 101 || tokens[len(tokens)-1].ID != 900 {
		t.Fatalf("unexpected paginated tokens: len=%d last=%#v", len(tokens), tokens[len(tokens)-1])
	}
	if strings.Join(requestedPages, ",") != "1,2" {
		t.Fatalf("requested pages = %v, want [1 2]", requestedPages)
	}
}

func TestFetchAccountTokenKeysSplitsBatches(t *testing.T) {
	app := NewApp()
	var batchSizes []int
	app.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/token/batch/keys" {
			return nil, fmt.Errorf("unexpected path: %s", request.URL.Path)
		}
		var payload struct {
			IDs []int `json:"ids"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			return nil, err
		}
		batchSizes = append(batchSizes, len(payload.IDs))
		keys := make(map[string]string, len(payload.IDs))
		for _, id := range payload.IDs {
			keys[fmt.Sprint(id)] = fmt.Sprintf("secret-%d", id)
		}
		return testDashboardResponse(request, map[string]any{"keys": keys}), nil
	})}

	ids := make([]int, 205)
	for index := range ids {
		ids[index] = index + 1
	}
	keys, err := app.fetchAccountTokenKeys("dashboard-session", ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != len(ids) || keys[1] != "secret-1" || keys[205] != "secret-205" {
		t.Fatalf("unexpected keys: len=%d first=%q last=%q", len(keys), keys[1], keys[205])
	}
	if fmt.Sprint(batchSizes) != "[100 100 5]" {
		t.Fatalf("batch sizes = %v, want [100 100 5]", batchSizes)
	}
}

func TestCreateToolKeyAcceptsSuccessfulResponseWithoutID(t *testing.T) {
	app := NewApp()
	app.account = dashboardSession{AccessToken: "dashboard-session", ExpiresAt: time.Now().Add(time.Hour)}
	var createPayload map[string]any
	var listCalls int
	app.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/user/self/groups":
			return testDashboardResponse(request, map[string]any{"gpt": map[string]any{"desc": "GPT", "ratio": 1}}), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/user/models":
			return testDashboardResponse(request, []string{"gpt-5.6-terra"}), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/token/":
			listCalls++
			items := []dashboardToken(nil)
			if listCalls > 1 {
				items = []dashboardToken{{ID: 321, Name: automaticToolKeyName, Status: 1, ExpiredTime: -1, UnlimitedQuota: true, CreatedTime: time.Now().Unix()}}
			}
			return testDashboardResponse(request, map[string]any{"page": 1, "page_size": 100, "total": len(items), "items": items}), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/token/":
			if err := json.NewDecoder(request.Body).Decode(&createPayload); err != nil {
				return nil, err
			}
			return testDashboardResponse(request, nil), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/token/321/key":
			return testDashboardResponse(request, map[string]string{"key": "secret-created"}), nil
		case request.Method == http.MethodGet && request.URL.Path == "/v1/models":
			return testRawJSONResponse(request, `{"data":[{"id":"gpt-5.6-terra"}]}`), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	})}

	result, err := app.CreateToolKey(ToolKeyRequest{ClientID: "codex", Group: "gpt"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != automaticToolKeyName || result.Existing || result.ProvisionID == "" {
		t.Fatalf("unexpected created result: %#v", result)
	}
	if createPayload["name"] != automaticToolKeyName {
		t.Fatalf("create name = %#v, want %q", createPayload["name"], automaticToolKeyName)
	}
	if _, ok := createPayload["model_limits_enabled"].(bool); !ok {
		t.Fatalf("create payload missing model limits: %#v", createPayload)
	}
	if listCalls != 2 {
		t.Fatalf("token list calls = %d, want before/after", listCalls)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "secret-created") {
		t.Fatalf("created key leaked through result: %s", encoded)
	}
	app.provisionMu.Lock()
	provision := app.provisions[result.ProvisionID]
	app.provisionMu.Unlock()
	if provision.Key != "secret-created" {
		t.Fatalf("provision did not retain key in memory")
	}
}

func TestCreateToolKeyRecoversWhenCreateResponseReportsFailureAfterCommit(t *testing.T) {
	app := NewApp()
	app.account = dashboardSession{AccessToken: "dashboard-session", ExpiresAt: time.Now().Add(time.Hour)}
	var listCalls int
	app.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/user/self/groups":
			return testDashboardResponse(request, map[string]any{"gpt": map[string]any{"desc": "GPT", "ratio": 1}}), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/user/models":
			return testDashboardResponse(request, []string{"gpt-5.6-terra"}), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/token/":
			listCalls++
			items := []dashboardToken(nil)
			if listCalls > 1 {
				items = []dashboardToken{{ID: 654, Name: automaticToolKeyName, Status: 1, ExpiredTime: -1, UnlimitedQuota: true, CreatedTime: time.Now().Unix()}}
			}
			return testDashboardResponse(request, map[string]any{"page": 1, "page_size": 100, "total": len(items), "items": items}), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/token/":
			return testDashboardResponse(request, map[string]any{"message": "网关超时"}), fmt.Errorf("simulated timeout after commit")
		case request.Method == http.MethodPost && request.URL.Path == "/api/token/654/key":
			return testDashboardResponse(request, map[string]string{"key": "secret-recovered"}), nil
		case request.Method == http.MethodGet && request.URL.Path == "/v1/models":
			return testRawJSONResponse(request, `{"data":[{"id":"gpt-5.6-terra"}]}`), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	})}

	result, err := app.CreateToolKey(ToolKeyRequest{ClientID: "codex", Group: "gpt"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ProvisionID == "" || result.Name != automaticToolKeyName {
		t.Fatalf("unexpected recovered result: %#v", result)
	}
	app.provisionMu.Lock()
	provision := app.provisions[result.ProvisionID]
	app.provisionMu.Unlock()
	if provision.Key != "secret-recovered" {
		t.Fatalf("recovered key was not retained: %q", provision.Key)
	}
}

func TestGetAccountToolOptionsRecommendsExistingKeyWithoutLeakingIt(t *testing.T) {
	app := NewApp()
	app.account = dashboardSession{AccessToken: "dashboard-session", ExpiresAt: time.Now().Add(time.Hour)}
	app.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/user/self/groups":
			return testDashboardResponse(request, map[string]any{"gpt": map[string]any{"desc": "GPT", "ratio": 1}}), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/user/models":
			return testDashboardResponse(request, []string{"gpt-5.6-terra"}), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/token/":
			return testDashboardResponse(request, map[string]any{
				"page": 1, "page_size": 100, "total": 1,
				"items": []dashboardToken{{ID: 88, Name: "工作 Key", Status: 1, ExpiredTime: -1, UnlimitedQuota: true, Group: "gpt"}},
			}), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/token/batch/keys":
			return testDashboardResponse(request, map[string]any{"keys": map[string]string{"88": "secret-existing"}}), nil
		case request.Method == http.MethodGet && request.URL.Path == "/v1/models":
			return testRawJSONResponse(request, `{"data":[{"id":"gpt-5.6-terra"}]}`), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	})}

	options, err := app.GetAccountToolOptions("codex")
	if err != nil {
		t.Fatal(err)
	}
	if len(options.ExistingKeys) != 1 || options.ExistingKeys[0].Name != "工作 Key" || !options.ExistingKeys[0].Existing {
		t.Fatalf("unexpected existing key recommendation: %#v", options.ExistingKeys)
	}
	encoded, err := json.Marshal(options)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "secret-existing") {
		t.Fatalf("existing key leaked through options: %s", encoded)
	}
}

func testDashboardResponse(request *http.Request, data any) *http.Response {
	body, _ := json.Marshal(map[string]any{"success": true, "message": "", "data": data})
	return testRawJSONResponse(request, string(body))
}

func testRawJSONResponse(request *http.Request, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    request,
	}
}
