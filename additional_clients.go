package main

// Native schemas and path rules checked against CC-Switch e217bfc
// (https://github.com/farion1231/cc-switch, MIT). Independent Go implementation.
import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const dshKeyReference = "CIYUANSHEN_DSH_API_KEY"

func validOMPProfile(name string) bool {
	if len(name) > 64 || name == "." || name == ".." || strings.HasSuffix(name, ".") {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-') {
			return false
		}
	}
	return name != ""
}

func dshRoot(home string) string {
	return filepath.Dir(filepath.Dir(filepath.Dir(deepSeekHarnessPaths(home)[0])))
}
func dshProfilePaths(home string) []string {
	paths := deepSeekHarnessPaths(home)
	for _, name := range []string{"headless", "acp"} {
		p := filepath.Join(dshRoot(home), "profiles", name, "cordis.patch.yml")
		if _, err := os.Stat(p); err == nil {
			paths = append(paths, p)
		}
	}
	seen := map[string]bool{}
	result := []string{}
	for _, p := range paths {
		identity := p
		if parent, err := filepath.EvalSymlinks(filepath.Dir(p)); err == nil {
			identity = filepath.Join(parent, filepath.Base(p))
		}
		if !seen[identity] {
			seen[identity] = true
			result = append(result, p)
		}
	}
	return result
}

