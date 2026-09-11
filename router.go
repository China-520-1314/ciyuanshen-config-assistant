package main

// The local Responses -> Chat bridge follows the architecture studied in
// farion1231/cc-switch. This Go implementation is independent of its Rust code.
import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const routerAlias = "gpt-5.6-terra"
const routerProvider = "ciyuanshen-local-router"

type RouterRequest struct {
	APIKey         string `json:"apiKey"`
	Model          string `json:"model"`
	UseExistingKey bool   `json:"useExistingKey"`
}
type RouterStatus struct {
	Running    bool   `json:"running"`
	Model      string `json:"model"`
	Alias      string `json:"alias"`
	Address    string `json:"address"`
	Requests   int    `json:"requests"`
	Failures   int    `json:"failures"`
	LastError  string `json:"lastError"`
	BackupPath string `json:"backupPath"`
}
type routerJournal struct {
	Path      string `json:"path"`
	Original  []byte `json:"original"`
	Installed []byte `json:"installed"`
	Existed   bool   `json:"existed"`
	Address   string `json:"address"`
}
type modelRouter struct {
	mu                   sync.Mutex
	status               RouterStatus
	server               *http.Server
	key, token, upstream string
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
		if _, err := os.Stat(routerJournalPath()); err == nil {
			s.LastError = "发现上次路由的恢复记录，请点击停止并恢复配置后再启动"
		}
		return s
	}
	a.router.mu.Lock()
	defer a.router.mu.Unlock()
	return a.router.status
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
	if !containsModel(models.Models, strings.TrimSpace(r.Model)) {
		return RouterStatus{}, errors.New("所选模型不在这个 Key 的可用模型列表中")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return RouterStatus{}, err
	}
	if custom := os.Getenv("CODEX_HOME"); custom != "" && filepath.Clean(custom) != filepath.Join(home, ".codex") {
		return RouterStatus{}, errors.New("检测到自定义 CODEX_HOME，请先恢复默认目录后使用一键路由")
	}
	path := filepath.Join(home, ".codex", "config.toml")
	original, err := os.ReadFile(path)
	existed := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return RouterStatus{}, err
	}
	root, err := readTOMLMap(path)
	if err != nil {
		return RouterStatus{}, err
	}
	providers := ensureMap(root, "model_providers")
	if _, ok := providers[routerProvider]; ok {
		return RouterStatus{}, errors.New("配置已含本地路由服务商，请先恢复原配置")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return RouterStatus{}, err
	}
	token, err := createProvisionID()
	if err != nil {
		listener.Close()
		return RouterStatus{}, err
	}
	address := "http://" + listener.Addr().String()
	root["model_provider"] = routerProvider
	root["model"] = routerAlias
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
	installed, err := marshalTOML(root)
	if err != nil {
		listener.Close()
		return RouterStatus{}, err
	}
	journal := routerJournal{Path: path, Original: original, Installed: installed, Existed: existed, Address: listener.Addr().String()}
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
	proxy := &modelRouter{key: key, token: token, upstream: defaultGatewayURL, client: &http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, journal: journal,
		status: RouterStatus{Running: true, Alias: routerAlias, Model: strings.TrimSpace(r.Model), Address: address, BackupPath: routerJournalPath()}}
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
	if j.Path != filepath.Join(home, ".codex", "config.toml") {
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
		return errors.New("路由启动后配置被其他程序修改，未覆盖你的修改；原配置保存在 " + routerJournalPath())
	}
	if j.Existed {
		err = atomicWrite(j.Path, j.Original)
	} else {
		err = os.Remove(j.Path)
	}
	if err != nil {
		return err
	}
	return os.Remove(routerJournalPath())
}
func (a *App) StopModelRouter() (RouterStatus, error) {
	a.routerMu.Lock()
	defer a.routerMu.Unlock()
	a.operation.Lock()
	defer a.operation.Unlock()
	if a.router == nil {
		raw, err := os.ReadFile(routerJournalPath())
		if errors.Is(err, os.ErrNotExist) {
			return RouterStatus{Alias: routerAlias}, nil
		}
		if err != nil {
			return RouterStatus{}, err
		}
		var j routerJournal
		if err = json.Unmarshal(raw, &j); err != nil {
			return RouterStatus{}, err
		}
		conn, dialErr := net.DialTimeout("tcp", j.Address, time.Second)
		if dialErr == nil {
			conn.Close()
			return RouterStatus{}, errors.New("另一个助手可能仍在运行路由，请在原窗口停止")
		}
		return RouterStatus{Alias: routerAlias}, restoreRouterJournal(j)
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
		writeRouteJSON(w, map[string]any{"object": "list", "data": []any{map[string]any{"id": routerAlias, "object": "model"}}})
		return
	}
	if r.Method != "POST" || r.URL.Path != "/v1/responses" {
		routeError(w, 400, "本地路由仅支持 Responses 对话；压缩、内置搜索等扩展暂不支持")
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
	upstreamURL := p.upstream
	p.mu.Unlock()
	request, specs, err := responsesToChat(input, model)
	if err != nil {
		p.fail(err.Error())
		routeError(w, 400, err.Error())
		return
	}
	body, _ := json.Marshal(request)
	upstream, err := http.NewRequestWithContext(r.Context(), "POST", upstreamURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		routeError(w, 502, "上游地址无效")
		return
	}
	upstream.Header.Set("Content-Type", "application/json")
	upstream.Header.Set("Authorization", "Bearer "+p.key)
	resp, err := p.client.Do(upstream)
	if err != nil {
		if r.Context().Err() == nil {
			p.fail("上游连接失败或超时")
			routeError(w, 502, "上游连接失败或超时")
		}
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		message := fmt.Sprintf("上游返回 HTTP %d，请检查 Key 分组、余额和模型可用性", resp.StatusCode)
		p.fail(message)
		routeError(w, resp.StatusCode, message)
		return
	}
	stream, _ := input["stream"].(bool)
	if err = chatToResponses(w, resp.Body, stream, model, specs); err != nil {
		p.fail(err.Error())
	}
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
