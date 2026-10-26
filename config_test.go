package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-config"))
	t.Setenv("APPDATA", filepath.Join(home, "appdata"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "localappdata"))
	t.Setenv("HERMES_HOME", "")
	return home
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func operationFor(t *testing.T, operations []fileOperation, path string) fileOperation {
	t.Helper()
	for _, operation := range operations {
		if operation.Path == path {
			return operation
		}
	}
	t.Fatalf("operation for %s not found", path)
	return fileOperation{}
}

func assertGeminiCLIModelCompatibility(t *testing.T, settings map[string]any, models ...string) {
	t.Helper()
	experimental, ok := settings["experimental"].(map[string]any)
	if !ok || experimental["dynamicModelConfiguration"] != true {
		t.Fatalf("dynamic Gemini model configuration was not enabled: %#v", settings["experimental"])
	}
	modelConfigs, ok := settings["modelConfigs"].(map[string]any)
	if !ok {
		t.Fatalf("modelConfigs was not written: %#v", settings["modelConfigs"])
	}
	aliases, ok := modelConfigs["customAliases"].(map[string]any)
	if !ok {
		t.Fatalf("customAliases was not written: %#v", modelConfigs["customAliases"])
	}
	definitions, ok := modelConfigs["modelDefinitions"].(map[string]any)
	if !ok {
		t.Fatalf("modelDefinitions was not written: %#v", modelConfigs["modelDefinitions"])
	}
	for _, model := range models {
		alias, ok := aliases[model].(map[string]any)
		if !ok || alias["extends"] != "chat-base-3" {
			t.Fatalf("model alias for %s is invalid: %#v", model, aliases[model])
		}
		modelConfig, ok := alias["modelConfig"].(map[string]any)
		if !ok || modelConfig["model"] != model {
			t.Fatalf("model config for %s is invalid: %#v", model, alias["modelConfig"])
		}
		definition, ok := definitions[model].(map[string]any)
		if !ok || definition["isVisible"] != true || definition["tier"] != "flash" {
			t.Fatalf("model definition for %s is invalid: %#v", model, definitions[model])
		}
	}
}

func TestConfigureClaudePreservesExistingSettings(t *testing.T) {
	home := isolateHome(t)
	path := filepath.Join(home, ".claude", "settings.json")
	writeFixture(t, path, `{"permissions":{"allow":["Bash"]},"env":{"OLD_VALUE":"keep"}}`)

	operations, err := configureClaude(home, "test-key", "gpt-test")
	if err != nil {
		t.Fatal(err)
	}
	operation := operationFor(t, operations, path)
	if operation.Action != "update" {
		t.Fatalf("expected update action, got %q", operation.Action)
	}
	var root map[string]any
	if err := json.Unmarshal(operation.Content, &root); err != nil {
		t.Fatal(err)
	}
	env, ok := root["env"].(map[string]any)
	if !ok {
		t.Fatalf("env was not preserved as an object: %#v", root["env"])
	}
	if env["ANTHROPIC_BASE_URL"] != claudeGatewayURL || env["ANTHROPIC_AUTH_TOKEN"] != "test-key" {
		t.Fatalf("unexpected Claude gateway values: %#v", env)
	}
	if env["ANTHROPIC_MODEL"] != "gpt-test" || env["OLD_VALUE"] != "keep" {
		t.Fatalf("existing/model values were not preserved: %#v", env)
	}
	if env["CLAUDE_CODE_SUBAGENT_MODEL"] != "gpt-test" {
		t.Fatalf("subagent model was not configured: %#v", env)
	}
	if _, exists := env["ANTHROPIC_API_KEY"]; exists {
		t.Fatal("legacy ANTHROPIC_API_KEY should be removed")
	}
	onboarding := operationFor(t, operations, filepath.Join(home, ".claude.json"))
	var onboardingRoot map[string]any
	if err := json.Unmarshal(onboarding.Content, &onboardingRoot); err != nil {
		t.Fatal(err)
	}
	if onboardingRoot["hasCompletedOnboarding"] != true {
		t.Fatalf("Claude onboarding flag was not written: %#v", onboardingRoot)
	}
}

