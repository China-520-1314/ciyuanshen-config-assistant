package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ConnectionCheckRequest describes the installed clients that should be
// validated against the CiyuanShen gateway.
type ConnectionCheckRequest struct {
	Targets []string `json:"targets"`
}

type ClientConnectionResult struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Success    bool      `json:"success"`
	Configured bool      `json:"configured"`
	Status     int       `json:"status"`
	Endpoint   string    `json:"endpoint"`
	Message    string    `json:"message"`
	CheckedAt  time.Time `json:"checkedAt"`
}

type ConnectionCheckReport struct {
	Results   []ClientConnectionResult `json:"results"`
	CheckedAt time.Time                `json:"checkedAt"`
}

func (a *App) CheckClientConnections(request ConnectionCheckRequest) (ConnectionCheckReport, error) {
	return checkClientConnections(a.client, request)
}

func checkClientConnections(client *http.Client, request ConnectionCheckRequest) (ConnectionCheckReport, error) {
	targets, err := normaliseConnectionTargets(request.Targets)
	if err != nil {
		return ConnectionCheckReport{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ConnectionCheckReport{}, errors.New("无法确定当前用户目录")
	}

	checkedAt := time.Now()
	report := ConnectionCheckReport{
		Results:   make([]ClientConnectionResult, 0, len(targets)),
		CheckedAt: checkedAt,
	}
	for _, target := range targets {
		result := ClientConnectionResult{
			ID:        target,
			Name:      clientDisplayName(target),
			Endpoint:  defaultGatewayURL + "/models",
			CheckedAt: checkedAt,
		}
		key, keyErr := readConfiguredClientAPIKey(home, target)
		if keyErr != nil {
			result.Message = "配置文件检查失败：" + keyErr.Error()
			report.Results = append(report.Results, result)
			continue
		}
		if configErr := verifyManagedClientConfiguration(home, target, key); configErr != nil {
			result.Message = "配置文件检查失败：" + configErr.Error()
			report.Results = append(report.Results, result)
			continue
		}
		result.Configured = true
		status, gatewayErr := probeCiyuanShenGateway(client, key)
		result.Status = status
		if gatewayErr != nil {
			result.Message = "网关连接失败：" + gatewayErr.Error()
			report.Results = append(report.Results, result)
			continue
		}
		result.Success = true
		result.Message = "配置文件与词元神网关均可用"
		report.Results = append(report.Results, result)
	}
	return report, nil
}

// readConfiguredClientAPIKey reads the credential already stored by each
// client. Connection checks intentionally use this value instead of requiring
// the user to paste a duplicate key into the assistant.
func readConfiguredClientAPIKey(home, target string) (string, error) {
	switch target {
	case "claude":
		path := firstClientPath("claude", home)
		if err := requireConfigFile(path); err != nil {
			return "", err
		}
		root, err := readJSONMap(path)
		if err != nil {
			return "", errors.New("配置文件格式无效")
		}
		env, err := requiredMap(root, "env")
		if err != nil {
			return "", err
		}
		return requiredNonEmptyString(env, "ANTHROPIC_AUTH_TOKEN")
	case "codex":
		path := filepath.Join(home, ".codex", "auth.json")
		if err := requireConfigFile(path); err != nil {
			return "", err
		}
		root, err := readJSONMap(path)
		if err != nil {
			return "", errors.New("auth.json 格式无效")
		}
		return requiredNonEmptyString(root, "OPENAI_API_KEY")
	case "gemini":
		path := filepath.Join(home, ".gemini", ".env")
		if err := requireConfigFile(path); err != nil {
			return "", err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		key, ok := envFileValue(string(content), "GEMINI_API_KEY")
		if !ok || strings.TrimSpace(key) == "" {
			return "", errors.New("GEMINI_API_KEY 未正确配置")
		}
		return strings.TrimSpace(key), nil
	case "grok":
		path := firstClientPath("grok", home)
		if err := requireConfigFile(path); err != nil {
			return "", err
		}
		root, err := readTOMLMap(path)
		if err != nil {
			return "", errors.New("配置文件格式无效")
		}
		model, err := requiredMap(root, "model")
		if err != nil {
			return "", err
		}
		provider, err := requiredMap(model, managedProviderName)
		if err != nil {
			return "", err
		}
		return requiredNonEmptyString(provider, "api_key")
	case "opencode":
		path := firstClientPath("opencode", home)
		if err := requireConfigFile(path); err != nil {
			return "", err
		}
		root, err := readJSON5Map(path)
		if err != nil {
			return "", errors.New("配置文件格式无效")
		}
		providers, err := requiredMap(root, "provider")
		if err != nil {
			return "", err
		}
		provider, err := requiredMap(providers, managedProviderName)
		if err != nil {
			return "", err
		}
		options, err := requiredMap(provider, "options")
		if err != nil {
			return "", err
		}
		return requiredNonEmptyString(options, "apiKey")
	case "openclaw":
		path := firstClientPath("openclaw", home)
		if err := requireConfigFile(path); err != nil {
			return "", err
		}
		root, err := readJSON5Map(path)
		if err != nil {
			return "", errors.New("配置文件格式无效")
		}
		models, err := requiredMap(root, "models")
		if err != nil {
			return "", err
		}
		providers, err := requiredMap(models, "providers")
		if err != nil {
			return "", err
		}
		provider, err := requiredMap(providers, managedProviderName)
		if err != nil {
			return "", err
		}
		return requiredNonEmptyString(provider, "apiKey")
	case "hermes":
		path := firstClientPath("hermes", home)
		if err := requireConfigFile(path); err != nil {
			return "", err
		}
		root, err := readYAMLMap(path)
		if err != nil {
			return "", errors.New("配置文件格式无效")
		}
		providers, ok := root["custom_providers"].([]any)
		if !ok {
			return "", errors.New("缺少 custom_providers")
		}
		for _, item := range providers {
			provider, ok := item.(map[string]any)
			if !ok || fmt.Sprint(provider["name"]) != managedProviderName {
				continue
			}
			return requiredNonEmptyString(provider, "api_key")
		}
		return "", errors.New("缺少 ciyuanshen 服务商配置")
	default:
		return "", fmt.Errorf("不支持的工具：%s", target)
	}
}

func normaliseConnectionTargets(values []string) ([]string, error) {
	known := map[string]bool{}
	for _, definition := range clientDefinitions() {
		known[definition.ID] = true
	}
	seen := map[string]bool{}
	targets := make([]string, 0, len(values))
	for _, value := range values {
		id := strings.ToLower(strings.TrimSpace(value))
		if id == "" || seen[id] {
			continue
		}
		if !known[id] {
			return nil, fmt.Errorf("不支持的工具：%s", value)
		}
		seen[id] = true
		targets = append(targets, id)
	}
	if len(targets) == 0 {
		return nil, errors.New("至少选择一个要检测的工具")
	}
	return targets, nil
}

func probeCiyuanShenGateway(client *http.Client, key string) (int, error) {
	if client == nil {
		client = http.DefaultClient
	}
	request, err := http.NewRequest(http.MethodGet, defaultGatewayURL+"/models", nil)
	if err != nil {
		return 0, errors.New("无法创建网关检测请求")
	}
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "ciyuanshen-config-assistant")

	response, err := client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	content, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if readErr != nil {
		return response.StatusCode, errors.New("读取网关检测响应失败")
	}
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		payload, decodeErr := decodeModelPayload(bytes.NewReader(content))
		if decodeErr != nil {
			return response.StatusCode, errors.New("网关未返回可识别的模型列表")
		}
		if len(payload.Models) == 0 {
			return response.StatusCode, errors.New("API Key 可用，但网关没有返回可用模型")
		}
		return response.StatusCode, nil
	}
	if response.StatusCode == http.StatusUnauthorized {
		return response.StatusCode, errors.New("API Key 无效或已过期")
	}
	if message := responseMessage(content); message != "" {
		return response.StatusCode, fmt.Errorf("HTTP %d：%s", response.StatusCode, message)
	}
	return response.StatusCode, fmt.Errorf("HTTP %d", response.StatusCode)
}

