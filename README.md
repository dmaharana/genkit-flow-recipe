# Genkit Recipe Generator Flow

A Go-based recipe generation service powered by [Firebase Genkit](https://github.com/firebase/genkit) and [Ollama](https://ollama.com/). It uses structured output to ensure the LLM returns valid JSON matching a specific schema, with built-in retry logic for robustness.

## Project Structure

```text
genkit-flow/
├── bin/              # Compiled binaries
├── cmd/
│   └── server/       # Main entry point
├── internal/
│   ├── flows/        # Genkit flow definitions
│   └── models/       # Data models (RecipeInput, Recipe)
├── Makefile          # Build and run tasks
└── go.mod            # Dependencies
```

## Prerequisites

1.  **Go**: version 1.25 or higher.
2.  **Ollama**: Ensure Ollama is installed and running locally.
3.  **Models**: The default model is `qwen3:0.6b`. You can pull it using:
    ```bash
    ollama pull qwen3:0.6b
    ```

## Getting Started

### Installation

Clone the repository and install dependencies:

```bash
go mod download
```

## Configuration

You can configure the LLM settings using environment variables, a `.env` file, or command-line parameters. Command-line parameters take the highest precedence, followed by environment variables/`.env` file, and finally the default values.

### Environment Variables / .env

Create a `.env` file in the root directory (see `.env.example`):

```env
LLM_PROVIDER=ollama
LLM_BASE_URL=http://127.0.0.1:11434/v1
LLM_API_KEY=ollama
LLM_MODEL=qwen3:0.6b
PORT=3400
```

### Command-line Parameters

When running the server, you can pass parameters:

```bash
./bin/genkit-server -provider openai -base-url https://api.openai.com/v1 -api-key YOUR_API_KEY -model gpt-4o -port 8080
```

### Running the Server

You can run the server directly using:

```bash
make run
```

The server will start on `http://localhost:3400`.

### Building the Binary

To build a compressed binary (using `-ldflags="-s -w"`):

```bash
make build
```

The binary will be located in `bin/genkit-server`.

## Usage

### API Endpoint

The recipe generator is exposed as a POST endpoint:

**POST** `http://localhost:3400/recipeGeneratorFlow`

**Body:**
```json
{
  "ingredient": "avocado",
  "dietaryRestrictions": "vegetarian"
}
```

### Developer UI

Genkit provides a built-in UI for inspecting and testing flows:

Open [http://localhost:3400/ui](http://localhost:3400/ui) in your browser.

## Features

- **Structured Output**: Uses `genkit.GenerateData` to enforce JSON schema adherence.
- **Robustness**: Includes a retry mechanism (3 attempts) to handle occasional schema validation or connection failures.
- **Clean Architecture**: Follows standard Go project layout for better maintainability.
- **Compressed Binaries**: Makefile optimized for small binary sizes.