func TestConfigureGeminiUpdatesEnvWithoutDroppingComments(t *testing.T) {
	home := isolateHome(t)
	envPath := filepath.Join(home, ".gemini", ".env")
	settingsPath := filepath.Join(home, ".gemini", "settings.json")
	writeFixture(t, envPath, "# keep this comment\nGEMINI_API_KEY=old\nOTHER=value\n")
	writeFixture(t, settingsPath, `{"mcpServers":{"local":{"command":"demo"}},"modelConfigs":{"customAliases":{"keep":{"modelConfig":{"model":"keep"}}}}}`)

	operations, err := configureGemini(home, "key with space", "gemini-3.8-flash")
	if err != nil {
		t.Fatal(err)
	}
	envOperation := operationFor(t, operations, envPath)
	envContent := string(envOperation.Content)
	for _, expected := range []string{
		"# keep this comment",
		`GEMINI_API_KEY="key with space"`,
		"GEMINI_MODEL=gemini-3.8-flash",
		"GOOGLE_GEMINI_BASE_URL=" + geminiGatewayURL,
		"GOOGLE_GENAI_API_VERSION=" + geminiAPIVersion,
		"OTHER=value",
	} {
		if !strings.Contains(envContent, expected) {
			t.Fatalf("updated .env is missing %q:\n%s", expected, envContent)
		}
	}
	if strings.Count(envContent, "GEMINI_API_KEY=") != 1 {
		t.Fatalf("GEMINI_API_KEY was duplicated:\n%s", envContent)
	}

	settingsOperation := operationFor(t, operations, settingsPath)
	var settings map[string]any
	if err := json.Unmarshal(settingsOperation.Content, &settings); err != nil {
		t.Fatal(err)
	}
	security := settings["security"].(map[string]any)
	auth := security["auth"].(map[string]any)
	if auth["selectedType"] != "gemini-api-key" {
		t.Fatalf("unexpected Gemini auth selection: %#v", auth)
	}
	if _, ok := settings["mcpServers"]; !ok {
		t.Fatal("existing Gemini settings were dropped")
	}
	assertGeminiCLIModelCompatibility(t, settings, "gemini-3.7-flash", "gemini-3.8-flash")
	modelConfigs := settings["modelConfigs"].(map[string]any)
	aliases := modelConfigs["customAliases"].(map[string]any)
	if _, ok := aliases["keep"]; !ok {
		t.Fatal("existing Gemini model aliases were dropped")
	}
}

func TestBuildConfigurationRejectsInvalidRequests(t *testing.T) {
	isolateHome(t)
	if _, _, err := buildConfiguration(ConfigurationRequest{}); err == nil {
		t.Fatal("expected missing API key error")
	}
	if _, _, err := buildConfiguration(ConfigurationRequest{APIKey: "key"}); err == nil {
		t.Fatal("expected missing target error")
	}
	if _, _, err := buildConfiguration(ConfigurationRequest{
		APIKey:  "key",
		Targets: []string{"unsupported"},
		Models:  map[string]string{"default": "model"},
	}); err == nil || !strings.Contains(err.Error(), "不支持") {
		t.Fatalf("expected unsupported target error, got %v", err)
	}
}

func TestValidateWrittenConfigReadsTheWrittenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeFixture(t, path, "not-json")
	err := validateWrittenConfig(fileOperation{
		Path:    path,
		Kind:    configJSON,
		Content: []byte(`{"this":"would be valid in memory"}`),
	})
	if err == nil {
		t.Fatal("expected validation to fail for the file on disk")
	}
}

