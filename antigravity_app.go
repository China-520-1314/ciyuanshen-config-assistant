package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func (a *App) startAntigravityAppProxy(key, model string, models ...string) (string, error) {
	a.antigravityAppMu.Lock()
	defer a.antigravityAppMu.Unlock()
	if a.antigravityAppServer != nil {
		return "", errors.New("Antigravity APP 代理已在运行，请先退出当前 APP 配置会话")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("启动 Antigravity APP 本地代理失败：%w", err)
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		listener.Close()
		return "", err
	}
	a.antigravityAppToken = hex.EncodeToString(token)
	a.antigravityAppKey = key
	a.antigravityAppModel = model
	a.antigravityAppModels = append([]string{model}, models...)
	server := &http.Server{Handler: http.HandlerFunc(a.serveAntigravityApp), ReadHeaderTimeout: 10 * time.Second}
	a.antigravityAppServer = server
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.stopAntigravityAppProxy(context.Background())
		}
	}()
	return "http://" + listener.Addr().String(), nil
}

func (a *App) stopAntigravityAppProxy(ctx context.Context) {
	a.antigravityAppMu.Lock()
	server := a.antigravityAppServer
	a.antigravityAppServer = nil
	a.antigravityAppToken = ""
	a.antigravityAppKey = ""
	a.antigravityAppModel = ""
	a.antigravityAppModels = nil
	a.antigravityAppMu.Unlock()
	if server != nil {
		timeout, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if server.Shutdown(timeout) != nil {
			_ = server.Close()
		}
	}
}

