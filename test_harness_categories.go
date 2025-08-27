package main

import (
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/harness"
)

func main() {
	logger := &observability.NoopLogger{}
	provider := harness.NewHarnessProvider(logger)

	definitions := provider.GetAIOptimizedDefinitions()

	categories := make(map[string]bool)
	for _, def := range definitions {
		categories[def.Category] = true
		fmt.Printf("Name: %s, Category: %s\n", def.Name, def.Category)
	}

	fmt.Println("\nUnique categories:")
	for cat := range categories {
		fmt.Printf("- %s\n", cat)
	}
}