func responseMessage(content []byte) string {
	var payload struct {
		Message string `json:"message"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(content, &payload) == nil {
		if message := strings.TrimSpace(payload.Message); message != "" {
			return message
		}
		if message := strings.TrimSpace(payload.Error.Message); message != "" {
			return message
		}
	}
	return ""
}

func verifyManagedClientConfiguration(home, target, key string) error {
	switch target {
	case "claude":
		return verifyClaudeConfiguration(home, key)
	case "codex":
		return verifyCodexConfiguration(home, key)
	case "gemini":
		return verifyGeminiConfiguration(home, key)
	case "grok":
		return verifyGrokConfiguration(home, key)
	case "opencode":
		return verifyOpenCodeConfiguration(home, key)
	case "openclaw":
		return verifyOpenClawConfiguration(home, key)
	case "hermes":
		return verifyHermesConfiguration(home, key)
	default:
		return fmt.Errorf("不支持的工具：%s", target)
	}
}

func verifyClaudeConfiguration(home, key string) error {
	path := firstClientPath("claude", home)
	if err := requireConfigFile(path); err != nil {
		return err
	}
	root, err := readJSONMap(path)
	if err != nil {
		return errors.New("配置文件格式无效")
	}
	env, err := requiredMap(root, "env")
	if err != nil {
		return err
	}
	if err := requiredString(env, "ANTHROPIC_BASE_URL", claudeGatewayURL); err != nil {
		return err
	}
	return requiredString(env, "ANTHROPIC_AUTH_TOKEN", key)
}

func verifyCodexConfiguration(home, key string) error {
	configPath := filepath.Join(home, ".codex", "config.toml")
	if err := requireConfigFile(configPath); err != nil {
		return err
	}
	config, err := readTOMLMap(configPath)
	if err != nil {
		return errors.New("config.toml 格式无效")
	}
	for field, expected := range map[string]string{
		"model_provider":        managedProviderName,
		"review_model":          "gpt-5.6-sol",
		"preferred_auth_method": "apikey",
		"service_tier":          "fast",
		"web_search":            "live",
	} {
		if err := requiredString(config, field, expected); err != nil {
			return err
		}
	}
	if err := requiredBool(config, "disable_response_storage", true); err != nil {
		return err
	}
	providers, err := requiredMap(config, "model_providers")
	if err != nil {
		return err
	}
	provider, err := requiredMap(providers, managedProviderName)
	if err != nil {
		return err
	}
	for field, expected := range map[string]string{
		"name":     managedProviderName,
		"base_url": defaultGatewayURL,
		"wire_api": "responses",
	} {
		if err := requiredString(provider, field, expected); err != nil {
			return err
		}
	}

	authPath := filepath.Join(home, ".codex", "auth.json")
	if err := requireConfigFile(authPath); err != nil {
		return err
	}
	auth, err := readJSONMap(authPath)
	if err != nil {
		return errors.New("auth.json 格式无效")
	}
	return requiredString(auth, "OPENAI_API_KEY", key)
}

func verifyGeminiConfiguration(home, key string) error {
	envPath := filepath.Join(home, ".gemini", ".env")
	if err := requireConfigFile(envPath); err != nil {
		return err
	}
	content, err := os.ReadFile(envPath)
	if err != nil {
		return err
	}
	for field, expected := range map[string]string{
		"GEMINI_API_KEY":           key,
		"GOOGLE_GEMINI_BASE_URL":   geminiGatewayURL,
		"GOOGLE_GENAI_API_VERSION": geminiAPIVersion,
	} {
		value, ok := envFileValue(string(content), field)
		if !ok || value != expected {
			return fmt.Errorf("%s 未正确配置", field)
		}
	}

	settingsPath := filepath.Join(home, ".gemini", "settings.json")
	if err := requireConfigFile(settingsPath); err != nil {
		return err
	}
	settings, err := readJSONMap(settingsPath)
	if err != nil {
		return errors.New("settings.json 格式无效")
	}
	security, err := requiredMap(settings, "security")
	if err != nil {
		return err
	}
	auth, err := requiredMap(security, "auth")
	if err != nil {
		return err
	}
	return requiredString(auth, "selectedType", "gemini-api-key")
}

func verifyGrokConfiguration(home, key string) error {
	path := firstClientPath("grok", home)
	if err := requireConfigFile(path); err != nil {
		return err
	}
	root, err := readTOMLMap(path)
	if err != nil {
		return errors.New("配置文件格式无效")
	}
	models, err := requiredMap(root, "models")
	if err != nil {
		return err
	}
	if err := requiredString(models, "default", managedProviderName); err != nil {
		return err
	}
	model, err := requiredMap(root, "model")
	if err != nil {
		return err
	}
	provider, err := requiredMap(model, managedProviderName)
	if err != nil {
		return err
	}
	if err := requiredString(provider, "base_url", defaultGatewayURL); err != nil {
		return err
	}
	return requiredString(provider, "api_key", key)
}

func verifyOpenCodeConfiguration(home, key string) error {
	path := firstClientPath("opencode", home)
	if err := requireConfigFile(path); err != nil {
		return err
	}
	root, err := readJSON5Map(path)
	if err != nil {
		return errors.New("配置文件格式无效")
	}
	providers, err := requiredMap(root, "provider")
	if err != nil {
		return err
	}
	provider, err := requiredMap(providers, managedProviderName)
	if err != nil {
		return err
	}
	options, err := requiredMap(provider, "options")
	if err != nil {
		return err
	}
	if err := requiredString(options, "baseURL", defaultGatewayURL); err != nil {
		return err
	}
	return requiredString(options, "apiKey", key)
}

func verifyOpenClawConfiguration(home, key string) error {
	path := firstClientPath("openclaw", home)
	if err := requireConfigFile(path); err != nil {
		return err
	}
	root, err := readJSON5Map(path)
	if err != nil {
		return errors.New("配置文件格式无效")
	}
	models, err := requiredMap(root, "models")
	if err != nil {
		return err
	}
	providers, err := requiredMap(models, "providers")
	if err != nil {
		return err
	}
	provider, err := requiredMap(providers, managedProviderName)
	if err != nil {
		return err
	}
	if err := requiredString(provider, "baseUrl", defaultGatewayURL); err != nil {
		return err
	}
	return requiredString(provider, "apiKey", key)
}

func verifyHermesConfiguration(home, key string) error {
	path := firstClientPath("hermes", home)
	if err := requireConfigFile(path); err != nil {
		return err
	}
	root, err := readYAMLMap(path)
	if err != nil {
		return errors.New("配置文件格式无效")
	}
	providers, ok := root["custom_providers"].([]any)
	if !ok {
		return errors.New("缺少 custom_providers")
	}
	for _, item := range providers {
		provider, ok := item.(map[string]any)
		if !ok || fmt.Sprint(provider["name"]) != managedProviderName {
			continue
		}
		if err := requiredString(provider, "base_url", defaultGatewayURL); err != nil {
			return err
		}
		return requiredString(provider, "api_key", key)
	}
	return errors.New("缺少 ciyuanshen 服务商配置")
}

func requireConfigFile(path string) error {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("未找到配置文件")
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("配置路径不是文件")
	}
	return nil
}

func requiredMap(root map[string]any, key string) (map[string]any, error) {
	value, ok := root[key].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("缺少 %s 配置", key)
	}
	return value, nil
}

func requiredString(root map[string]any, key, expected string) error {
	value, ok := root[key]
	if !ok || fmt.Sprint(value) != expected {
		return fmt.Errorf("%s 未正确配置", key)
	}
	return nil
}

func requiredNonEmptyString(root map[string]any, key string) (string, error) {
	value, ok := root[key].(string)
	value = strings.TrimSpace(value)
	if !ok || value == "" {
		return "", fmt.Errorf("%s 未正确配置", key)
	}
	return value, nil
}

func requiredBool(root map[string]any, key string, expected bool) error {
	value, ok := root[key].(bool)
	if !ok || value != expected {
		return fmt.Errorf("%s 未正确配置", key)
	}
	return nil
}

func envFileValue(content, expectedKey string) (string, bool) {
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != expectedKey {
			continue
		}
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "\"") {
			if unquoted, err := strconv.Unquote(value); err == nil {
				return unquoted, true
			}
		}
		return value, true
	}
	return "", false
}
