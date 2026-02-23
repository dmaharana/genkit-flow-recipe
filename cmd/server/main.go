package main

import (
	"context"
	"log"
	"net/http"

	"genkit-flow/internal/config"
	"genkit-flow/internal/flows"
	"genkit-flow/internal/ui"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai"
	"github.com/firebase/genkit/go/plugins/server"
)

func main() {
	// Load configuration
	cfg := config.Load()

	ctx := context.Background()
	llmModel := cfg.LLMModel()

	log.Printf("Initializing Genkit with provider: %s, model: %s, base URL: %s", cfg.Provider, cfg.Model, cfg.BaseURL)

	// Initialize Genkit with the OpenAI-compatible plugin
	g := genkit.Init(ctx,
		genkit.WithPlugins(&compat_oai.OpenAICompatible{
			BaseURL:  cfg.BaseURL,
			APIKey:   cfg.APIKey,
			Provider: cfg.Provider,
		}),
		genkit.WithDefaultModel(llmModel),
	)

	// Define recipe generator flow from internal/flows
	recipeGeneratorFlow := flows.DefineRecipeFlow(g)

	// Start server (Dev UI + flow endpoint)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/recipeGeneratorFlow", genkit.Handler(recipeGeneratorFlow))
	mux.HandleFunc("/recipeGeneratorFlow", genkit.Handler(recipeGeneratorFlow))
	mux.Handle("/", ui.Handler())

	addr := cfg.Addr()
	log.Printf("Starting server on http://%s", addr)
	log.Printf("Flow available at:  POST http://%s/api/recipeGeneratorFlow", addr)
	log.Printf("Open http://%s/ui for Genkit developer UI", addr)

	log.Fatal(server.Start(ctx, addr, mux))
}
