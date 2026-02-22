package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"genkit-flow/internal/flows"
	"genkit-flow/internal/models"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai"
	"github.com/firebase/genkit/go/plugins/server"
)

func main() {
	ctx := context.Background()
	model := "qwen3:0.6b"
	llmModel := fmt.Sprintf("%s/%s", "ollama", model)

	// Initialize Genkit with the Ollama plugin
	g := genkit.Init(ctx,
		genkit.WithPlugins(&compat_oai.OpenAICompatible{
			BaseURL:  "http://127.0.0.1:11434/v1",
			APIKey:   "ollama", // dummy — Ollama ignores it
			Provider: "ollama",
		}),
		genkit.WithDefaultModel(llmModel),
	)

	// Define recipe generator flow from internal/flows
	recipeGeneratorFlow := flows.DefineRecipeFlow(g)

	log.Printf("Running quick test with model: %s", llmModel)
	// Quick test run
	recipe, err := recipeGeneratorFlow.Run(ctx, &models.RecipeInput{
		Ingredient:          "avocado",
		DietaryRestrictions: "vegetarian",
	})
	if err != nil {
		log.Printf("could not generate recipe in test run: %v", err)
		// We don't log.Fatal here so the server can still start
	} else {
		recipeJSON, _ := json.MarshalIndent(recipe, "", "  ")
		fmt.Println("Sample recipe generated:")
		fmt.Println(string(recipeJSON))
	}

	// Start server (Dev UI + flow endpoint)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /recipeGeneratorFlow", genkit.Handler(recipeGeneratorFlow))

	log.Println("Starting server on http://localhost:3400")
	log.Println("Flow available at:  POST http://localhost:3400/recipeGeneratorFlow")
	log.Println("Open http://localhost:3400/ui for Genkit developer UI")

	log.Fatal(server.Start(ctx, "127.0.0.1:3400", mux))
}
