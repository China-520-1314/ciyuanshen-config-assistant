package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/pelletier/go-toml/v2"
	json5 "github.com/yosuke-furukawa/json5/encoding/json5"
	"gopkg.in/yaml.v3"
)

type configKind string

const (
	configJSON  configKind = "json"
	configJSON5 configKind = "json5"
	configTOML  configKind = "toml"
	configYAML  configKind = "yaml"
	configEnv   configKind = "env"
)

type fileOperation struct {
	ClientID string
	Path     string
	Action   string
	Kind     configKind
	Content  []byte
}

func buildConfiguration(request ConfigurationRequest) ([]fileOperation, []string, error) {
	key := strings.TrimSpace(request.APIKey)
	if key == "" {
		return nil, nil, errors.New("请输入 API Key")
	}
	if len(request.Targets) == 0 {
		return nil, nil, errors.New("至少选择一个要配置的工具")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, errors.New("无法确定当前用户目录")
	}

	models := request.Models
	if models == nil {
		models = map[string]string{}
	}
	seen := map[string]bool{}
	operations := make([]fileOperation, 0, len(request.Targets)+2)
	warnings := make([]string, 0)
	for _, rawTarget := range request.Targets {
		target := strings.ToLower(strings.TrimSpace(rawTarget))
		if target == "" || seen[target] {
			continue
		}
		seen[target] = true
		model := strings.TrimSpace(models[target])
		if model == "" {
			model = strings.TrimSpace(models["default"])
		}
		if target != "codex" && model == "" {
			return nil, nil, fmt.Errorf("请为 %s 选择模型", clientDisplayName(target))
		}

		var targetOperations []fileOperation
		switch target {
		case "claude":
			targetOperations, err = configureClaude(home, key, model)
		case "claude-desktop":
			targetOperations, err = configureClaudeDesktop(home, key, model)
		case "codex":
			targetOperations, err = configureCodex(home, key, model)
		case "gemini":
			targetOperations, err = configureGemini(home, key, model)
		case "grok":
			targetOperations, err = configureGrok(home, key, model)
		case "opencode":
			targetOperations, err = configureOpenCode(home, key, model)
		case "openclaw":
			targetOperations, err = configureOpenClaw(home, key, model)
		case "hermes":
			targetOperations, err = configureHermes(home, key, model)
		default:
			return nil, nil, fmt.Errorf("不支持的工具：%s", rawTarget)
		}
		if err != nil {
			return nil, nil, err
		}
		operations = append(operations, targetOperations...)
		if target == "opencode" || target == "openclaw" || target == "hermes" {
			warnings = append(warnings, fmt.Sprintf("%s 的结构化配置会重新排版，原文件会先备份。", clientDisplayName(target)))
		}
	}
	if len(operations) == 0 {
		return nil, nil, errors.New("没有可写入的配置")
	}
	return operations, uniqueStrings(warnings), nil
}

func clientDisplayName(id string) string {
	for _, definition := range clientDefinitions() {
		if definition.ID == id {
			return definition.Name
		}
	}
	return id
}

func configureClaude(home, key, model string) ([]fileOperation, error) {
	path := firstClientPath("claude", home)
	root, err := readJSONMap(path)
	if err != nil {
		return nil, fmt.Errorf("读取 Claude Code 配置失败：%w", err)
	}
	env := ensureMap(root, "env")
	env["ANTHROPIC_BASE_URL"] = claudeGatewayURL
	env["ANTHROPIC_AUTH_TOKEN"] = key
	delete(env, "ANTHROPIC_API_KEY")
	env["ANTHROPIC_MODEL"] = model
	env["ANTHROPIC_DEFAULT_HAIKU_MODEL"] = model
	env["ANTHROPIC_DEFAULT_SONNET_MODEL"] = model
	env["ANTHROPIC_DEFAULT_OPUS_MODEL"] = model
	content, err := marshalJSON(root)
	if err != nil {
		return nil, fmt.Errorf("生成 Claude Code 配置失败：%w", err)
	}
	return []fileOperation{newOperation("claude", path, configJSON, content)}, nil
}

