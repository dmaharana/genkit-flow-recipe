package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"genkit-flow/internal/config"
	"genkit-flow/internal/flows"
	"genkit-flow/internal/ui"
	"github.com/firebase/genkit/go/core/api"
	"github.com/firebase/genkit/go/core/tracing"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai"
	"github.com/firebase/genkit/go/plugins/server"
	opentelemetry "github.com/xavidop/genkit-opentelemetry-go"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/trace"

	_ "modernc.org/sqlite"
)

// SQLiteTelemetryClient implements tracing.TelemetryClient to save traces to a SQLite database.
type SQLiteTelemetryClient struct {
	db *sql.DB
}

func (c *SQLiteTelemetryClient) Save(ctx context.Context, data *tracing.Data) error {
	if data == nil || data.TraceID == "" {
		log.Printf("SQLiteTelemetryClient: received empty or nil trace data")
		return nil
	}

	// Extract tokens from spans for DB storage
	var inputTokens, outputTokens int
	for _, span := range data.Spans {
		if span.Attributes != nil {
			if it, ok := span.Attributes["genkit:metadata:input_tokens"]; ok {
				inputTokens = int(toInt64(it))
			}
			if ot, ok := span.Attributes["genkit:metadata:output_tokens"]; ok {
				outputTokens = int(toInt64(ot))
			}
		}
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("SQLiteTelemetryClient: failed to marshal trace %s: %v", data.TraceID, err)
		return err
	}

	_, err = c.db.ExecContext(ctx, `
		INSERT INTO traces (id, data, input_tokens, output_tokens, created_at) 
		VALUES (?, ?, ?, ?, ?) 
		ON CONFLICT(id) DO UPDATE SET data = ?, input_tokens = ?, output_tokens = ?, created_at = ?`,
		data.TraceID, string(jsonData), inputTokens, outputTokens, time.Now(),
		string(jsonData), inputTokens, outputTokens, time.Now())
	
	if err != nil {
		log.Printf("SQLiteTelemetryClient: failed to save trace %s to DB: %v", data.TraceID, err)
	} else {
		log.Printf("SQLiteTelemetryClient: successfully saved trace %s to DB", data.TraceID)
	}
	return err
}

func toInt64(v any) int64 {
	switch i := v.(type) {
	case int:
		return int64(i)
	case int64:
		return i
	case float64:
		return int64(i)
	default:
		return 0
	}
}

// MultiSpanExporter implements trace.SpanExporter by forwarding spans to multiple exporters.
type MultiSpanExporter struct {
	exporters []trace.SpanExporter
}

func (m *MultiSpanExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	for _, e := range m.exporters {
		if err := e.ExportSpans(ctx, spans); err != nil {
			return err
		}
	}
	return nil
}

func (m *MultiSpanExporter) Shutdown(ctx context.Context) error {
	for _, e := range m.exporters {
		if err := e.Shutdown(ctx); err != nil {
			return err
		}
	}
	return nil
}

func initDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite", "genkit_traces.db")
	if err != nil {
		return nil, err
	}

	// Create table if not exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS traces (
			id TEXT PRIMARY KEY,
			data TEXT,
			input_tokens INTEGER DEFAULT 0,
			output_tokens INTEGER DEFAULT 0,
			created_at DATETIME
		)`)
	if err != nil {
		return nil, err
	}

	// Check if columns exist (for migration)
	var count int
	err = db.QueryRow("SELECT count(*) FROM pragma_table_info('traces') WHERE name='input_tokens'").Scan(&count)
	if err == nil && count == 0 {
		_, _ = db.Exec("ALTER TABLE traces ADD COLUMN input_tokens INTEGER DEFAULT 0")
		_, _ = db.Exec("ALTER TABLE traces ADD COLUMN output_tokens INTEGER DEFAULT 0")
	}

	return db, nil
}

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize SQLite DB
	db, err := initDB()
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	llmModel := cfg.LLMModel()

	log.Printf("Initializing Genkit with provider: %s, model: %s, base URL: %s", cfg.Provider, cfg.Model, cfg.BaseURL)

	// OpenTelemetry configuration
	var plugins []api.Plugin
	plugins = append(plugins, &compat_oai.OpenAICompatible{
		BaseURL:  cfg.BaseURL,
		APIKey:   cfg.APIKey,
		Provider: cfg.Provider,
	})

	if cfg.TracingEnabled {
		log.Printf("Enabling OpenTelemetry tracing with OTLP endpoint: %s", cfg.OTLPEndpoint)

		// Create OTLP exporter
		otlpExporter, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithInsecure(),
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		)
		if err != nil {
			log.Fatalf("failed to create OTLP exporter: %v", err)
		}

		// Ensure graceful shutdown to flush remaining spans
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := otlpExporter.Shutdown(shutdownCtx); err != nil {
				log.Printf("Error shutting down OpenTelemetry exporter: %v", err)
			}
		}()

		otelPlugin := opentelemetry.New(opentelemetry.Config{
			ServiceName:   "genkit-flow-app",
			TraceExporter: otlpExporter,
			ForceExport:   true,
		})
		plugins = append(plugins, otelPlugin)
	}

	// Initialize Genkit with the plugins
	g := genkit.Init(ctx,
		genkit.WithPlugins(plugins...),
		genkit.WithDefaultModel(llmModel),
	)

	// Register SQLite-based telemetry (DO THIS AFTER genkit.Init)
	tracing.WriteTelemetryImmediate(&SQLiteTelemetryClient{db: db})

	// Define recipe generator flow from internal/flows
	recipeGeneratorFlow := flows.DefineRecipeFlow(g)

	// Start server (Dev UI + flow endpoint)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/recipeGeneratorFlow", genkit.Handler(recipeGeneratorFlow))
	mux.HandleFunc("/recipeGeneratorFlow", genkit.Handler(recipeGeneratorFlow))

	// Admin API for traces
	mux.HandleFunc("/api/admin/traces", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		rows, err := db.QueryContext(r.Context(), "SELECT id FROM traces ORDER BY created_at DESC")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		traces := []string{}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			traces = append(traces, id)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(traces)
	})

	mux.HandleFunc("/api/admin/traces/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Path[len("/api/admin/traces/"):]
		if id == "" {
			http.Error(w, "Trace ID is required", http.StatusBadRequest)
			return
		}

		var jsonData string
		err := db.QueryRowContext(r.Context(), "SELECT data FROM traces WHERE id = ?", id).Scan(&jsonData)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Trace not found", http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonData))
	})

	mux.Handle("/", ui.Handler())

	addr := cfg.Addr()
	log.Printf("Starting server on http://%s", addr)
	log.Printf("Flow available at:  POST http://%s/api/recipeGeneratorFlow", addr)
	log.Printf("Open http://%s/ui for Genkit developer UI", addr)

	log.Fatal(server.Start(ctx, addr, mux))
}
