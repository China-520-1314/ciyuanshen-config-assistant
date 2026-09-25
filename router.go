package main

// The local Responses -> Chat bridge follows the architecture studied in
// farion1231/cc-switch. This Go implementation is independent of its Rust code.
import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pelletier/go-toml/v2"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const routerAlias = "gpt-5.6-terra"
const routerProvider = "ciyuanshen"

type RouterRequest struct {
	APIKey         string   `json:"apiKey"`
	ProvisionID    string   `json:"provisionId"`
	Model          string   `json:"model"`
	DefaultModel   string   `json:"defaultModel,omitempty"`
	Models         []string `json:"models,omitempty"`
	UseExistingKey bool     `json:"useExistingKey"`
	Client         string   `json:"client"` // codex or claude
	ClaudeOneM     bool     `json:"claudeOneM,omitempty"`
}
type RouterStatus struct {
	Running    bool     `json:"running"`
	Model      string   `json:"model"`
	Alias      string   `json:"alias"`
	Models     []string `json:"models,omitempty"`
	Address    string   `json:"address"`
	Requests   int      `json:"requests"`
	Failures   int      `json:"failures"`
	LastError  string   `json:"lastError"`
	BackupPath string   `json:"backupPath"`
}
type routerJournal struct {
	Path            string   `json:"path"`
	Original        []byte   `json:"original"`
	Installed       []byte   `json:"installed"`
	Existed         bool     `json:"existed"`
	Address         string   `json:"address"`
	CatalogPath     string   `json:"catalogPath,omitempty"`
	CatalogOriginal []byte   `json:"catalogOriginal,omitempty"`
	CatalogExisted  bool     `json:"catalogExisted,omitempty"`
	Models          []string `json:"models,omitempty"`
	DefaultModel    string   `json:"defaultModel,omitempty"`
}
type modelRouter struct {
	mu                   sync.Mutex
	status               RouterStatus
	server               *http.Server
	key, token, upstream string
	models               []string
	client               *http.Client
	journal              routerJournal
}

func routerJournalPath() string {
	return filepath.Join(filepath.Dir(backupRoot()), "router-recovery.json")
}