func TestConfigureCodexAddsDefaultsAndPreservesExistingConfig(t *testing.T) {
	home := isolateHome(t)
	configPath := filepath.Join(home, ".codex", "config.toml")
	authPath := filepath.Join(home, ".codex", "auth.json")
	writeFixture(t, configPath, "model = \"old-model\"\ncustom_top_level = \"keep\"\n[model_providers.old]\nbase_url = \"https://old.example\"\ncustom = true\n")
	writeFixture(t, authPath, `{"OPENAI_API_KEY":"old-key","tokens":{"access_token":"keep-out"}}`)

	operations, err := configureCodex(home, "new-key", "ignored-model")
	if err != nil {
		t.Fatal(err)
	}
	configOperation := operationFor(t, operations, configPath)
	configContent := string(configOperation.Content)
	for _, expected := range []string{
		`model = "old-model"`,
		`custom_top_level = "keep"`,
		`model_provider = "ciyuanshen"`,
		`model_reasoning_effort = "medium"`,
		`disable_response_storage = true`,
		`preferred_auth_method = "apikey"`,
		`web_search = "live"`,
		`[model_providers.old]`,
		`base_url = "https://old.example"`,
		`custom = true`,
		`[model_providers.ciyuanshen]`,
		`name = "ciyuanshen"`,
		`base_url = "https://api.ciyuanshen.top/v1"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
	} {
		if !strings.Contains(configContent, expected) {
			t.Fatalf("Codex config is missing %q:\n%s", expected, configContent)
		}
	}

	authOperation := operationFor(t, operations, authPath)
	var auth map[string]string
	if err := json.Unmarshal(authOperation.Content, &auth); err != nil {
		t.Fatal(err)
	}
	if len(auth) != 1 || auth["OPENAI_API_KEY"] != "new-key" {
		t.Fatalf("auth.json was not updated: %#v", auth)
	}
}

func TestCodexRemovesRetiredOptionsAndPreservesOtherSettings(t *testing.T) {
	const base = `model_provider = "ciyuanshen"
model = "gpt-6.1-sol"
model_reasoning_effort = "high"
web_search = "live"
notify = ["notify.exe", "turn-ended"]

`
	const provider = `[model_providers.ciyuanshen]
name = "ciyuanshen"
base_url = "https://api.ciyuanshen.top/v1"
wire_api = "responses"
requires_openai_auth = true
custom = "keep"

[desktop]
localeOverride = "zh-CN"
custom_desktop = true

[other]
enabled = true
experimental_mode = true
use_history_notes_extension = true
`
	for _, tc := range []struct {
		name, retired string
		preserved     []string
	}{
		{"no options", "", nil},
		{"screenshot", "context_management = { experimental_mode = true }\ntoken_budget.enabled = true\ntoken_budget.use_history_notes_extension = true\n", nil},
		{"dotted false", "context_management.experimental_mode = false\ntoken_budget.enabled = false\ntoken_budget.use_history_notes_extension = false\n", nil},
		{"empty tables", "[context_management]\nexperimental_mode = true\n\n[token_budget]\nenabled = true\nuse_history_notes_extension = true\n", nil},
		{"custom tables", "[context_management]\nexperimental_mode = true\nexperimental_mode = true\ncustom_context = 'keep'\n\n[token_budget]\nenabled = true\nenabled = true\nuse_history_notes_extension = true\ncustom_budget = [1, 2]\n", []string{"custom_context = 'keep'", "custom_budget = [1, 2]"}},
		{"inline with comments", "context_management = { experimental_mode = true, custom_context = \"keep # string\" } # keep comment\ntoken_budget = { enabled = false, use_history_notes_extension = true, custom_budget = { nested = [1, 2] } }\n", []string{`custom_context = "keep # string"`, "# keep comment", "custom_budget = { nested = [1, 2] }"}},
		{"duplicate inline", "context_management = { experimental_mode = true, experimental_mode = true, custom_context = 'keep' }\ntoken_budget = { enabled = true, enabled = true, use_history_notes_extension = true }\n", []string{"custom_context = 'keep'"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := isolateHome(t)
			path := filepath.Join(home, ".codex", "config.toml")
			writeFixture(t, path, base+tc.retired+"\n"+provider)
			// Old frontend requests must not re-enable the retired options.
			operations, err := configureCodexWithSettings(home, "fixture-key", "ignored-model", CodexExperimentalSettings{
				ContextManagementExperimentalMode: true, TokenBudgetEnabled: true, TokenBudgetUseHistoryNotesExtension: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			patched := string(operationFor(t, operations, path).Content)
			var root map[string]any
			if err := toml.Unmarshal([]byte(patched), &root); err != nil {
				t.Fatalf("invalid TOML: %v\n%s", err, patched)
			}
			for table, fields := range map[string][]string{
				"context_management": {"experimental_mode"},
				"token_budget":       {"enabled", "use_history_notes_extension"},
			} {
				if values, ok := root[table].(map[string]any); ok {
					for _, field := range fields {
						if _, exists := values[field]; exists {
							t.Fatalf("retired field %s.%s remains", table, field)
						}
					}
				}
			}
			for _, expected := range append(strings.Split(strings.TrimSpace(base), "\n"), tc.preserved...) {
				if !strings.Contains(patched, expected) {
					t.Fatalf("lost other setting %q:\n%s", expected, patched)
				}
			}
			for _, expected := range strings.Split(strings.TrimSpace(provider), "\n") {
				if !strings.Contains(patched, expected) {
					t.Fatalf("provider or unrelated setting %q changed:\n%s", expected, patched)
				}
			}
			if again := patchCodexConfig(patched, "ignored-model"); again != patched {
				t.Fatalf("cleanup is not idempotent:\n%s", again)
			}
		})
	}
}

func TestCodexFastTierRequiresExplicitOptIn(t *testing.T) {
	for _, existing := range []string{"", "service_tier = \"fast\"\n", "service_tier = \"flex\"\nservice_tier = \"fast\"\n"} {
		for _, enabled := range []bool{false, true} {
			settings := CodexExperimentalSettings{FastMode: enabled}
			patched := patchCodexConfigWithSettings(existing, "", settings)
			var root map[string]any
			if err := toml.Unmarshal([]byte(patched), &root); err != nil {
				t.Fatal(err)
			}
			value, exists := root["service_tier"]
			if exists != enabled || (enabled && value != "fast") {
				t.Fatalf("Fast opt-in %v produced tier %v", enabled, value)
			}
			desktop := root["desktop"].(map[string]any)
			if desktop["fast_mode"] != true || desktop["conversationDetailMode"] != "STEPS_COMMANDS" {
				t.Fatalf("unexpected desktop settings: %#v", desktop)
			}
			if again := patchCodexConfigWithSettings(patched, "", settings); again != patched {
				t.Fatal("Fast configuration is not idempotent")
			}
			if strings.Contains(patchCodexConfig(patched, ""), "service_tier") {
				t.Fatal("unchecked configuration must clear a previous Fast tier")
			}
		}
	}
}

func TestCodexServiceTierSelection(t *testing.T) {
	for _, tc := range []struct {
		name     string
		settings CodexExperimentalSettings
		want     string
	}{
		{"standard", CodexExperimentalSettings{ServiceTier: "default"}, ""},
		{"fast", CodexExperimentalSettings{ServiceTier: "fast"}, "fast"},
		{"ultrafast", CodexExperimentalSettings{ServiceTier: "ultrafast"}, "ultrafast"},
		{"legacy fast", CodexExperimentalSettings{FastMode: true}, "fast"},
		{"explicit standard overrides legacy", CodexExperimentalSettings{ServiceTier: "default", FastMode: true}, ""},
		{"explicit ultrafast overrides legacy", CodexExperimentalSettings{ServiceTier: "ultrafast", FastMode: true}, "ultrafast"},
		{"invalid tier", CodexExperimentalSettings{ServiceTier: "invalid"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, existingTier := range []string{"fast", "ultrafast"} {
				home := isolateHome(t)
				path := filepath.Join(home, ".codex", "config.toml")
				writeFixture(t, path, "model = \"user-model\"\nservice_tier = "+strconv.Quote(existingTier)+"\n")
				operations, err := configureCodexWithSettings(home, "fixture-key", "", tc.settings)
				if err != nil {
					t.Fatal(err)
				}
				patched := string(operationFor(t, operations, path).Content)
				var root map[string]any
				if err := toml.Unmarshal([]byte(patched), &root); err != nil {
					t.Fatal(err)
				}
				value, exists := root["service_tier"]
				if exists != (tc.want != "") || (exists && value != tc.want) {
					t.Fatalf("tier = %v, want %q", value, tc.want)
				}
				if root["model"] != "user-model" {
					t.Fatal("existing model changed")
				}
				if again := patchCodexConfigWithSettings(patched, "", tc.settings); again != patched {
					t.Fatal("service tier is not idempotent")
				}
			}
		})
	}
}

func TestConfigureCodexUsesNewDefaultModelForEmptyConfig(t *testing.T) {
	home := isolateHome(t)
	operations, err := configureCodex(home, "new-key", "")
	if err != nil {
		t.Fatal(err)
	}
	content := string(operationFor(t, operations, filepath.Join(home, ".codex", "config.toml")).Content)
	if !strings.Contains(content, `model = "gpt-6.1-sol"`) || !strings.Contains(content, `model_reasoning_effort = "medium"`) {
		t.Fatalf("empty Codex config did not receive the new default model:\n%s", content)
	}
	if strings.Contains(content, "review_model") {
		t.Fatal("legacy review_model should not be added to a new Codex config")
	}
}

func TestPatchCodexConfigPreservesExistingManagedValuesAndOtherTables(t *testing.T) {
	existing := `# keep this comment
model_provider = "user-provider"
model = "user-model"
disable_response_storage = false

[model_providers.ciyuanshen]
name = "user-name"
custom_provider_value = "keep"

[other]
model = "other-model"
custom = true
`
	patched := patchCodexConfig(existing, "ignored-model")
	for _, expected := range []string{
		`model_provider = "user-provider"`,
		`model = "user-model"`,
		`disable_response_storage = false`,
		`model_reasoning_effort = "medium"`,
		`preferred_auth_method = "apikey"`,
		`web_search = "live"`,
		`[model_providers.user-provider]`,
		`name = "user-provider"`,
		`custom_provider_value = "keep"`,
		`base_url = "https://api.ciyuanshen.top/v1"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
		`[other]`,
		`model = "other-model"`,
		`custom = true`,
	} {
		if !strings.Contains(patched, expected) {
			t.Fatalf("patched Codex config is missing %q:\n%s", expected, patched)
		}
	}
	if strings.Count(patched, `model_provider =`) != 1 || strings.Count(patched, `model = "user-model"`) != 1 {
		t.Fatalf("existing top-level assignments were duplicated:\n%s", patched)
	}
}

