package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// updateConfiguredClientModel changes only the target client's default model.
// It deliberately bypasses the full configuration transaction: the caller has
// already validated the stored API key and requested model, and this operation
// must not create a backup for a one-field change.
func updateConfiguredClientModel(home, target, model string) error {
	model = strings.TrimSpace(model)
	if model == "" {
		return errors.New("请选择默认模型")
	}

	var operation fileOperation
	var err error
	switch target {
	case "claude":
		path := firstClientPath("claude", home)
		root, readErr := readJSONMap(path)
		if readErr != nil {
			err = readErr
			break
		}
		env, mapErr := requiredMap(root, "env")
		if mapErr != nil {
			err = mapErr
			break
		}
		env["ANTHROPIC_MODEL"] = model
		var content []byte
		content, err = marshalJSON(root)
		operation = newOperation(target, path, configJSON, content)
	case "claude-desktop":
		paths, ok := claudeDesktopPaths(home)
		if !ok {
			err = errors.New("Claude Code客户端仅支持 Windows 和 macOS")
			break
		}
		root, readErr := readJSONMap(paths.Profile)
		if readErr != nil {
			err = readErr
			break
		}
		root["inferenceModels"] = []any{model}
		var content []byte
		content, err = marshalJSON(root)
		operation = newOperation(target, paths.Profile, configJSON, content)
	case "codex":
		path := filepath.Join(home, ".codex", "config.toml")
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			err = readErr
			break
		}
		var patched string
		patched, err = patchCodexModel(string(content), model)
		operation = newOperation(target, path, configTOML, []byte(patched))
	case "gemini":
		path := filepath.Join(home, ".gemini", ".env")
		content, readErr := readTextOrEmpty(path)
		if readErr != nil {
			err = readErr
			break
		}
		operation = newOperation(target, path, configEnv, []byte(updateEnvFile(content, map[string]string{"GEMINI_MODEL": model})))
	case "grok":
		path := firstClientPath("grok", home)
		root, readErr := readTOMLMap(path)
		if readErr != nil {
			err = readErr
			break
		}
		models, mapErr := requiredMap(root, "model")
		if mapErr != nil {
			err = mapErr
			break
		}
		provider, mapErr := requiredMap(models, managedProviderName)
		if mapErr != nil {
			err = mapErr
			break
		}
		provider["model"] = model
		var content []byte
		content, err = marshalTOML(root)
		operation = newOperation(target, path, configTOML, content)
	case "opencode":
		path := firstClientPath("opencode", home)
		root, readErr := readJSON5Map(path)
		if readErr != nil {
			err = readErr
			break
		}
		root["model"] = managedProviderName + "/" + model
		var content []byte
		content, err = marshalJSON(root)
		operation = newOperation(target, path, configJSON5, content)
	case "openclaw":
		path := firstClientPath("openclaw", home)
		root, readErr := readJSON5Map(path)
		if readErr != nil {
			err = readErr
			break
		}
		agents, mapErr := requiredMap(root, "agents")
		if mapErr != nil {
			err = mapErr
			break
		}
		defaults, mapErr := requiredMap(agents, "defaults")
		if mapErr != nil {
			err = mapErr
			break
		}
		defaultModel, mapErr := requiredMap(defaults, "model")
		if mapErr != nil {
			err = mapErr
			break
		}
		defaultModel["primary"] = managedProviderName + "/" + model
		var content []byte
		content, err = marshalJSON(root)
		operation = newOperation(target, path, configJSON5, content)
	case "hermes":
		path := firstClientPath("hermes", home)
		root, readErr := readYAMLMap(path)
		if readErr != nil {
			err = readErr
			break
		}
		modelConfig, mapErr := requiredMap(root, "model")
		if mapErr != nil {
			err = mapErr
			break
		}
		modelConfig["default"] = model
		var content []byte
		content, err = yaml.Marshal(root)
		operation = newOperation(target, path, configYAML, content)
	default:
		err = fmt.Errorf("不支持的工具：%s", target)
	}
	if err != nil {
		return fmt.Errorf("读取 %s 默认模型失败：%w", clientDisplayName(target), err)
	}
	original, readErr := os.ReadFile(operation.Path)
	originalExists := readErr == nil
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return fmt.Errorf("读取 %s 当前配置失败：%w", clientDisplayName(target), readErr)
	}
	restoreOriginal := func() {
		if originalExists {
			_ = atomicWrite(operation.Path, original)
		} else {
			_ = os.Remove(operation.Path)
		}
	}
	if err := atomicWrite(operation.Path, operation.Content); err != nil {
		restoreOriginal()
		return fmt.Errorf("写入 %s 默认模型失败：%w", clientDisplayName(target), err)
	}
	if err := validateWrittenConfig(operation); err != nil {
		restoreOriginal()
		return fmt.Errorf("校验 %s 默认模型失败：%w", clientDisplayName(target), err)
	}
	return nil
}

func patchCodexModel(existing, selectedModel string) (string, error) {
	selectedModel = strings.TrimSpace(selectedModel)
	if selectedModel == "" {
		return existing, errors.New("请选择默认模型")
	}
	lines := strings.Split(existing, "\n")
	firstTable := len(lines)
	for index, line := range lines {
		if codexTableName(line) != "" {
			firstTable = index
			break
		}
	}
	for index := 0; index < firstTable; index++ {
		key, _, ok := codexAssignment(lines[index])
		if !ok || key != "model" {
			continue
		}
		indent := lines[index][:len(lines[index])-len(strings.TrimLeft(lines[index], " \t"))]
		comment := ""
		commentStart := codexStripComment(lines[index])
		if len(commentStart) < len(lines[index]) {
			comment = strings.TrimSpace(lines[index][len(commentStart):])
		}
		lines[index] = indent + "model = " + strconv.Quote(selectedModel)
		if comment != "" {
			lines[index] += " " + comment
		}
		return strings.Join(lines, "\n"), nil
	}
	return existing, errors.New("配置文件缺少 model")
}
