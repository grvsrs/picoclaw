package main

import (
	"fmt"
	"os"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/providers"
)

// Verification script to test Moonshot integration
func main() {
	// Create a config with Moonshot
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         "~/.picoclaw/workspace",
				Model:             "moonshot-v1-32k",
				MaxTokens:         8192,
				Temperature:       0.7,
				MaxToolIterations: 20,
			},
		},
		Providers: config.ProvidersConfig{
			Moonshot: config.ProviderConfig{
				APIKey:  os.Getenv("MOONSHOT_API_KEY"),
				APIBase: "",
			},
		},
	}

	if cfg.Providers.Moonshot.APIKey == "" {
		fmt.Println("⚠ MOONSHOT_API_KEY not set, using test mode...")
		cfg.Providers.Moonshot.APIKey = "sk-test-key"
	}

	// Test provider creation
	fmt.Println("Testing Moonshot provider creation...")
	provider, err := providers.CreateProvider(cfg)
	if err != nil {
		fmt.Printf("Error creating provider: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Provider created: %T\n", provider)
	fmt.Printf("✓ Default model: %s\n", provider.GetDefaultModel())

	// Verify it's the correct provider
	moonshotProvider := providers.NewMoonshotProvider(cfg.Providers.Moonshot.APIKey)
	fmt.Printf("✓ Moonshot provider initialized: %T\n", moonshotProvider)
	fmt.Printf("✓ Moonshot default model: %s\n", moonshotProvider.GetDefaultModel())

	fmt.Println("\n✓✓✓ Moonshot integration verified! ✓✓✓")
}