func TestPatchCodexConfigAddsDesktopDefaultsBeforeProviderTables(t *testing.T) {
	existing := `model = "user-model"

[model_providers.example]
name = "example"
base_url = "https://example.test/v1"
`
	patched := patchCodexConfig(existing, "")
	for _, expected := range []string{
		`[desktop]`,
		`followUpQueueMode = "queue"`,
		`enabled-reasoning-efforts = [ "low", "medium", "high", "xhigh", "ultra", "max" ]`,
		`localeOverride = "zh-CN"`,
		`conversationDetailMode = "STEPS_COMMANDS"`,
		`fast_mode = true`,
	} {
		if !strings.Contains(patched, expected) {
			t.Fatalf("Codex desktop configuration is missing %q:\n%s", expected, patched)
		}
	}
	if strings.Index(patched, "[desktop]") > strings.Index(patched, "[model_providers.") {
		t.Fatalf("desktop configuration should be placed before provider tables:\n%s", patched)
	}
	var decoded map[string]any
	if err := toml.Unmarshal([]byte(patched), &decoded); err != nil {
		t.Fatalf("patched Codex configuration is not valid TOML: %v\n%s", err, patched)
	}
}

func TestPatchCodexConfigEnforcesDesktopPreferencesAndPreservesOtherValues(t *testing.T) {
	existing := `model_provider = "ciyuanshen"

[desktop]
followUpQueueMode = "immediate"
enabled-reasoning-efforts = [
  "low",
]
localeOverride = "en-US"
conversationDetailMode = "COMPACT"
custom_desktop_option = true

[desktop.custom_file_handlers.editor]
label = "Editor"

[model_providers.ciyuanshen]
name = "ciyuanshen"
`
	patched := patchCodexConfig(existing, "")
	for _, expected := range []string{
		`followUpQueueMode = "immediate"`,
		`enabled-reasoning-efforts = [ "low", "medium", "high", "xhigh", "ultra", "max" ]`,
		`conversationDetailMode = "STEPS_COMMANDS"`,
		`custom_desktop_option = true`,
		`[desktop.custom_file_handlers.editor]`,
		`label = "Editor"`,
		`localeOverride = "zh-CN"`,
	} {
		if !strings.Contains(patched, expected) {
			t.Fatalf("patched desktop configuration is missing %q:\n%s", expected, patched)
		}
	}
	if strings.Count(patched, "followUpQueueMode =") != 1 || strings.Count(patched, "enabled-reasoning-efforts =") != 1 {
		t.Fatalf("existing desktop value was duplicated:\n%s", patched)
	}
	if again := patchCodexConfig(patched, ""); again != patched {
		t.Fatal("desktop configuration is not idempotent")
	}
	var decoded map[string]any
	if err := toml.Unmarshal([]byte(patched), &decoded); err != nil {
		t.Fatalf("patched Codex configuration is not valid TOML: %v\n%s", err, patched)
	}
}