// YAML nodes keep tags, anchors and comments on unrelated native plugins.
func readDSHDocument(path string) (*yaml.Node, error) {
	text, err := readTextOrEmpty(path)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if strings.TrimSpace(text) != "" {
		if err = yaml.Unmarshal([]byte(text), &doc); err != nil {
			return nil, err
		}
	}
	if len(doc.Content) == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.SequenceNode, Tag: "!!seq"}}}
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.SequenceNode {
		return nil, errors.New("DeepSeek Harness profile 必须是 YAML 列表")
	}
	return &doc, nil
}
func yamlMapValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
func yamlSetValue(node *yaml.Node, key string, value *yaml.Node) error {
	if node == nil || node.Kind != yaml.MappingNode {
		return errors.New("配置节点必须是映射，拒绝覆盖原有内容")
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = value
			return nil
		}
	}
	node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
	return nil
}
func yamlEnsureMapping(node *yaml.Node, key string) (*yaml.Node, error) {
	value := yamlMapValue(node, key)
	if value == nil || value.Tag == "!!null" {
		value = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		if err := yamlSetValue(node, key, value); err != nil {
			return nil, err
		}
	}
	if value.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s 不是映射，拒绝覆盖", key)
	}
	return value, nil
}
func dshPlugin(doc *yaml.Node, create bool) (*yaml.Node, error) {
	var found *yaml.Node
	for _, entry := range doc.Content[0].Content {
		id := yamlMapValue(entry, "id")
		if id != nil && id.Value == "llm-pi-ai" {
			if found != nil {
				return nil, errors.New("DeepSeek Harness 存在重复 llm-pi-ai 插件")
			}
			found = entry
		}
	}
	if found == nil && create {
		found = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		var id yaml.Node
		_ = id.Encode("llm-pi-ai")
		_ = yamlSetValue(found, "id", &id)
		doc.Content[0].Content = append(doc.Content[0].Content, found)
	}
	return found, nil
}
func dshProvider(home string) (map[string]any, error) {
	for _, path := range dshProfilePaths(home) {
		doc, err := readDSHDocument(path)
		if err != nil {
			return nil, err
		}
		plugin, err := dshPlugin(doc, false)
		if err != nil {
			return nil, err
		}
		provider := yamlMapValue(yamlMapValue(yamlMapValue(plugin, "config"), "providers"), managedProviderName)
		if provider != nil {
			var result map[string]any
			if err = provider.Decode(&result); err != nil {
				return nil, err
			}
			return result, nil
		}
	}
	return nil, errors.New("未找到 DeepSeek Harness 词元神服务商")
}
func configureDeepSeekHarness(home, key, model string) ([]fileOperation, error) {
	homePatch := filepath.Join(dshRoot(home), "cordis.patch.yml")
	if text, err := readTextOrEmpty(homePatch); err != nil {
		return nil, err
	} else if strings.TrimSpace(text) != "" {
		doc, err := readDSHDocument(homePatch)
		if err != nil {
			return nil, err
		}
		plugin, err := dshPlugin(doc, false)
		if err != nil {
			return nil, err
		}
		if yamlMapValue(plugin, "config") != nil {
			return nil, fmt.Errorf("home 级 llm-pi-ai 补丁会覆盖 profile，请先在 %s 中处理", homePatch)
		}
	}
	operations := []fileOperation{}
	for _, path := range dshProfilePaths(home) {
		if _, err := os.Stat(path + ".lock"); err == nil {
			return nil, errors.New("DeepSeek Harness 正在写入配置，请关闭工具后重试")
		}
		doc, err := readDSHDocument(path)
		if err != nil {
			return nil, err
		}
		plugin, err := dshPlugin(doc, true)
		if err != nil {
			return nil, err
		}
		config, err := yamlEnsureMapping(plugin, "config")
		if err != nil {
			return nil, err
		}
		providers, err := yamlEnsureMapping(config, "providers")
		if err != nil {
			return nil, err
		}
		var provider yaml.Node
		if err = provider.Encode(map[string]any{"displayName": "词元神", "api": "openai-completions", "baseURL": defaultGatewayURL, "apiKeyEnv": dshKeyReference, "models": []any{map[string]any{"id": model}}}); err != nil {
			return nil, err
		}
		if err = yamlSetValue(providers, managedProviderName, &provider); err != nil {
			return nil, err
		}
		content, err := yaml.Marshal(doc)
		if err != nil {
			return nil, err
		}
		operations = append(operations, newOperation("deepseek-harness", path, configYAML, content))
	}
	credentialPath := filepath.Join(dshRoot(home), ".credentials.yaml")
	if _, err := os.Stat(credentialPath + ".lock"); err == nil {
		return nil, errors.New("DeepSeek Harness 凭据正在更新，请关闭工具后重试")
	}
	credentials, err := readYAMLMap(credentialPath)
	if err != nil {
		return nil, err
	}
	if value, ok := credentials["refs"]; ok && value != nil {
		if _, ok = value.(map[string]any); !ok {
			return nil, errors.New("DeepSeek Harness refs 格式无效")
		}
	}
	if _, ok := credentials["version"]; !ok {
		credentials["version"] = 1
	}
	ensureYAMLMap(credentials, "refs")[dshKeyReference] = key
	content, err := yaml.Marshal(credentials)
	if err != nil {
		return nil, err
	}
	operations = append(operations, newOperation("deepseek-harness", credentialPath, configYAML, content))
	return operations, nil
}

