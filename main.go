package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai"
	"github.com/firebase/genkit/go/plugins/server"
)

// Define input schema
type RecipeInput struct {
	Ingredient          string `json:"ingredient" jsonschema:"description=Main ingredient or cuisine type"`
	DietaryRestrictions string `json:"dietaryRestrictions,omitempty" jsonschema:"description=Any dietary restrictions"`
}

// Define output schema
type Recipe struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	PrepTime     string   `json:"prepTime"`
	CookTime     string   `json:"cookTime"`
	Servings     int      `json:"servings"`
	Ingredients  []string `json:"ingredients"`
	Instructions []string `json:"instructions"`
	Tips         []string `json:"tips,omitempty"`
}

func main() {
	ctx := context.Background()
	// model := "gemma3:1bq4"
	// model := "lfm2.5-thinking:1.2b"
	model := "qwen3:0.6b"
	llmModel := fmt.Sprintf("%s/%s", "ollama", model)

	// Initialize Genkit with the Ollama plugin
	g := genkit.Init(ctx,
		genkit.WithPlugins(&compat_oai.OpenAICompatible{
			BaseURL:  "http://127.0.0.1:11434/v1",
			APIKey:   "ollama", // dummy — Ollama ignores it
			Provider: "ollama",
		}),
		// genkit.WithDefaultModel("ollama/deepseek-v3.1:671b-cloud"),
		// genkit.WithDefaultModel("ollama/qwen3:0.6b"), // or just "llama3.1" — some versions trim prefix
		genkit.WithDefaultModel(llmModel),

		// Tip: use a model that understands structured output well
		// Good 2025–2026 choices: llama3.1:8b, llama3.2:3b, gemma2:9b, qwen2.5:14b, etc.
	)

	// Define a recipe generator flow
	recipeGeneratorFlow := genkit.DefineFlow(g, "recipeGeneratorFlow", func(ctx context.Context, input *RecipeInput) (*Recipe, error) {
		dietaryRestrictions := input.DietaryRestrictions
		if dietaryRestrictions == "" {
			dietaryRestrictions = "none"
		}

		prompt := fmt.Sprintf(`You are a professional chef. Create a complete, realistic recipe using ONLY the following constraints:

Main ingredient / theme: %s
Dietary restrictions: %s

Return the answer **strictly** as valid JSON matching this exact schema — no extra text, no markdown, no comments:

{
  "title": string,
  "description": string,
  "prepTime": string,      // e.g. "15 minutes"
  "cookTime": string,      // e.g. "30 minutes"
  "servings": integer,
  "ingredients": string[],
  "instructions": string[],
  "tips": string[]         // optional
}

Be concise, accurate and creative.`, input.Ingredient, dietaryRestrictions)

		// Generate structured recipe data
		recipe, _, err := genkit.GenerateData[Recipe](ctx, g,
			ai.WithPrompt(prompt),
			// Optional: force stronger JSON adherence (works better on some models)
			// ai.WithTemperature(0.1),
			ai.WithConfig(map[string]any{"response_format": map[string]string{"type": "json_object"}}),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to generate recipe: %w", err)
		}

		return recipe, nil
	})

	log.Printf("Running quick test with model: %s", llmModel)
	// Quick test run
	recipe, err := recipeGeneratorFlow.Run(ctx, &RecipeInput{
		Ingredient:          "avocado",
		DietaryRestrictions: "vegetarian",
	})
	if err != nil {
		log.Fatalf("could not generate recipe: %v", err)
	}

	recipeJSON, _ := json.MarshalIndent(recipe, "", "  ")
	fmt.Println("Sample recipe generated:")
	fmt.Println(string(recipeJSON))

	// Start server (Dev UI + flow endpoint)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /recipeGeneratorFlow", genkit.Handler(recipeGeneratorFlow))

	log.Println("Starting server on http://localhost:3400")
	log.Println("Flow available at:  POST http://localhost:3400/recipeGeneratorFlow")
	log.Println("Open http://localhost:3400/ui for Genkit developer UI")

	log.Fatal(server.Start(ctx, "127.0.0.1:3400", mux))
}