// Called while holding operation; do not acquire routerMu in this helper.
func checkRouterConfigurationUnlocked() error {
	if _, err := os.Stat(routerJournalPath()); err == nil {
		return errors.New("请先在模型路由页面停止并恢复配置，再修改客户端配置或恢复备份")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (a *App) routerKey(r RouterRequest) (string, error) {
	if strings.EqualFold(r.Client, "claude") && r.UseExistingKey && strings.TrimSpace(r.ProvisionID) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return readConfiguredClientAPIKey(home, "claude")
	}
	if strings.TrimSpace(r.ProvisionID) != "" {
		a.provisionMu.Lock()
		defer a.provisionMu.Unlock()
		p, ok := a.provisions[strings.TrimSpace(r.ProvisionID)]
		if !ok || p.Key == "" {
			return "", errors.New("所选分组 Key 已失效，请重新读取分组")
		}
		return p.Key, nil
	}
	if !r.UseExistingKey {
		if strings.TrimSpace(r.APIKey) == "" {
			return "", errors.New("请粘贴词元神 Key，或选择使用 Codex 当前 Key")
		}
		return strings.TrimSpace(r.APIKey), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	root, err := readTOMLMap(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		return "", err
	}
	// Never send a credential belonging to another provider to this gateway.
	provider := str(root["model_provider"])
	providers, _ := root["model_providers"].(map[string]any)
	p, _ := providers[provider].(map[string]any)
	if strings.TrimRight(str(p["base_url"]), "/") != defaultGatewayURL {
		return "", errors.New("Codex 当前未直连词元神，请粘贴要用于路由的词元神 Key")
	}
	return readConfiguredClientAPIKey(home, "codex")
}
func (a *App) GetRouterModels(r RouterRequest) (ModelResponse, error) {
	key, err := a.routerKey(r)
	if err != nil {
		return ModelResponse{}, err
	}
	return a.fetchGatewayModels(key)
}
func (a *App) GetModelRouterStatus() RouterStatus {
	a.routerMu.Lock()
	defer a.routerMu.Unlock()
	if a.router == nil {
		s := RouterStatus{Alias: routerAlias, BackupPath: routerJournalPath()}
		if journal, err := readRouterJournal(); err == nil {
			s.BackupPath = routerJournalPath()
			s.Address = "http://" + journal.Address
			s.Model, s.Models = routerModelsFromJournal(journal)
			s.Alias = s.Model
			if routerAddressReachable(journal.Address) {
				s.Running = true
				s.LastError = "路由仍在运行，但当前助手不是启动它的窗口；点击停止并恢复配置即可接管恢复"
			} else {
				s.LastError = "发现上次路由的恢复记录，请点击停止并恢复配置后再启动"
			}
		}
		return s
	}
	a.router.mu.Lock()
	defer a.router.mu.Unlock()
	return a.router.status
}

func readRouterJournal() (routerJournal, error) {
	raw, err := os.ReadFile(routerJournalPath())
	if err != nil {
		return routerJournal{}, err
	}
	var journal routerJournal
	if err := json.Unmarshal(raw, &journal); err != nil {
		return routerJournal{}, err
	}
	return journal, nil
}

func routerAddressReachable(address string) bool {
	conn, err := net.DialTimeout("tcp", address, 250*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func routerModelsFromJournal(j routerJournal) (string, []string) {
	if len(j.Models) > 0 {
		model := strings.TrimSpace(j.DefaultModel)
		if model == "" {
			model = strings.TrimSpace(j.Models[0])
		}
		return model, uniqueModelIDs(j.Models)
	}
	var root map[string]any
	if err := toml.Unmarshal(j.Installed, &root); err == nil {
		model := strings.TrimSpace(str(root["model"]))
		if model != "" {
			return model, []string{model}
		}
	}
	return routerAlias, []string{routerAlias}
}
func (a *App) StartModelRouter(r RouterRequest) (RouterStatus, error) {
	a.routerMu.Lock()
	defer a.routerMu.Unlock()
	a.operation.Lock()
	defer a.operation.Unlock()
	if a.router != nil {
		return RouterStatus{}, errors.New("路由已启动；更换模型前请先停止并恢复配置")
	}
	if err := checkRouterConfigurationUnlocked(); err != nil {
		return RouterStatus{}, err
	}
	key, err := a.routerKey(r)
	if err != nil {
		return RouterStatus{}, err
	}
	models, err := a.fetchGatewayModels(key)
	if err != nil {
		return RouterStatus{}, err
	}
	selectedModels := uniqueModelIDs(r.Models)
	if len(selectedModels) == 0 && strings.TrimSpace(r.Model) != "" {
		selectedModels = []string{strings.TrimSpace(r.Model)}
	}
	if len(selectedModels) == 0 {
		return RouterStatus{}, errors.New("请先选择至少一个模型")
	}
	for _, selected := range selectedModels {
		if !containsModel(models.Models, selected) {
			return RouterStatus{}, fmt.Errorf("所选模型 %q 不在这个 Key 的可用模型列表中", selected)
		}
	}
	primaryModel := selectedModels[0]
	defaultModel := strings.TrimSpace(r.DefaultModel)
	if defaultModel == "" {
		defaultModel = primaryModel
	}
	r.Model = defaultModel
	home, err := os.UserHomeDir()
	if err != nil {
		return RouterStatus{}, err
	}
	if custom := os.Getenv("CODEX_HOME"); custom != "" && filepath.Clean(custom) != filepath.Join(home, ".codex") {
		return RouterStatus{}, errors.New("检测到自定义 CODEX_HOME，请先恢复默认目录后使用一键路由")
	}
	client := strings.ToLower(strings.TrimSpace(r.Client))
	if client == "" {
		client = "codex"
	}
	if client != "codex" && client != "claude" {
		return RouterStatus{}, errors.New("不支持的路由客户端")
	}
	path := filepath.Join(home, ".codex", "config.toml")
	if client == "claude" {
		path = firstClientPath("claude", home)
	}
	original, err := os.ReadFile(path)
	existed := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return RouterStatus{}, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return RouterStatus{}, err
	}
	if client == "claude" {
		r.Models = selectedModels
		return a.startClaudeRouter(r, key, path, home, listener)
	}
	root, err := readTOMLMap(path)
	if err != nil {
		return RouterStatus{}, err
	}
	providers := ensureMap(root, "model_providers")
	token, err := createProvisionID()
	if err != nil {
		listener.Close()
		return RouterStatus{}, err
	}
	address := "http://" + listener.Addr().String()
	root["model_provider"] = routerProvider
	// Codex reads the selected model from config.toml before it queries the
	// provider. Write the mapped upstream model here so the client displays the
	// same name that the router sends upstream.
	root["model"] = defaultModel
	root["web_search"] = "disabled"
	root["disable_response_storage"] = true
	delete(root, "service_tier")
	delete(root, "model_reasoning_effort")
	delete(root, "profile")
	delete(root, "context_management")
	delete(root, "token_budget")
	features := ensureMap(root, "features")
	features["enable_request_compression"] = false
	delete(features, "context_management")
	providers[routerProvider] = map[string]any{"name": "词元神本地路由", "base_url": address + "/v1", "wire_api": "responses", "experimental_bearer_token": token, "supports_websockets": false}
	// Keep a stable provider entry after takeover is stopped so Codex can open
	// conversations created while routing was enabled.
	installed, err := marshalTOML(root)
	if err != nil {
		listener.Close()
		return RouterStatus{}, err
	}
	catalogPath := filepath.Join(filepath.Dir(path), "ciyuanshen-model-catalog.json")
	catalogOriginal, catalogErr := os.ReadFile(catalogPath)
	catalogExisted := catalogErr == nil
	if catalogErr != nil && !errors.Is(catalogErr, os.ErrNotExist) {
		listener.Close()
		return RouterStatus{}, catalogErr
	}
	catalogModels := make([]map[string]any, 0, len(selectedModels))
	for _, modelID := range selectedModels {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			continue
		}
		catalogModels = append(catalogModels, map[string]any{
			"slug":         modelID,
			"display_name": modelID,
			"description":  "词元神路由模型",
			"supported_reasoning_levels": []map[string]string{
				{"effort": "low", "description": "Low reasoning"},
				{"effort": "medium", "description": "Medium reasoning"},
				{"effort": "high", "description": "High reasoning"},
				{"effort": "xhigh", "description": "Extra high reasoning"},
				{"effort": "ultra", "description": "Ultra reasoning"},
				{"effort": "max", "description": "Maximum reasoning"},
			},
			"default_reasoning_level":          "medium",
			"context_window":                   114688,
			"max_context_window":               114688,
			"effective_context_window_percent": 100,
			"supports_reasoning_summaries":     true,
			"supports_parallel_tool_calls":     true,
			"input_modalities":                 []string{"text", "image"},
			"support_verbosity":                false,
			"visibility":                       "list",
			"supported_in_api":                 true,
			"priority":                         0,
			"is_default":                       modelID == defaultModel,
		})
	}
	catalog, _ := json.Marshal(map[string]any{"models": catalogModels})
	if err = atomicWrite(catalogPath, catalog); err != nil {
		listener.Close()
		return RouterStatus{}, err
	}
	root["model_catalog_json"] = "ciyuanshen-model-catalog.json"
	installed, err = marshalTOML(root)
	if err != nil {
		listener.Close()
		return RouterStatus{}, err
	}
	journal := routerJournal{Path: path, Original: original, Installed: installed, Existed: existed, Address: listener.Addr().String(), CatalogPath: catalogPath, CatalogOriginal: catalogOriginal, CatalogExisted: catalogExisted, Models: selectedModels, DefaultModel: defaultModel}
	encoded, _ := json.Marshal(journal)
	if err = createRouterJournal(encoded); err != nil {
		listener.Close()
		return RouterStatus{}, err
	}
	if err = atomicWrite(path, installed); err != nil {
		listener.Close()
		_ = os.Remove(routerJournalPath())
		return RouterStatus{}, err
	}
	proxy := &modelRouter{key: key, token: token, upstream: defaultGatewayURL, models: selectedModels, client: &http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, journal: journal,
		status: RouterStatus{Running: true, Alias: defaultModel, Model: defaultModel, Models: selectedModels, Address: address, BackupPath: routerJournalPath()}}
	proxy.server = &http.Server{Handler: proxy, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second}
	a.router = proxy
	go func() {
		if err := proxy.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			proxy.mu.Lock()
			proxy.status.Running = false
			proxy.status.LastError = "本地路由停止，请恢复配置后重启"
			proxy.mu.Unlock()
		}
	}()
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	return proxy.status, nil
}

func (a *App) startClaudeRouter(r RouterRequest, key, path, home string, listener net.Listener) (RouterStatus, error) {
	original, err := os.ReadFile(path)
	existed := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		listener.Close()
		return RouterStatus{}, err
	}
	root, err := readJSONMap(path)
	if err != nil {
		listener.Close()
		return RouterStatus{}, err
	}
	env := ensureMap(root, "env")
	env["ANTHROPIC_BASE_URL"] = "http://" + listener.Addr().String() + "/v1"
	env["ANTHROPIC_AUTH_TOKEN"] = key
	defaultModel := strings.TrimSpace(r.DefaultModel)
	if defaultModel == "" {
		defaultModel = strings.TrimSpace(r.Model)
	}
	env["ANTHROPIC_MODEL"] = defaultModel
	// Claude Code enables its extended context path through these environment
	// variables. CC Switch uses 1,000,000 tokens for the user-facing 1M mode.
	if r.ClaudeOneM {
		env["CLAUDE_CODE_MAX_CONTEXT_TOKENS"] = "1000000"
		env["CLAUDE_CODE_AUTO_COMPACT_WINDOW"] = "1000000"
	} else {
		delete(env, "CLAUDE_CODE_MAX_CONTEXT_TOKENS")
		delete(env, "CLAUDE_CODE_AUTO_COMPACT_WINDOW")
	}
	// Claude Code exposes these role defaults in its model picker. Mapping all
	// selected models keeps the menu useful while the request body remains the
	// source of truth for the model sent upstream.
	if selected := uniqueModelIDs(r.Models); len(selected) > 0 {
		env["ANTHROPIC_DEFAULT_SONNET_MODEL"] = selected[0]
		env["ANTHROPIC_DEFAULT_OPUS_MODEL"] = selected[minInt(1, len(selected)-1)]
		env["ANTHROPIC_DEFAULT_HAIKU_MODEL"] = selected[minInt(2, len(selected)-1)]
	}
	installed, err := marshalJSON(root)
	if err != nil {
		listener.Close()
		return RouterStatus{}, err
	}
	token, err := createProvisionID()
	if err != nil {
		listener.Close()
		return RouterStatus{}, err
	}
	journal := routerJournal{Path: path, Original: original, Installed: installed, Existed: existed, Address: listener.Addr().String(), Models: uniqueModelIDs(r.Models), DefaultModel: defaultModel}
	encoded, _ := json.Marshal(journal)
	if err = createRouterJournal(encoded); err != nil {
		listener.Close()
		return RouterStatus{}, err
	}
	if err = atomicWrite(path, installed); err != nil {
		listener.Close()
		_ = os.Remove(routerJournalPath())
		return RouterStatus{}, err
	}
	proxy := &modelRouter{key: key, token: token, upstream: defaultGatewayURL, models: uniqueModelIDs(r.Models), client: &http.Client{Timeout: 10 * time.Minute}, journal: journal, status: RouterStatus{Running: true, Alias: defaultModel, Model: defaultModel, Models: uniqueModelIDs(r.Models), Address: "http://" + listener.Addr().String(), BackupPath: routerJournalPath()}}
	proxy.server = &http.Server{Handler: proxy, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second}
	a.router = proxy
	go func() {
		if err := proxy.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			proxy.mu.Lock()
			proxy.status.Running = false
			proxy.status.LastError = "本地路由停止，请恢复配置后重启"
			proxy.mu.Unlock()
		}
	}()
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	return proxy.status, nil
}