func TestPatchCodexConfigUnifiesDuplicateCustomProvider(t *testing.T) {
	existing := `model_provider = "ciyuanshen"
model = "gpt-5.5"
model_reasoning_effort = "medium"
disable_response_storage = true
preferred_auth_method = "apikey"
service_tier = "fast"
web_search = "live"

[model_providers.custom]
name = "custom"
wire_api = "responses"
requires_openai_auth = true

[model_providers.ciyuanshen]
name = "ciyuanshen"
base_url = "[https://ciyuanshen.top/v1](https://ciyuanshen.top/v1)"
wire_api = "responses"
requires_openai_auth = true
`
	patched := patchCodexConfig(existing, "ignored-model")
	if strings.Count(patched, "[model_providers.") != 1 {
		t.Fatalf("provider tables were not unified:\n%s", patched)
	}
	for _, expected := range []string{
		`model_provider = "ciyuanshen"`,
		`[model_providers.ciyuanshen]`,
		`name = "ciyuanshen"`,
		`base_url = "https://api.ciyuanshen.top/v1"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
	} {
		if !strings.Contains(patched, expected) {
			t.Fatalf("unified Codex config is missing %q:\n%s", expected, patched)
		}
	}
	if strings.Contains(patched, "model_providers.custom") || strings.Contains(patched, `name = "custom"`) {
		t.Fatalf("stale custom provider remained:\n%s", patched)
	}
	if strings.Count(patched, `name = "ciyuanshen"`) != 1 || strings.Count(patched, `base_url = "https://api.ciyuanshen.top/v1"`) != 1 {
		t.Fatalf("provider fields were duplicated:\n%s", patched)
	}
}

func TestPatchCodexConfigKeepsCustomProviderName(t *testing.T) {
	existing := `model_provider = "custom"
[model_providers.custom]
name = "custom"
base_url = "https://custom.example/v1"
wire_api = "responses"
requires_openai_auth = true
`
	patched := patchCodexConfig(existing, "")
	if !strings.Contains(patched, `model_provider = "custom"`) || !strings.Contains(patched, `[model_providers.custom]`) || !strings.Contains(patched, `name = "custom"`) {
		t.Fatalf("custom provider name was not preserved:\n%s", patched)
	}
	if !strings.Contains(patched, `base_url = "https://custom.example/v1"`) {
		t.Fatalf("custom provider URL was overwritten:\n%s", patched)
	}
	if strings.Contains(patched, `[model_providers.ciyuanshen]`) {
		t.Fatalf("unexpected ciyuanshen provider was added:\n%s", patched)
	}
}

func TestPatchCodexConfigPreservesUnrelatedProviderTable(t *testing.T) {
	existing := `model_provider = "ciyuanshen"
[model_providers.ciyuanshen]
name = "ciyuanshen"
base_url = "https://api.ciyuanshen.top/v1"

[model_providers.other]
name = "other"
base_url = "https://other.example/v1"
wire_api = "responses"
requires_openai_auth = true
`
	patched := patchCodexConfig(existing, "")
	for _, expected := range []string{
		`[model_providers.ciyuanshen]`,
		`[model_providers.other]`,
		`name = "other"`,
		`base_url = "https://other.example/v1"`,
	} {
		if !strings.Contains(patched, expected) {
			t.Fatalf("unrelated provider was not preserved (%q):\n%s", expected, patched)
		}
	}
}

func TestPatchCodexConfigUnifiesAnyProviderName(t *testing.T) {
	existing := `model_provider = "foo"
[model_providers.legacy]
name = "foo"
base_url = "https://foo.example/v1"
wire_api = "responses"
requires_openai_auth = true
legacy_value = "keep"

[model_providers.other]
name = "other"
base_url = "https://other.example/v1"
wire_api = "responses"
requires_openai_auth = true
`
	patched := patchCodexConfig(existing, "")
	for _, expected := range []string{
		`model_provider = "foo"`,
		`[model_providers.foo]`,
		`name = "foo"`,
		`base_url = "https://foo.example/v1"`,
		`legacy_value = "keep"`,
		`[model_providers.other]`,
		`name = "other"`,
		`base_url = "https://other.example/v1"`,
	} {
		if !strings.Contains(patched, expected) {
			t.Fatalf("arbitrary provider configuration is missing %q:\n%s", expected, patched)
		}
	}
	if strings.Contains(patched, "[model_providers.legacy]") || strings.Count(patched, "[model_providers.foo]") != 1 {
		t.Fatalf("provider table was not renamed and unified:\n%s", patched)
	}
}

func TestPatchCodexConfigMigratesSingleMismatchedProviderForAnyName(t *testing.T) {
	existing := `model_provider = "foo"
[model_providers.custom]
name = "custom"
base_url = "https://custom.example/v1"
wire_api = "responses"
requires_openai_auth = true
`
	patched := patchCodexConfig(existing, "")
	for _, expected := range []string{
		`model_provider = "foo"`,
		`[model_providers.foo]`,
		`name = "foo"`,
		`base_url = "https://custom.example/v1"`,
	} {
		if !strings.Contains(patched, expected) {
			t.Fatalf("single arbitrary provider was not migrated (%q):\n%s", expected, patched)
		}
	}
	if strings.Contains(patched, "[model_providers.custom]") || strings.Contains(patched, `name = "custom"`) {
		t.Fatalf("stale provider name remained:\n%s", patched)
	}
}

func TestConfigureAllSupportedClientsFromEmptyFiles(t *testing.T) {
	home := isolateHome(t)
	targets := []string{"claude", "codex", "gemini", "grok", "opencode", "openclaw", "hermes"}
	operations, _, err := buildConfiguration(ConfigurationRequest{
		APIKey:  "test-key",
		Targets: targets,
		Models:  map[string]string{"default": "gpt-test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range operations {
		if err := atomicWrite(operation.Path, operation.Content); err != nil {
			t.Fatal(err)
		}
		if err := validateWrittenConfig(operation); err != nil {
			t.Fatalf("%s output did not validate: %v", operation.ClientID, err)
		}
	}
	if len(operations) != 10 {
		t.Fatalf("expected 10 file operations, got %d", len(operations))
	}
	for _, target := range targets {
		if err := verifyManagedClientConfiguration(home, target, "test-key"); err != nil {
			t.Fatalf("%s output did not pass connection configuration check: %v", target, err)
		}
	}
}