func configureCodex(home, key, model string) ([]fileOperation, error) {
	configPath := firstClientPath("codex", home)
	existingConfig, err := readTextOrEmpty(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取 Codex 配置失败：%w", err)
	}
	configContent := []byte(patchCodexConfig(existingConfig, model))

	authPath := filepath.Join(home, ".codex", "auth.json")
	authContent, err := marshalJSON(map[string]string{"OPENAI_API_KEY": key})
	if err != nil {
		return nil, fmt.Errorf("生成 Codex 认证文件失败：%w", err)
	}
	return []fileOperation{
		newOperation("codex", configPath, configTOML, configContent),
		newOperation("codex", authPath, configJSON, authContent),
	}, nil
}

const codexDefaultModel = "gpt-5.6-terra"

type codexTemplateField struct {
	Key   string
	Value string
}

var codexTopLevelTemplate = []codexTemplateField{
	{Key: "model_provider", Value: `"ciyuanshen"`},
	{Key: "model", Value: `"gpt-5.6-terra"`},
	{Key: "model_reasoning_effort", Value: `"max"`},
	{Key: "disable_response_storage", Value: "true"},
	{Key: "preferred_auth_method", Value: `"apikey"`},
	{Key: "service_tier", Value: `"fast"`},
	{Key: "web_search", Value: `"live"`},
}

var codexProviderTemplate = []codexTemplateField{
	{Key: "name", Value: `"ciyuanshen"`},
	{Key: "base_url", Value: `"https://api.ciyuanshen.top/v1"`},
	{Key: "wire_api", Value: `"responses"`},
	{Key: "requires_openai_auth", Value: "true"},
}

type codexTableBlock struct {
	Name  string
	Lines []string
}

// patchCodexConfig edits TOML text in place. The provider name is taken from
// the existing top-level model_provider, or inferred from an existing provider
// table when the top-level field is missing. All three provider references are
// then kept in sync without discarding unrelated provider tables.
func patchCodexConfig(existing, selectedModel string) string {
	lines := []string(nil)
	if existing != "" {
		lines = strings.Split(existing, "\n")
	}
	trailingNewline := strings.HasSuffix(existing, "\n")
	firstTable := len(lines)
	for index, line := range lines {
		if codexTableName(line) != "" {
			firstTable = index
			break
		}
	}

	prefix := append([]string(nil), lines[:firstTable]...)
	blocks := codexTableBlocks(lines[firstTable:])
	explicitProviderName := codexTopLevelProviderName(prefix)
	providerName := codexProviderName(prefix, blocks)
	prefix = patchCodexTopLevel(prefix, providerName, selectedModel)
	blocks, providerIndex := patchCodexProviderBlocks(blocks, providerName, explicitProviderName != "")
	if providerIndex < 0 {
		blocks = append(blocks, codexTableBlock{
			Name:  codexProviderTableName(providerName),
			Lines: codexProviderBlockLines(nil, providerName),
		})
	}

	resultLines := append([]string(nil), prefix...)
	for _, block := range blocks {
		if len(resultLines) > 0 && strings.TrimSpace(resultLines[len(resultLines)-1]) != "" {
			resultLines = append(resultLines, "")
		}
		resultLines = append(resultLines, block.Lines...)
	}
	result := strings.Join(resultLines, "\n")
	if trailingNewline || strings.TrimSpace(result) != "" {
		result = strings.TrimRight(result, "\n") + "\n"
	}
	return result
}

func codexTableBlocks(lines []string) []codexTableBlock {
	blocks := make([]codexTableBlock, 0)
	for index := 0; index < len(lines); {
		name := codexTableName(lines[index])
		if name == "" {
			index++
			continue
		}
		end := index + 1
		for end < len(lines) && codexTableName(lines[end]) == "" {
			end++
		}
		blocks = append(blocks, codexTableBlock{Name: name, Lines: append([]string(nil), lines[index:end]...)})
		index = end
	}
	return blocks
}

