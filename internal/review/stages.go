package review

import "github.com/LucasNav6/code-review-cli/internal/prompts"

func securityPrompt() string        { return prompts.Security }
func maintainabilityPrompt() string { return prompts.Maintainability }
func testingPrompt() string         { return prompts.Testing }
func resiliencePrompt() string      { return prompts.Resilience }