// Exclusive creation prevents two assistant processes from taking over Codex.
func createRouterJournal(encoded []byte) error {
	path := routerJournalPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("无法创建路由恢复记录（可能已有助手正在启动路由）：%w", err)
	}
	_, err = f.Write(encoded)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
	}
	return err
}
func restoreRouterJournal(j routerJournal) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if j.Path != filepath.Join(home, ".codex", "config.toml") && filepath.Base(j.Path) != "settings.json" {
		return errors.New("恢复记录的配置路径不匹配")
	}
	current, err := os.ReadFile(j.Path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if bytes.Equal(current, j.Original) {
		return os.Remove(routerJournalPath())
	}
	if !bytes.Equal(current, j.Installed) {
		// The user explicitly asked to stop the router and restore the original
		// Codex configuration. Preserve any intervening edits before restoring.
		conflict := j.Path + ".router-conflict-" + time.Now().Format("20060102-150405")
		if err := atomicWrite(conflict, current); err != nil {
			return fmt.Errorf("无法备份当前配置，未恢复原配置：%w", err)
		}
	}
	if j.Existed {
		restored := j.Original
		// Keep the stable provider ID available for conversations created while
		// routing was active. Its endpoint is the normal gateway after stop.
		if filepath.Base(j.Path) == "config.toml" {
			root := map[string]any{}
			parseErr := toml.Unmarshal(j.Original, &root)
			if parseErr == nil {
				providers := ensureMap(root, "model_providers")
				if _, exists := providers[routerProvider]; !exists {
					providers[routerProvider] = map[string]any{"name": "词元神", "base_url": defaultGatewayURL, "wire_api": "responses", "supports_websockets": false}
					if out, marshalErr := marshalTOML(root); marshalErr == nil {
						restored = out
					}
				}
			}
		}
		err = atomicWrite(j.Path, restored)
	} else {
		err = os.Remove(j.Path)
	}
	if err != nil {
		return err
	}
	if j.CatalogPath != "" {
		if j.CatalogExisted {
			_ = atomicWrite(j.CatalogPath, j.CatalogOriginal)
		} else {
			_ = os.Remove(j.CatalogPath)
		}
	}
	return os.Remove(routerJournalPath())
}
func (a *App) StopModelRouter() (RouterStatus, error) {
	a.routerMu.Lock()
	defer a.routerMu.Unlock()
	a.operation.Lock()
	defer a.operation.Unlock()
	if a.router == nil {
		j, err := readRouterJournal()
		if errors.Is(err, os.ErrNotExist) {
			return RouterStatus{Alias: routerAlias}, nil
		}
		if err != nil {
			return RouterStatus{}, err
		}
		// The original assistant process may have exited without running its
		// shutdown hook. The journal is the recovery authority; restoring it is
		// safe even if the orphaned local listener is still accepting requests,
		// because Codex will immediately stop pointing at that listener.
		return RouterStatus{Alias: routerAlias, BackupPath: routerJournalPath()}, restoreRouterJournal(j)
	}
	proxy := a.router
	if err := restoreRouterJournal(proxy.journal); err != nil {
		return RouterStatus{}, err
	}
	_ = proxy.server.Close()
	a.router = nil
	return RouterStatus{Alias: routerAlias}, nil
}
func (p *modelRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+p.token)) != 1 {
		routeError(w, 401, "本地路由认证失败")
		return
	}
	if r.Method == "GET" && r.URL.Path == "/v1/models" {
		p.mu.Lock()
		advertised := append([]string(nil), p.models...)
		p.mu.Unlock()
		data := make([]any, 0, len(advertised))
		for _, id := range advertised {
			data = append(data, map[string]any{"id": id, "object": "model"})
		}
		writeRouteJSON(w, map[string]any{"object": "list", "data": data})
		return
	}
	if r.Method == "POST" && r.URL.Path == "/v1/messages" {
		p.serveAnthropic(w, r)
		return
	}
	if r.Method != "POST" || r.URL.Path != "/v1/responses" {
		routeError(w, 400, "本地路由支持 Responses 与 Anthropic Messages 对话")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
	var input map[string]any
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		routeError(w, 400, "请求格式错误或超过 16MB")
		return
	}
	p.mu.Lock()
	p.status.Requests++
	model := p.status.Model
	if requested := str(input["model"]); requested != "" {
		// Codex may request a model that is intentionally absent from the
		// catalog. Keep /v1/models limited to mappings while allowing direct
		// requests, matching CC Switch behavior.
		model = requested
	}
	upstreamURL := p.upstream
	p.mu.Unlock()
	request, specs, err := responsesToChat(input, model)
	if err != nil {
		p.fail(err.Error())
		routeError(w, 400, err.Error())
		return
	}
	body, _ := json.Marshal(request)
	var resp *http.Response
	for attempt := 0; attempt < 3; attempt++ {
		upstream, requestErr := http.NewRequestWithContext(r.Context(), "POST", upstreamURL+"/chat/completions", bytes.NewReader(body))
		if requestErr != nil {
			routeError(w, 502, "上游地址无效")
			return
		}
		upstream.Header.Set("Content-Type", "application/json")
		upstream.Header.Set("Authorization", "Bearer "+p.key)
		resp, err = p.client.Do(upstream)
		if err != nil || resp.StatusCode != http.StatusServiceUnavailable {
			break
		}
		resp.Body.Close()
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
		}
	}
	if err != nil {
		if r.Context().Err() == nil {
			p.fail("上游连接失败或超时")
			routeError(w, 502, "上游连接失败或超时")
		}
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		message := fmt.Sprintf("上游返回 HTTP %d，当前分组内支持该模型的供应商暂时不可用；请稍后重试或更换分组", resp.StatusCode)
		p.fail(message)
		routeError(w, resp.StatusCode, message)
		return
	}
	stream, _ := input["stream"].(bool)
	if err = chatToResponses(w, resp.Body, stream, model, specs); err != nil {
		p.fail(err.Error())
	}
}