func antigravityEnvPath(home string) string {
	return filepath.Join(filepath.Dir(filepath.Dir(antigravityPaths(home)[0])), ".env")
}
func antigravityLauncherPath(home string) string {
	ext := ".sh"
	if runtime.GOOS == "windows" {
		ext = ".ps1"
	}
	return filepath.Join(filepath.Dir(filepath.Dir(antigravityPaths(home)[0])), "launch-ciyuanshen"+ext)
}
func configureAntigravity(home, key, model string) ([]fileOperation, error) {
	path := antigravityPaths(home)[0]
	root, err := readJSONMap(path)
	if err != nil {
		return nil, err
	}
	names, err := antigravityRuntimeModels(home, key)
	if err != nil {
		return nil, err
	}
	displayName, ok := names[model]
	if !ok {
		return nil, errors.New("Antigravity 当前版本不支持所选模型，请更新工具或选择原生支持的 Gemini 模型")
	}
	root["modelProvider"] = "gemini"
	root["model"] = displayName
	content, err := marshalJSON(root)
	if err != nil {
		return nil, err
	}
	// agy only accepts process environment. Use a private --gemini_dir and an
	// explicit launcher; never change native official-login settings or shell rc.
	dir := filepath.Dir(filepath.Dir(path))
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	launcher := "#!/bin/sh\nexport GEMINI_API_KEY=" + quote(key) + "\nexport GOOGLE_GEMINI_BASE_URL=" + quote(geminiGatewayURL) + "\nexec agy --gemini_dir=" + quote(dir) + " \"$@\"\n"
	if runtime.GOOS == "windows" {
		launcher = "$env:GEMINI_API_KEY = " + powershellQuote(key) + "\r\n$env:GOOGLE_GEMINI_BASE_URL = " + powershellQuote(geminiGatewayURL) + "\r\n& agy " + powershellQuote("--gemini_dir="+dir) + " @args\r\n"
	}
	env := "GEMINI_MODEL=" + strconv.Quote(model) + "\nGEMINI_API_KEY=" + strconv.Quote(key) + "\nGOOGLE_GEMINI_BASE_URL=" + strconv.Quote(geminiGatewayURL) + "\n"
	return []fileOperation{newOperation("antigravity", path, configJSON, content), newOperation("antigravity", antigravityEnvPath(home), configEnv, []byte(env)), newOperation("antigravity", antigravityLauncherPath(home), configScript, []byte(launcher))}, nil
}

func firstProviderModel(provider map[string]any) (string, error) {
	models, ok := provider["models"].([]any)
	if !ok || len(models) == 0 {
		return "", errors.New("服务商未配置模型")
	}
	model, ok := models[0].(map[string]any)
	if !ok {
		return "", errors.New("服务商模型格式无效")
	}
	return requiredNonEmptyString(model, "id")
}
func additionalProvider(home, target string) (map[string]any, error) {
	var root map[string]any
	var err error
	section := "providers"
	switch target {
	case "deepseek-harness":
		return dshProvider(home)
	case "pi":
		root, err = readJSON5Map(piPaths(home)[0])
	case "mcode":
		root, err = readYAMLMap(firstClientPath(target, home))
		section = "custom_provider"
	case "kimi":
		root, err = readTOMLMap(firstClientPath(target, home))
	case "omp":
		paths := ompPaths(home)
		if len(paths) == 0 {
			return nil, errors.New("Oh My Pi profile 名称无效")
		}
		root, err = readYAMLMap(paths[0])
	default:
		return nil, errors.New("未知客户端")
	}
	if err != nil {
		return nil, err
	}
	providers, err := requiredMap(root, section)
	if err != nil {
		return nil, err
	}
	return requiredMap(providers, managedProviderName)
}
func verifyAdditionalClientConfiguration(home, target, key string) error {
	stored, err := readConfiguredClientAPIKey(home, target)
	if err != nil {
		return err
	}
	if stored != key {
		return errors.New("API Key 未正确配置")
	}
	model, err := readConfiguredClientDefaultModel(home, target)
	if err != nil {
		return err
	}
	if model == "" {
		return errors.New("未配置模型")
	}
	if target == "antigravity" {
		root, err := readJSONMap(antigravityPaths(home)[0])
		if err != nil {
			return err
		}
		if err = requiredString(root, "modelProvider", "gemini"); err != nil {
			return err
		}
		content, err := os.ReadFile(antigravityEnvPath(home))
		if err != nil {
			return err
		}
		if value, ok := envFileValue(string(content), "GOOGLE_GEMINI_BASE_URL"); !ok || value != geminiGatewayURL {
			return errors.New("Antigravity 网关地址不正确")
		}
		return requireConfigFile(antigravityLauncherPath(home))
	}
	provider, err := additionalProvider(home, target)
	if err != nil {
		return err
	}
	field := "baseUrl"
	if target == "deepseek-harness" {
		field = "baseURL"
	}
	if target == "kimi" {
		field = "base_url"
	}
	if target == "mcode" {
		provider, err = requiredMap(provider, "options")
		if err != nil {
			return err
		}
		field = "baseURL"
	}
	if err = requiredString(provider, field, defaultGatewayURL); err != nil {
		return err
	}
	return nil
}

