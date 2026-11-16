package main

import (
	"bufio"
	"context"
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

const antigravityAppModelID = "MODEL_PLACEHOLDER_M512"

func (a *App) startAntigravityAppProxy(key, model string) (string, error) {
	a.antigravityAppMu.Lock()
	defer a.antigravityAppMu.Unlock()
	if a.antigravityAppServer != nil {
		return "", errors.New("Antigravity APP 代理已在运行，请先退出当前 APP 配置会话")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("启动 Antigravity APP 本地代理失败：%w", err)
	}
	a.antigravityAppKey = key
	a.antigravityAppModel = model
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
	a.antigravityAppKey = ""
	a.antigravityAppModel = ""
	a.antigravityAppMu.Unlock()
	if server != nil {
		timeout, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if server.Shutdown(timeout) != nil {
			_ = server.Close()
		}
	}
}

func (a *App) serveAntigravityApp(w http.ResponseWriter, r *http.Request) {
	a.antigravityAppMu.Lock()
	key, model := a.antigravityAppKey, a.antigravityAppModel
	a.antigravityAppMu.Unlock()
	if key == "" || model == "" {
		http.Error(w, "Antigravity APP proxy is not configured", http.StatusServiceUnavailable)
		return
	}
	if r.Header.Get("Origin") != "" {
		http.Error(w, "Browser access is not allowed", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	path := strings.Trim(r.URL.Path, "/")
	if strings.Contains(path, "fetchAvailableModels") || strings.Contains(path, "GetAvailableModels") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"models": map[string]any{
			"custom-ciyuanshen": map[string]any{"displayName": model, "model": antigravityAppModelID, "planModel": antigravityAppModelID, "requestedModel": antigravityAppModelID, "apiProvider": "API_PROVIDER_GOOGLE_GEMINI", "modelProvider": "MODEL_PROVIDER_GOOGLE", "recommended": false, "maxTokens": 32768, "maxOutputTokens": 8192, "supportsImages": false, "supportsTools": false, "supportsThinking": false},
		}})
		return
	}
	if !strings.Contains(path, "generateContent") && !strings.Contains(path, "streamGenerateContent") {
		a.forwardAntigravityControl(w, r)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<20))
	if err != nil {
		http.Error(w, "Invalid Antigravity request", http.StatusBadRequest)
		return
	}
	requestBody, err := antigravityAppRequestBody(body)
	if err != nil {
		http.Error(w, "Invalid Antigravity request JSON", http.StatusBadRequest)
		return
	}
	stream := strings.Contains(path, "streamGenerateContent")
	endpoint := antigravityAppGeminiEndpoint(model, key, stream)
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, strings.NewReader(string(requestBody)))
	if err != nil {
		http.Error(w, "Could not create Gemini request", http.StatusBadGateway)
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := a.antigravityHTTPClient().Do(request)
	if err != nil {
		http.Error(w, "连接词元神 Gemini 网关失败，请检查网络和模型权限", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, io.LimitReader(response.Body, 1<<20))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		if flusher, ok := w.(http.Flusher); ok {
			reader := bufio.NewReader(response.Body)
			for {
				line, readErr := reader.ReadString('\n')
				if strings.HasPrefix(line, "data:") {
					data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
					if data != "" && data != "[DONE]" {
						_, _ = fmt.Fprintf(w, "data: {\"response\":%s,\"traceId\":\"\",\"metadata\":{}}\n\n", data)
						flusher.Flush()
					}
				}
				if readErr != nil {
					return
				}
			}
		}
		return
	}
	var result json.RawMessage
	if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
		http.Error(w, "Gemini 网关返回了无效响应", http.StatusBadGateway)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"response": result, "traceId": "", "metadata": map[string]any{}})
}

func antigravityAppRequestBody(body []byte) (json.RawMessage, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if len(payload["request"]) > 0 {
		return payload["request"], nil
	}
	return json.RawMessage(body), nil
}

func antigravityAppGeminiEndpoint(model, key string, stream bool) string {
	method := "generateContent"
	query := "?key=" + url.QueryEscape(key)
	if stream {
		method = "streamGenerateContent"
		query = "?alt=sse&key=" + url.QueryEscape(key)
	}
	return "https://api.ciyuanshen.top/v1/models/" + url.PathEscape(model) + ":" + method + query
}

func (a *App) launchAntigravityAppWithProxy(key, model string) ToolRestartResult {
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
	endpoint, err := a.startAntigravityAppProxy(key, model)
	if err != nil {
		result.ManualRestartRequired = true
		result.Message = err.Error()
		return result
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		bundle := macApplicationBundlePath(executable)
		if bundle == "" {
			a.stopAntigravityAppProxy(context.Background())
			result.ManualRestartRequired = true
			result.Message = "无法确定 Antigravity APP 应用包位置"
			return result
		}
		cmd = exec.Command("/usr/bin/open", "--env", "CLOUD_CODE_URL="+endpoint, bundle)
	} else {
		cmd = exec.Command(executable)
		cmd.Env = append(os.Environ(), "CLOUD_CODE_URL="+endpoint)
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
	result.Message = "已发起 Antigravity APP 启动，尚未验证 APP 接入。需先完成 Google 登录，词元神 Key 不能替代登录；使用期间请保持助手运行。"
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

// Keep Google sign-in/control credentials on Google's endpoint, never the model gateway.
func (a *App) antigravityHTTPClient() *http.Client {
	return &http.Client{Transport: a.client.Transport, Timeout: 2 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func (a *App) forwardAntigravityControl(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/v1internal:") || strings.ContainsAny(path, "?#") {
		http.Error(w, "Unsupported Cloud Code route", 404)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "https://daily-cloudcode-pa.googleapis.com"+path, http.MaxBytesReader(w, r.Body, 8<<20))
	if err != nil {
		http.Error(w, "Invalid control request", 400)
		return
	}
	for _, name := range []string{"Authorization", "Content-Type", "X-Goog-Api-Client", "User-Agent"} {
		req.Header.Set(name, r.Header.Get(name))
	}
	response, err := a.antigravityHTTPClient().Do(req)
	if err != nil {
		http.Error(w, "无法连接 Google 官方服务，请检查登录与网络", 502)
		return
	}
	defer response.Body.Close()
	w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(response.Body, 8<<20))
}