func codexProviderName(prefix []string, blocks []codexTableBlock) string {
	if name := codexTopLevelProviderName(prefix); name != "" {
		return name
	}
	for _, block := range blocks {
		if !strings.HasPrefix(block.Name, "model_providers.") {
			continue
		}
		shortName := validCodexProviderName(strings.TrimPrefix(block.Name, "model_providers."))
		blockName := validCodexProviderName(codexBlockAssignmentValue(block.Lines, "name"))
		if shortName == "custom" || shortName == managedProviderName {
			if blockName != "" {
				return blockName
			}
			return shortName
		}
		if blockName == shortName {
			return blockName
		}
	}
	for _, block := range blocks {
		if !strings.HasPrefix(block.Name, "model_providers.") {
			continue
		}
		if blockName := validCodexProviderName(codexBlockAssignmentValue(block.Lines, "name")); blockName != "" {
			return blockName
		}
	}
	return managedProviderName
}

func codexTopLevelProviderName(prefix []string) string {
	for _, line := range prefix {
		key, value, ok := codexAssignment(line)
		if key == "model_provider" && ok {
			if name := validCodexProviderName(value); name != "" {
				return name
			}
		}
	}
	return ""
}

func patchCodexTopLevel(lines []string, providerName, selectedModel string) []string {
	if strings.TrimSpace(selectedModel) == "" {
		selectedModel = codexDefaultModel
	}
	values := map[string]string{
		"model_provider":           strconv.Quote(providerName),
		"model":                    strconv.Quote(strings.TrimSpace(selectedModel)),
		"model_reasoning_effort":   `"max"`,
		"disable_response_storage": "true",
		"preferred_auth_method":    `"apikey"`,
		"service_tier":             `"fast"`,
		"web_search":               `"live"`,
	}
	managed := make(map[string]bool, len(values))
	for key := range values {
		managed[key] = true
	}
	patched := make([]string, 0, len(lines)+len(values))
	present := make(map[string]bool, len(values))
	for _, line := range lines {
		key, _, ok := codexAssignment(line)
		if ok && managed[key] {
			if present[key] {
				continue
			}
			present[key] = true
			if key == "model_provider" {
				line = "model_provider = " + values[key]
			}
		}
		patched = append(patched, line)
	}
	missing := make([]string, 0, len(values))
	for _, field := range codexTopLevelTemplate {
		value := values[field.Key]
		if !present[field.Key] {
			missing = append(missing, field.Key+" = "+value)
			present[field.Key] = true
		}
	}
	return insertCodexBeforeTrailingBlankLines(patched, missing)
}

func patchCodexProviderBlocks(blocks []codexTableBlock, providerName string, hasExplicitProvider bool) ([]codexTableBlock, int) {
	targetName := codexProviderTableName(providerName)
	targetIndex := -1
	for index, block := range blocks {
		if block.Name == targetName {
			targetIndex = index
			break
		}
	}
	if targetIndex < 0 {
		candidate := -1
		providerIndices := make([]int, 0)
		for index, block := range blocks {
			if !strings.HasPrefix(block.Name, "model_providers.") {
				continue
			}
			providerIndices = append(providerIndices, index)
			blockName := validCodexProviderName(codexBlockAssignmentValue(block.Lines, "name"))
			if blockName == providerName {
				candidate = index
				break
			}
		}
		if candidate < 0 && len(providerIndices) == 1 {
			onlyBlock := blocks[providerIndices[0]]
			blockName := validCodexProviderName(codexBlockAssignmentValue(onlyBlock.Lines, "name"))
			if hasExplicitProvider || blockName != "" {
				candidate = providerIndices[0]
			}
		}
		if candidate >= 0 {
			targetIndex = candidate
			blocks[targetIndex].Name = targetName
		}
	}
	if targetIndex < 0 {
		return blocks, -1
	}

	merged := append([]string(nil), blocks[targetIndex].Lines...)
	for index, block := range blocks {
		if index == targetIndex || (!codexStaleProviderBlock(block, providerName) && block.Name != targetName) {
			continue
		}
		merged = mergeCodexProviderLines(merged, block.Lines)
	}
	blocks[targetIndex] = codexTableBlock{
		Name:  targetName,
		Lines: codexProviderBlockLines(merged, providerName),
	}
	filtered := make([]codexTableBlock, 0, len(blocks))
	for index, block := range blocks {
		if index != targetIndex && (codexStaleProviderBlock(block, providerName) || block.Name == targetName) {
			continue
		}
		filtered = append(filtered, block)
	}
	newIndex := -1
	for index := range filtered {
		if filtered[index].Name == targetName {
			newIndex = index
			break
		}
	}
	return filtered, newIndex
}

