package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"genkit-flow/internal/flows"
	"genkit-flow/internal/models"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai"
	"github.com/firebase/genkit/go/plugins/server"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables or flags")
	}

	// Define flags
	providerFlag := flag.String("provider", getEnv("LLM_PROVIDER", "ollama"), "LLM provider (e.g., ollama, openai)")
	baseURLFlag := flag.String("base-url", getEnv("LLM_BASE_URL", "http://127.0.0.1:11434/v1"), "LLM base URL")
	apiKeyFlag := flag.String("api-key", getEnv("LLM_API_KEY", "ollama"), "LLM API key")
	modelFlag := flag.String("model", getEnv("LLM_MODEL", "qwen3:0.6b"), "LLM model name")
	portFlag := flag.String("port", getEnv("PORT", "3400"), "Server port")
	flag.Parse()

	provider := *providerFlag
	baseURL := *baseURLFlag
	apiKey := *apiKeyFlag
	modelName := *modelFlag
	port := *portFlag

	ctx := context.Background()
	llmModel := fmt.Sprintf("%s/%s", provider, modelName)

	log.Printf("Initializing Genkit with provider: %s, model: %s, base URL: %s", provider, modelName, baseURL)

	// Initialize Genkit with the OpenAI-compatible plugin
	g := genkit.Init(ctx,
		genkit.WithPlugins(&compat_oai.OpenAICompatible{
			BaseURL:  baseURL,
			APIKey:   apiKey,
			Provider: provider,
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

	addr := fmt.Sprintf("127.0.0.1:%s", port)
	log.Printf("Starting server on http://%s", addr)
	log.Printf("Flow available at:  POST http://%s/recipeGeneratorFlow", addr)
	log.Printf("Open http://%s/ui for Genkit developer UI", addr)

	log.Fatal(server.Start(ctx, addr, mux))
}

// getEnv retrieves the value of the environment variable named by the key.
// It returns the value, which will be empty if the variable is not present.
// If a default value is provided, it returns that if the variable is not present.
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
