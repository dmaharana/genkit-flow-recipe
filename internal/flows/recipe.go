package flows

import (
	"context"
	"fmt"
	"log"

	"genkit-flow/internal/models"
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
	"github.com/firebase/genkit/go/genkit"
)

// DefineRecipeFlow initializes and registers the recipe generator flow.
func DefineRecipeFlow(g *genkit.Genkit) *core.Flow[*models.RecipeInput, *models.Recipe, struct{}] {
	return genkit.DefineFlow(g, "recipeGeneratorFlow", func(ctx context.Context, input *models.RecipeInput) (*models.Recipe, error) {
		dietaryRestrictions := input.DietaryRestrictions
		if dietaryRestrictions == "" {
			dietaryRestrictions = "none"
		}

		prompt := fmt.Sprintf(`You are a professional chef. Create a complete, realistic recipe using ONLY the following constraints:

Main ingredient / theme: %s
Dietary restrictions: %s

Return the answer **strictly** as valid JSON matching this exact schema — no extra text, no markdown, no comments:

{
  "title": "Recipe Title",
  "description": "Short description",
  "prepTime": "15 minutes",
  "cookTime": "30 minutes",
  "servings": 4,
  "ingredients": ["item 1", "item 2"],
  "instructions": ["step 1", "step 2"],
  "tips": ["optional tip 1"]
}

Be concise, accurate and creative.`, input.Ingredient, dietaryRestrictions)

		// Generate structured recipe data with retries
		var recipe *models.Recipe
		var err error
		maxRetries := 3

		for i := 0; i < maxRetries; i++ {
			recipe, _, err = genkit.GenerateData[models.Recipe](ctx, g,
				ai.WithPrompt(prompt),
				// Optional: force stronger JSON adherence (works better on some models)
				// ai.WithTemperature(0.1),
				ai.WithConfig(map[string]any{"response_format": map[string]string{"type": "json_object"}}),
			)
			if err == nil {
				break
			}
			log.Printf("Attempt %d failed: %v. Retrying...", i+1, err)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to generate recipe after %d attempts: %w", maxRetries, err)
		}

		return recipe, nil
	})
}