func codexStaleProviderBlock(block codexTableBlock, providerName string) bool {
	if !strings.HasPrefix(block.Name, "model_providers.") {
		return false
	}
	shortName := strings.TrimPrefix(block.Name, "model_providers.")
	if shortName == providerName {
		return false
	}
	if blockName := validCodexProviderName(codexBlockAssignmentValue(block.Lines, "name")); blockName == providerName {
		return true
	}
	// Older releases could leave both the managed provider and custom provider
	// behind. They are only interchangeable when one of those names is the
	// active provider; a custom table must remain untouched for other providers.
	return (providerName == managedProviderName || providerName == "custom") &&
		(shortName == "custom" || shortName == managedProviderName)
}

func mergeCodexProviderLines(target, source []string) []string {
	merged := append([]string(nil), target...)
	for _, line := range source {
		key, _, ok := codexAssignment(line)
		if !ok || codexProviderManagedField(key) || codexHasAssignment(merged, key) {
			continue
		}
		merged = append(merged, line)
	}
	return merged
}

func codexProviderBlockLines(existing []string, providerName string) []string {
	targetName := codexProviderTableName(providerName)
	values := map[string]string{
		"name":                 strconv.Quote(providerName),
		"base_url":             strconv.Quote(defaultGatewayURL),
		"wire_api":             `"responses"`,
		"requires_openai_auth": "true",
	}
	managed := make(map[string]bool, len(values))
	for key := range values {
		managed[key] = true
	}
	lines := existing
	if len(lines) == 0 {
		lines = []string{"[" + targetName + "]"}
	}
	patched := make([]string, 0, len(lines)+len(values))
	present := make(map[string]bool, len(values))
	for index, line := range lines {
		if index == 0 {
			patched = append(patched, "["+targetName+"]")
			continue
		}
		key, _, ok := codexAssignment(line)
		if ok && managed[key] {
			if present[key] {
				continue
			}
			present[key] = true
			switch key {
			case "name":
				line = "name = " + values[key]
			case "base_url":
				if providerName == managedProviderName {
					line = "base_url = " + values[key]
				}
			case "wire_api":
				if providerName == managedProviderName {
					line = "wire_api = " + values[key]
				}
			case "requires_openai_auth":
				if providerName == managedProviderName {
					line = "requires_openai_auth = " + values[key]
				}
			}
		}
		patched = append(patched, line)
	}
	missing := make([]string, 0, len(values))
	for _, field := range codexProviderTemplate {
		value := values[field.Key]
		if !present[field.Key] {
			missing = append(missing, field.Key+" = "+value)
			present[field.Key] = true
		}
	}
	return insertCodexBeforeTrailingBlankLines(patched, missing)
}

func insertCodexBeforeTrailingBlankLines(lines, additions []string) []string {
	if len(additions) == 0 {
		return lines
	}
	insertAt := len(lines)
	for insertAt > 0 && strings.TrimSpace(lines[insertAt-1]) == "" {
		insertAt--
	}
	result := make([]string, 0, len(lines)+len(additions))
	result = append(result, lines[:insertAt]...)
	result = append(result, additions...)
	result = append(result, lines[insertAt:]...)
	return result
}

func codexProviderTableName(providerName string) string {
	return "model_providers." + providerName
}

func validCodexProviderName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, " \t\r\n[]=") {
		return ""
	}
	return value
}

func codexBlockAssignmentValue(lines []string, wanted string) string {
	for _, line := range lines {
		key, value, ok := codexAssignment(line)
		if ok && key == wanted {
			return value
		}
	}
	return ""
}