// Native desktop gateway protocol, recovered from the supplied setup script.
func (a *App) serveAntigravityApp(w http.ResponseWriter, r *http.Request) {
	a.antigravityAppMu.Lock()
	key, model, token := a.antigravityAppKey, a.antigravityAppModel, a.antigravityAppToken
	models := append([]string{model}, a.antigravityAppModels...)
	a.antigravityAppMu.Unlock()
	if key == "" || model == "" || token == "" {
		http.Error(w, "Gateway is not configured", 503)
		return
	}
	supplied := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if supplied == "" {
		supplied = r.Header.Get("x-goog-api-key")
	}
	if supplied == "" {
		supplied = r.URL.Query().Get("key")
	}
	if subtle.ConstantTimeCompare([]byte(supplied), []byte(token)) != 1 {
		http.Error(w, "Unauthorized", 401)
		return
	}
	if r.Header.Get("Origin") != "" {
		http.Error(w, "Browser access is not allowed", 403)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", 405)
		return
	}
	const prefix = "/v1beta/models/"
	requested, action, found := strings.Cut(strings.TrimPrefix(r.URL.Path, prefix), ":")
	enabled := false
	for _, candidate := range models {
		if candidate == requested {
			enabled = true
			break
		}
	}
	if !strings.HasPrefix(r.URL.Path, prefix) || !found || !enabled {
		http.Error(w, "Model is not enabled", 404)
		return
	}
	model = requested
	if action != "generateContent" && action != "streamGenerateContent" && action != "countTokens" {
		http.Error(w, "Unsupported endpoint", 404)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32<<20))
	if err != nil || !json.Valid(body) {
		http.Error(w, "Invalid request JSON", 400)
		return
	}
	query := r.URL.Query()
	for name := range query {
		if strings.EqualFold(name, "key") {
			query.Del(name)
		}
	}
	endpoint := "https://api.ciyuanshen.top/v1beta/models/" + url.PathEscape(model) + ":" + action
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		http.Error(w, "Invalid gateway request", 502)
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("x-goog-api-key", key)
	request.Header.Set("Accept-Encoding", "identity")
	response, err := a.antigravityHTTPClient().Do(request)
	if err != nil {
		http.Error(w, "连接词元神 Gemini 网关失败，请检查网络和模型权限", 502)
		return
	}
	defer response.Body.Close()
	for _, name := range []string{"Content-Type", "Retry-After", "X-Request-Id"} {
		if value := response.Header.Get(name); value != "" {
			w.Header().Set(name, value)
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(response.StatusCode)
	buffer := make([]byte, 16384)
	for {
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			if _, err := w.Write(buffer[:n]); err != nil {
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		if readErr != nil {
			return
		}
	}
}

func antigravityGatewayEnvironment(base []string, endpoint, token, model string) []string {
	env := make([]string, 0, len(base)+4)
	for _, entry := range base {
		name := strings.ToUpper(strings.SplitN(entry, "=", 2)[0])
		if !strings.HasPrefix(name, "AGY_LLM_GATEWAY_") && name != "CLOUD_CODE_URL" {
			env = append(env, entry)
		}
	}
	return append(env, "AGY_LLM_GATEWAY_URL="+endpoint, "AGY_LLM_GATEWAY_API_KEY="+token, "AGY_LLM_GATEWAY_WIRE_PROTOCOL=genai", "AGY_LLM_GATEWAY_MODELS="+model)
}

func (a *App) launchAntigravityAppWithProxy(key, model string, models []string) ToolRestartResult {
	result := ToolRestartResult{ClientID: "antigravity-app"}
	definition, _ := clientDefinitionForID("antigravity-app")
	home, err := os.UserHomeDir()
	if err != nil {
		result.ManualRestartRequired = true
		result.Message = "无法确定 Antigravity APP 安装位置"
		return result
	}
	executable := findClientExecutable(definition, home)
	if executable == "" {
		result.ManualRestartRequired = true
		result.Message = "未检测到 Antigravity APP，请先安装桌面版"
		return result
	}
	if running, checkErr := isAntigravityAppRunning(executable); checkErr != nil || running {
		result.ManualRestartRequired = true
		result.Message = "请先完全退出 Antigravity APP，再进行本地代理配置；已有进程不会继承新代理地址"
		return result
	}
	// The process check above ensures an active desktop session is not interrupted.
	a.stopAntigravityAppProxy(context.Background())
	endpoint, err := a.startAntigravityAppProxy(key, model, models...)
	if err != nil {
		result.ManualRestartRequired = true
		result.Message = err.Error()
		return result
	}
	a.antigravityAppMu.Lock()
	token := a.antigravityAppToken
	a.antigravityAppMu.Unlock()
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		bundle := macApplicationBundlePath(executable)
		if bundle == "" {
			a.stopAntigravityAppProxy(context.Background())
			result.ManualRestartRequired = true
			result.Message = "无法确定 Antigravity APP 应用包位置"
			return result
		}
		args := []string{}
		for _, entry := range antigravityGatewayEnvironment(nil, endpoint, token, strings.Join(models, ",")) {
			args = append(args, "--env", entry)
		}
		args = append(args, bundle)
		cmd = exec.Command("/usr/bin/open", args...)
		cmd.Env = antigravityGatewayEnvironment(os.Environ(), endpoint, token, strings.Join(models, ","))
	} else {
		cmd = exec.Command(executable)
		cmd.Env = antigravityGatewayEnvironment(os.Environ(), endpoint, token, strings.Join(models, ","))
	}
	if err = cmd.Start(); err != nil {
		a.stopAntigravityAppProxy(context.Background())
		result.ManualRestartRequired = true
		result.Message = "启动 Antigravity APP 失败：" + err.Error()
		return result
	}
	go func() { _ = cmd.Wait() }()
	result.Attempted = true
	result.Restarted = true
	result.Message = "已发起 Antigravity APP 启动，尚未验证 APP 接入。已启用原生 API Key 网关模式，无需预先 Google 登录；使用期间请保持助手运行。若仍显示登录页，请检查 APP 版本是否支持该模式。"
	return result
}

func isAntigravityAppRunning(executable string) (bool, error) {
	name := applicationProcessName(executable)
	if runtime.GOOS == "darwin" {
		err := exec.Command("pgrep", "-x", name).Run()
		if err == nil {
			return true, nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, errors.New("无法检查 Antigravity APP 运行状态")
	}
	output, err := exec.Command("tasklist", "/FI", "IMAGENAME eq "+name+".exe", "/NH").CombinedOutput()
	if err != nil {
		return false, errors.New("无法检查 Antigravity APP 运行状态")
	}
	return strings.Contains(strings.ToLower(string(output)), strings.ToLower(name+".exe")), nil
}

func (a *App) antigravityHTTPClient() *http.Client {
	return &http.Client{Transport: a.client.Transport, Timeout: 5 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// Keep the chosen model first, while exposing every usable Gemini model.
func antigravityModelMenu(selected string, models []Model) []string {
	menu := []string{}
	seen := map[string]bool{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		if !strings.HasPrefix(id, "gemini") || strings.ContainsAny(id, ",:/\r\n") || seen[id] {
			return
		}
		seen[id] = true
		menu = append(menu, id)
	}
	add(selected)
	for _, model := range models {
		add(model.ID)
	}
	return menu
}