// serveAnthropic provides the Claude Code compatible Messages endpoint. It
// translates the standard Anthropic request to OpenAI Chat Completions and
// returns an Anthropic-shaped response, allowing Claude Code to use any
// upstream model through the same local router.
func (p *modelRouter) serveAnthropic(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<20))
	var in map[string]any
	if json.Unmarshal(raw, &in) != nil {
		routeError(w, 400, "Anthropic 请求格式错误")
		return
	}
	p.mu.Lock()
	p.status.Requests++
	model, upstream, key := p.status.Model, p.upstream, p.key
	if requested := str(in["model"]); requested != "" {
		model = requested
	}
	p.mu.Unlock()
	msgs := []any{}
	if sys, ok := in["system"].(string); ok && sys != "" {
		msgs = append(msgs, map[string]any{"role": "system", "content": sys})
	}
	if arr, ok := in["messages"].([]any); ok {
		for _, m := range arr {
			if mm, ok := m.(map[string]any); ok {
				msgs = append(msgs, map[string]any{"role": mm["role"], "content": mm["content"]})
			}
		}
	}
	body, _ := json.Marshal(map[string]any{"model": model, "messages": msgs, "stream": false, "max_tokens": in["max_tokens"]})
	var resp *http.Response
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		up, requestErr := http.NewRequestWithContext(r.Context(), "POST", upstream+"/chat/completions", bytes.NewReader(body))
		if requestErr != nil {
			routeError(w, 502, "上游地址无效")
			return
		}
		up.Header.Set("Content-Type", "application/json")
		up.Header.Set("Authorization", "Bearer "+key)
		resp, err = p.client.Do(up)
		if err != nil || resp.StatusCode != http.StatusServiceUnavailable {
			break
		}
		resp.Body.Close()
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
		}
	}
	if err != nil {
		p.fail("上游连接失败")
		routeError(w, 502, "上游连接失败")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		p.fail(fmt.Sprintf("上游返回 HTTP %d", resp.StatusCode))
		routeError(w, resp.StatusCode, "上游模型暂时不可用")
		return
	}
	var out map[string]any
	if json.NewDecoder(resp.Body).Decode(&out) != nil {
		routeError(w, 502, "上游响应格式错误")
		return
	}
	content := ""
	if choices, ok := out["choices"].([]any); ok && len(choices) > 0 {
		if c, ok := choices[0].(map[string]any); ok {
			if m, ok := c["message"].(map[string]any); ok {
				content = str(m["content"])
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": "msg_local_router", "type": "message", "role": "assistant", "model": model, "content": []any{map[string]any{"type": "text", "text": content}}, "stop_reason": "end_turn", "stop_sequence": nil, "usage": map[string]any{"input_tokens": 0, "output_tokens": 0}})
}
func (p *modelRouter) fail(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status.Failures++
	p.status.LastError = message
}
func routeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"type": "router_error", "message": message}})
}
func writeRouteJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func str(v any) string { s, _ := v.(string); return s }

func uniqueModelIDs(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