func codexHasAssignment(lines []string, wanted string) bool {
	for _, line := range lines {
		key, _, ok := codexAssignment(line)
		if ok && key == wanted {
			return true
		}
	}
	return false
}

func codexProviderManagedField(key string) bool {
	switch key {
	case "name", "base_url", "wire_api", "requires_openai_auth":
		return true
	default:
		return false
	}
}

func codexTableName(line string) string {
	trimmed := strings.TrimSpace(codexStripComment(line))
	if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		return ""
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"))
}

func codexAssignment(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(codexStripComment(line))
	if trimmed == "" || strings.HasPrefix(trimmed, "[") {
		return "", "", false
	}
	key, value, ok := strings.Cut(trimmed, "=")
	if !ok {
		return "", "", false
	}
	key = strings.Trim(strings.TrimSpace(key), "\"'")
	if key == "" || strings.ContainsAny(key, " \t") {
		return "", "", false
	}
	value = strings.TrimSpace(value)
	if decoded, err := strconv.Unquote(value); err == nil {
		value = decoded
	} else {
		value = strings.Trim(value, "\"'")
	}
	return key, strings.TrimSpace(value), true
}

func codexAssignmentKey(line string) (string, bool) {
	key, _, ok := codexAssignment(line)
	return key, ok
}

func codexStripComment(line string) string {
	inBasic, inLiteral, escaped := false, false, false
	for index := 0; index < len(line); index++ {
		char := line[index]
		if char == '\\' && inBasic && !escaped {
			escaped = true
			continue
		}
		if char == '"' && !inLiteral && !escaped {
			inBasic = !inBasic
		} else if char == '\'' && !inBasic {
			inLiteral = !inLiteral
		} else if char == '#' && !inBasic && !inLiteral {
			return line[:index]
		}
		escaped = false
	}
	return line
}

func configureGemini(home, key, model string) ([]fileOperation, error) {
	envPath := filepath.Join(home, ".gemini", ".env")
	envContent, err := readTextOrEmpty(envPath)
	if err != nil {
		return nil, fmt.Errorf("读取 Gemini 环境文件失败：%w", err)
	}
	envContent = updateEnvFile(envContent, map[string]string{
		"GEMINI_API_KEY":           key,
		"GEMINI_MODEL":             model,
		"GOOGLE_GEMINI_BASE_URL":   geminiGatewayURL,
		"GOOGLE_GENAI_API_VERSION": geminiAPIVersion,
	})

	settingsPath := filepath.Join(home, ".gemini", "settings.json")
	settings, err := readJSONMap(settingsPath)
	if err != nil {
		return nil, fmt.Errorf("读取 Gemini 设置失败：%w", err)
	}
	security := ensureMap(settings, "security")
	auth := ensureMap(security, "auth")
	auth["selectedType"] = "gemini-api-key"
	settingsContent, err := marshalJSON(settings)
	if err != nil {
		return nil, fmt.Errorf("生成 Gemini 设置失败：%w", err)
	}
	return []fileOperation{
		newOperation("gemini", envPath, configEnv, []byte(envContent)),
		newOperation("gemini", settingsPath, configJSON, settingsContent),
	}, nil
}

func configureGrok(home, key, model string) ([]fileOperation, error) {
	path := firstClientPath("grok", home)
	root, err := readTOMLMap(path)
	if err != nil {
		return nil, fmt.Errorf("读取 Grok Build 配置失败：%w", err)
	}
	models := ensureMap(root, "models")
	models["default"] = managedProviderName
	modelTable := ensureMap(root, "model")
	modelTable[managedProviderName] = map[string]any{
		"model":          model,
		"base_url":       defaultGatewayURL,
		"name":           "词元神",
		"api_key":        key,
		"api_backend":    "responses",
		"context_window": int64(500000),
	}
	content, err := marshalTOML(root)
	if err != nil {
		return nil, fmt.Errorf("生成 Grok Build 配置失败：%w", err)
	}
	return []fileOperation{newOperation("grok", path, configTOML, content)}, nil
}

