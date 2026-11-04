package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAdditionalClientsRoundTrip(t *testing.T) {
	for _, target := range []string{"deepseek-harness", "pi", "mcode", "kimi", "antigravity", "omp"} {
		t.Run(target, func(t *testing.T) {
			home := t.TempDir()
			if target == "antigravity" {
				if runtime.GOOS == "windows" {
					t.Skip("fixture uses POSIX shell")
				}
				binDir := filepath.Join(home, "bin")
				if err := os.MkdirAll(binDir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(binDir, "agy"), []byte("#!/bin/sh\nprintf 'gemini-3-pro\\tGemini 3 Pro (High)\\ngemini-3-flash\\tGemini 3 Flash (Medium)\\n'\n"), 0700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			}
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			for _, key := range []string{"DSH_HOME", "PI_CODING_AGENT_DIR", "MINIMAX_DATA_DIR", "MAVIS_DATA_DIR", "KIMI_CODE_HOME", "OMP_PROFILE", "PI_PROFILE", "PI_CONFIG_DIR"} {
				t.Setenv(key, "")
			}
			if models := filterModelsForClient(target, []Model{{ID: "deepseek-v4.1-flash"}, {ID: "gemini-3-pro"}}); len(models) == 0 {
				t.Fatal("models filtered out")
			}
			operations, _, err := buildConfiguration(ConfigurationRequest{APIKey: "test-key", Targets: []string{target}, Models: map[string]string{target: "gemini-3-pro"}})
			if err != nil {
				t.Fatal(err)
			}
			for _, op := range operations {
				if err := atomicWrite(op.Path, op.Content); err != nil {
					t.Fatal(err)
				}
				if err := validateWrittenConfig(op); err != nil {
					t.Fatal(err)
				}
			}
			key, err := readConfiguredClientAPIKey(home, target)
			if err != nil || key != "test-key" {
				t.Fatalf("credential roundtrip: %q %v", key, err)
			}
			if err := verifyManagedClientConfiguration(home, target, key); err != nil {
				t.Fatal(err)
			}
			if err := updateConfiguredClientModel(home, target, "gemini-3-flash"); err != nil {
				t.Fatal(err)
			}
			model, err := readConfiguredClientDefaultModel(home, target)
			if err != nil || model != "gemini-3-flash" {
				t.Fatalf("model roundtrip: %q %v", model, err)
			}
			if err := verifyManagedClientConfiguration(home, target, key); err != nil {
				t.Fatal(err)
			}
			view, err := NewApp().GetClientConfiguration(target, false)
			if err != nil {
				t.Fatal(err)
			}
			for _, file := range view.Files {
				if strings.Contains(file.Content, "test-key") {
					t.Fatalf("secret leaked in %s", file.Path)
				}
			}
		})
	}
}

func TestDeepSeekHarnessPreservesNativePluginsAndCredentials(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DSH_HOME", filepath.Join(home, "custom-dsh"))
	paths := deepSeekHarnessPaths(home)
	original := "# user plugin\n- id: other\n  config:\n    expression: !!js process.env.HOME\n- id: llm-pi-ai\n  config:\n    retryPolicy: {mode: normal}\n    providers:\n      native: {apiKeyEnv: NATIVE_KEY}\n"
	if err := atomicWrite(paths[0], []byte(original)); err != nil {
		t.Fatal(err)
	}
	credential := filepath.Join(dshRoot(home), ".credentials.yaml")
	if err := atomicWrite(credential, []byte("version: 1\nrefs: {NATIVE_KEY: keep}\nrecords: {account: {value: keep-login}}\n")); err != nil {
		t.Fatal(err)
	}
	headless := filepath.Join(dshRoot(home), "profiles", "headless", "cordis.patch.yml")
	if err := atomicWrite(headless, []byte("[]\n")); err != nil {
		t.Fatal(err)
	}
	ops, err := configureDeepSeekHarness(home, "new-key", "deepseek-v4")
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 4 {
		t.Fatalf("profiles/credential count: %d", len(ops))
	}
	for _, op := range ops {
		if err := atomicWrite(op.Path, op.Content); err != nil {
			t.Fatal(err)
		}
	}
	content, _ := os.ReadFile(paths[0])
	if !strings.Contains(string(content), "!!js") || !strings.Contains(string(content), "retryPolicy") || !strings.Contains(string(content), "native:") {
		t.Fatalf("native content lost: %s", content)
	}
	creds, err := readYAMLMap(credential)
	if err != nil {
		t.Fatal(err)
	}
	if creds["records"] == nil {
		t.Fatal("login records lost")
	}
	if err := atomicWrite(filepath.Join(dshRoot(home), "cordis.patch.yml"), []byte("- id: llm-pi-ai\n  config: {providers: {}}\n")); err != nil {
		t.Fatal(err)
	}
	if _, err = configureDeepSeekHarness(home, "new-key", "model"); err == nil {
		t.Fatal("shadowed profile accepted")
	}
}

func TestAdditionalClientsNativeShapes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("OMP_PROFILE", "")
	t.Setenv("PI_PROFILE", "")
	t.Setenv("PI_CONFIG_DIR", "")
	ops, err := configureOMP(home, "key", "model")
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err = yaml.Unmarshal(ops[1].Content, &root); err != nil {
		t.Fatal(err)
	}
	if root["modelRoles"].(map[string]any)["default"] != "ciyuanshen/model" {
		t.Fatal("wrong omp selector")
	}
	pi := piPaths(home)[0]
	if err := atomicWrite(filepath.Join(filepath.Dir(pi), "settings.json"), []byte(`{"theme":"dark"}`)); err != nil {
		t.Fatal(err)
	}
	ops, err = configurePi(home, "key", "model")
	if err != nil {
		t.Fatal(err)
	}
	if ops[0].Path != pi {
		t.Fatal("models written into settings.json")
	}
	if !strings.Contains(string(ops[1].Content), "dark") {
		t.Fatal("user settings lost")
	}
	t.Setenv("OMP_PROFILE", "../outside")
	if err := validateAdditionalConfigurationPaths(home, "omp"); err == nil {
		t.Fatal("invalid profile accepted")
	}
}

func TestPiCredentialIsLiteral(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", "")
	key := "!do-not-run-$SECRET"
	ops, err := configurePi(home, key, "model")
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range ops {
		if err := atomicWrite(op.Path, op.Content); err != nil {
			t.Fatal(err)
		}
	}
	stored, err := readConfiguredClientAPIKey(home, "pi")
	if err != nil || stored != key {
		t.Fatalf("literal credential roundtrip failed: %v", err)
	}
	root, _ := readJSON5Map(piPaths(home)[0])
	provider := root["providers"].(map[string]any)[managedProviderName].(map[string]any)
	encoded := provider["apiKey"].(string)
	if strings.HasPrefix(encoded, "!") || !strings.Contains(encoded, "$$SECRET") {
		t.Fatal("credential would be interpreted by Pi")
	}
}

func TestAdditionalConfigurationRejectsInvalidNativeSections(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MINIMAX_DATA_DIR", "")
	t.Setenv("MAVIS_DATA_DIR", "")
	path := mcodePaths(home)[0]
	if err := atomicWrite(path, []byte("custom_provider: [invalid]\n")); err != nil {
		t.Fatal(err)
	}
	if err := validateAdditionalConfigurationPaths(home, "mcode"); err == nil {
		t.Fatal("invalid native section accepted")
	}
}
