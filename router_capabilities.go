package main

import "fmt"

// Capabilities are explicit overrides, not guesses based on a model's name.
// Zero/nil values retain the bridge's existing compatibility defaults.
type RouterModelOptions struct {
	ContextWindow     int   `json:"contextWindow,omitempty"`
	Reasoning         bool  `json:"reasoning"`
	ImageInput        *bool `json:"imageInput,omitempty"`
	ParallelToolCalls *bool `json:"parallelToolCalls,omitempty"`
}

func (o RouterModelOptions) contextWindow() int {
	if o.ContextWindow > 0 {
		return o.ContextWindow
	}
	return 114688
}

func routeCapability(enabled *bool) bool { return enabled == nil || *enabled }

func validateRouterModelOptions(options map[string]RouterModelOptions) error {
	for model, o := range options {
		if o.ContextWindow != 0 && (o.ContextWindow < 1024 || o.ContextWindow > 10000000) {
			return fmt.Errorf("模型 %q 的上下文长度须在 1024 到 10000000 之间", model)
		}
	}
	return nil
}

func routerCatalogModel(model, defaultModel string, o RouterModelOptions) map[string]any {
	levels := []map[string]string{}
	defaultEffort := "none"
	if o.Reasoning {
		for _, effort := range []string{"low", "medium", "high"} {
			levels = append(levels, map[string]string{"effort": effort, "description": effort})
		}
		defaultEffort = "medium"
	}
	modalities := []string{"text"}
	if routeCapability(o.ImageInput) {
		modalities = append(modalities, "image")
	}
	return map[string]any{
		"slug": model, "display_name": model, "description": "词元神路由模型",
		"shell_type": "shell_command", "base_instructions": "You are a coding assistant. Use the provided tools to complete the user's task.",
		"default_reasoning_summary": "none", "truncation_policy": map[string]any{"mode": "bytes", "limit": 10000},
		"supports_search_tool": false, "experimental_supported_tools": []string{},
		"supported_reasoning_levels": levels, "default_reasoning_level": defaultEffort,
		"context_window": o.contextWindow(), "max_context_window": o.contextWindow(),
		"effective_context_window_percent": 100, "supports_reasoning_summaries": o.Reasoning,
		"supports_parallel_tool_calls": routeCapability(o.ParallelToolCalls),
		"input_modalities":             modalities, "support_verbosity": false,
		"visibility": "list", "supported_in_api": true, "priority": 0, "is_default": model == defaultModel,
	}
}