func configureOpenCode(home, key, model string) ([]fileOperation, error) {
	path := firstClientPath("opencode", home)
	root, err := readJSON5Map(path)
	if err != nil {
		return nil, fmt.Errorf("读取 OpenCode 配置失败：%w", err)
	}
	providers := ensureMap(root, "provider")
	providers[managedProviderName] = map[string]any{
		"npm":  "@ai-sdk/openai-compatible",
		"name": "词元神",
		"options": map[string]any{
			"baseURL": defaultGatewayURL,
			"apiKey":  key,
		},
		"models": map[string]any{
			model: map[string]any{"name": model},
		},
	}
	root["model"] = managedProviderName + "/" + model
	content, err := marshalJSON(root)
	if err != nil {
		return nil, fmt.Errorf("生成 OpenCode 配置失败：%w", err)
	}
	return []fileOperation{newOperation("opencode", path, configJSON5, content)}, nil
}

func configureOpenClaw(home, key, model string) ([]fileOperation, error) {
	path := firstClientPath("openclaw", home)
	root, err := readJSON5Map(path)
	if err != nil {
		return nil, fmt.Errorf("读取 OpenClaw 配置失败：%w", err)
	}
	models := ensureMap(root, "models")
	if _, ok := models["mode"]; !ok {
		models["mode"] = "merge"
	}
	providers := ensureMap(models, "providers")
	providers[managedProviderName] = map[string]any{
		"baseUrl": defaultGatewayURL,
		"apiKey":  key,
		"api":     "openai-completions",
		"models":  []any{map[string]any{"id": model, "name": model}},
	}
	agents := ensureMap(root, "agents")
	defaults := ensureMap(agents, "defaults")
	defaultModel := ensureMap(defaults, "model")
	defaultModel["primary"] = managedProviderName + "/" + model
	content, err := marshalJSON(root)
	if err != nil {
		return nil, fmt.Errorf("生成 OpenClaw 配置失败：%w", err)
	}
	return []fileOperation{newOperation("openclaw", path, configJSON5, content)}, nil
}

func configureHermes(home, key, model string) ([]fileOperation, error) {
	path := firstClientPath("hermes", home)
	root, err := readYAMLMap(path)
	if err != nil {
		return nil, fmt.Errorf("读取 Hermes 配置失败：%w", err)
	}
	providers := ensureYAMLSequence(root, "custom_providers")
	provider := map[string]any{
		"name":     managedProviderName,
		"base_url": defaultGatewayURL,
		"api_key":  key,
		"api_mode": "chat_completions",
		"model":    model,
		"models":   map[string]any{model: map[string]any{}},
	}
	updated := false
	for index, entry := range providers {
		if item, ok := entry.(map[string]any); ok && fmt.Sprint(item["name"]) == managedProviderName {
			providers[index] = provider
			updated = true
			break
		}
	}
	if !updated {
		providers = append(providers, provider)
	}
	root["custom_providers"] = providers
	modelConfig := ensureYAMLMap(root, "model")
	modelConfig["default"] = model
	modelConfig["provider"] = managedProviderName
	content, err := yaml.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("生成 Hermes 配置失败：%w", err)
	}
	return []fileOperation{newOperation("hermes", path, configYAML, content)}, nil
}

func newOperation(clientID, path string, kind configKind, content []byte) fileOperation {
	action := "update"
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		action = "create"
	}
	return fileOperation{ClientID: clientID, Path: path, Action: action, Kind: kind, Content: content}
}

func firstClientPath(id, home string) string {
	for _, definition := range clientDefinitions() {
		if definition.ID != id {
			continue
		}
		paths := definition.Paths(home)
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
		if len(paths) > 0 {
			return paths[0]
		}
	}
	return filepath.Join(home, ".config", id, "config.json")
}