// Never reinterpret dynamic YAML while adding a provider through map encoders.
func validateAdditionalConfigurationPaths(home, target string) error {
	if target == "omp" && len(ompPaths(home)) == 0 {
		return errors.New("Oh My Pi profile 名称无效")
	}
	var paths []string
	switch target {
	case "mcode":
		paths = mcodePaths(home)
	case "omp":
		paths = ompPaths(home)
	default:
		return nil
	}
	for _, path := range paths {
		if _, err := os.Stat(path + ".lock"); err == nil {
			return fmt.Errorf("配置正被客户端写入，请关闭工具后重试：%s", path)
		}
		text, err := readTextOrEmpty(path)
		if err != nil {
			return err
		}
		if text == "" {
			continue
		}
		var doc yaml.Node
		if err = yaml.Unmarshal([]byte(text), &doc); err != nil {
			return err
		}
		var check func(*yaml.Node) error
		check = func(n *yaml.Node) error {
			if n.Kind == yaml.AliasNode || n.Anchor != "" || n.Tag == "!!merge" || strings.HasPrefix(n.Tag, "!") && !strings.HasPrefix(n.Tag, "!!") {
				return fmt.Errorf("配置含动态 YAML 或锚点，请在客户端中编辑：%s", path)
			}
			for _, child := range n.Content {
				if err := check(child); err != nil {
					return err
				}
			}
			return nil
		}
		if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
			return fmt.Errorf("配置必须是 YAML 映射：%s", path)
		}
		for _, field := range []string{"providers", "custom_provider", "modelRoles"} {
			value := yamlMapValue(doc.Content[0], field)
			if value != nil && value.Tag != "!!null" && value.Kind != yaml.MappingNode {
				return fmt.Errorf("%s 字段必须是映射：%s", field, path)
			}
		}
		if err = check(&doc); err != nil {
			return err
		}
	}
	return nil
}

func parseAntigravityModels(output string) map[string]string {
	result := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		id, name, ok := strings.Cut(strings.TrimSpace(line), "\t")
		name = strings.TrimSpace(name)
		if !ok || id == "" || name == "" || strings.ContainsAny(name, "\r\n\t") {
			continue
		}
		valid := true
		for _, c := range id {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-_.:/", c)) {
				valid = false
				break
			}
		}
		if valid {
			if _, ok = result[id]; !ok {
				result[id] = name
			}
		}
	}
	return result
}
func antigravityRuntimeModels(home, key string) (map[string]string, error) {
	definition, _ := clientDefinitionForID("antigravity")
	bin := findClientExecutable(definition, home)
	if bin == "" {
		return nil, errors.New("请先安装 Antigravity CLI，再读取其原生模型列表进行配置")
	}
	dir, err := os.MkdirTemp("", "ciyuanshen-agy-models-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	settings := filepath.Join(dir, "antigravity-cli", "settings.json")
	if err = atomicWrite(settings, []byte(`{"modelProvider":"gemini"}`)); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--gemini_dir="+dir, "models")
	if runtime.GOOS == "windows" && strings.EqualFold(filepath.Ext(bin), ".cmd") {
		cmd = newWindowsShellCommand(ctx, "call "+quoteWindowsBatchPath(bin)+" "+quoteWindowsBatchPath("--gemini_dir="+dir)+" models")
	}
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GEMINI_API_KEY="+key, "GOOGLE_GEMINI_BASE_URL="+geminiGatewayURL)
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.New("无法读取 Antigravity 原生模型列表，请检查工具版本与网络后重试")
	}
	models := parseAntigravityModels(string(output))
	if len(models) == 0 {
		return nil, errors.New("Antigravity 未返回可识别的原生模型列表")
	}
	return models, nil
}

func literalPiKey(key string) string {
	value := strings.ReplaceAll(key, "$", "$$")
	if strings.HasPrefix(value, "!") {
		value = "$!" + value[1:]
	}
	return value
}