func readJSONMap(path string) (map[string]any, error) {
	content, err := readTextOrEmpty(path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(content) == "" {
		return map[string]any{}, nil
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(content), &root); err != nil {
		return nil, err
	}
	if root == nil {
		return map[string]any{}, nil
	}
	return root, nil
}

func readJSON5Map(path string) (map[string]any, error) {
	content, err := readTextOrEmpty(path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(content) == "" {
		return map[string]any{}, nil
	}
	var root map[string]any
	if err := json5.Unmarshal([]byte(content), &root); err != nil {
		return nil, err
	}
	if root == nil {
		return map[string]any{}, nil
	}
	return root, nil
}

func readTOMLMap(path string) (map[string]any, error) {
	content, err := readTextOrEmpty(path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(content) == "" {
		return map[string]any{}, nil
	}
	root := map[string]any{}
	if err := toml.Unmarshal([]byte(content), &root); err != nil {
		return nil, err
	}
	return root, nil
}

func readYAMLMap(path string) (map[string]any, error) {
	content, err := readTextOrEmpty(path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(content) == "" {
		return map[string]any{}, nil
	}
	root := map[string]any{}
	if err := yaml.Unmarshal([]byte(content), &root); err != nil {
		return nil, err
	}
	return root, nil
}

func readTextOrEmpty(path string) (string, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(content), err
}

func marshalJSON(value any) ([]byte, error) {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func marshalTOML(value any) ([]byte, error) {
	content, err := toml.Marshal(value)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func ensureMap(root map[string]any, key string) map[string]any {
	if existing, ok := root[key].(map[string]any); ok {
		return existing
	}
	value := map[string]any{}
	root[key] = value
	return value
}

func ensureYAMLMap(root map[string]any, key string) map[string]any {
	if existing, ok := root[key].(map[string]any); ok {
		return existing
	}
	value := map[string]any{}
	root[key] = value
	return value
}

func ensureYAMLSequence(root map[string]any, key string) []any {
	if existing, ok := root[key].([]any); ok {
		return existing
	}
	value := []any{}
	root[key] = value
	return value
}

func updateEnvFile(content string, values map[string]string) string {
	newline := "\n"
	if strings.Contains(content, "\r\n") {
		newline = "\r\n"
	}
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	seen := map[string]bool{}
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, _, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if value, exists := values[key]; exists {
			lines[index] = key + "=" + envValue(value)
			seen[key] = true
		}
	}
	for key, value := range values {
		if !seen[key] {
			if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
				lines = append(lines, "")
			}
			lines = append(lines, key+"="+envValue(value))
		}
	}
	return strings.Join(lines, newline)
}

func envValue(value string) string {
	if strings.ContainsAny(value, " \t#\"") {
		return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
	}
	return value
}

func inspectConfig(path string, kind configKind) (string, string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "error", err.Error()
	}
	if len(bytes.TrimSpace(content)) == 0 {
		return "empty", "文件为空，配置时会创建所需字段"
	}
	var value any
	switch kind {
	case configJSON:
		err = json.Unmarshal(content, &value)
	case configJSON5:
		err = json5.Unmarshal(content, &value)
	case configTOML:
		err = toml.Unmarshal(content, &value)
	case configYAML:
		err = yaml.Unmarshal(content, &value)
	case configEnv:
		return "valid", "环境变量文件可读"
	}
	if err != nil {
		return "invalid", "文件格式无法解析"
	}
	return "valid", "配置文件格式正常"
}

func validateWrittenConfig(operation fileOperation) error {
	content, err := os.ReadFile(operation.Path)
	if err != nil {
		return err
	}
	return validateConfigContent(operation.Kind, content)
}

func validateConfigContent(kind configKind, content []byte) error {
	var value any
	switch kind {
	case configJSON:
		return json.Unmarshal(content, &value)
	case configJSON5:
		return json.Unmarshal(content, &value)
	case configTOML:
		return toml.Unmarshal(content, &value)
	case configYAML:
		return yaml.Unmarshal(content, &value)
	case configEnv:
		return nil
	default:
		return errors.New("未知配置格式")
	}
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func isEnvKey(value string) bool {
	for _, character := range value {
		if !(unicode.IsLetter(character) || unicode.IsDigit(character) || character == '_') {
			return false
		}
	}
	return value != ""
}
