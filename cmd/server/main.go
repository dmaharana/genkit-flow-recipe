package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"
)

// FileTelemetryClient implements tracing.TelemetryClient to save traces to the filesystem.
type FileTelemetryClient struct {
	Dir string
}

func (c *FileTelemetryClient) Save(ctx context.Context, data *tracing.Data) error {
	if data == nil || data.TraceID == "" {
		return nil
	}
	
	path := filepath.Join(c.Dir, data.TraceID)
	// Ensure directory exists
	os.MkdirAll(c.Dir, 0755)
	
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	
	return json.NewEncoder(f).Encode(data)
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

func main() {
	// Load configuration
	cfg := config.Load()
	
	// Register file-based telemetry to save traces to .genkit/traces
	wd, _ := os.Getwd()
	traceDir := filepath.Join(wd, ".genkit/traces")
	tracing.WriteTelemetryImmediate(&FileTelemetryClient{Dir: traceDir})

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
		log.Printf("Enabling OpenTelemetry tracing with OTLP endpoint: %s and console logs", cfg.OTLPEndpoint)

		// Create OTLP exporter
		otlpExporter, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithInsecure(),
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		)
		if err != nil {
			log.Fatalf("failed to create OTLP exporter: %v", err)
		}

		// Create Console exporter
		stdoutExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			log.Fatalf("failed to create stdout exporter: %v", err)
		}

		// Combine both exporters
		multiExporter := &MultiSpanExporter{
			exporters: []trace.SpanExporter{otlpExporter, stdoutExporter},
		}

		// Ensure graceful shutdown to flush remaining spans
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := multiExporter.Shutdown(shutdownCtx); err != nil {
				log.Printf("Error shutting down OpenTelemetry exporters: %v", err)
			}
		}()

		otelPlugin := opentelemetry.New(opentelemetry.Config{
			ServiceName:   "genkit-flow-app",
			TraceExporter: multiExporter,
			ForceExport:   true,
		})
		plugins = append(plugins, otelPlugin)
	}

	// Initialize Genkit with the plugins
	g := genkit.Init(ctx,
		genkit.WithPlugins(plugins...),
		genkit.WithDefaultModel(llmModel),
	)

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
		// Get current working directory for robustness
		wd, _ := os.Getwd()
		traceDir := filepath.Join(wd, ".genkit/traces")
		
		files, err := os.ReadDir(traceDir)
		if err != nil {
			http.Error(w, "Trace directory not found: "+err.Error(), http.StatusInternalServerError)
			return
		}
		
		type TraceFile struct {
			Name    string
			ModTime time.Time
		}
		var traceFiles []TraceFile
		for _, f := range files {
			if !f.IsDir() {
				info, err := f.Info()
				if err == nil {
					traceFiles = append(traceFiles, TraceFile{
						Name:    f.Name(),
						ModTime: info.ModTime(),
					})
				}
			}
		}
		
		// Sort by ModTime descending
		sort.Slice(traceFiles, func(i, j int) bool {
			return traceFiles[i].ModTime.After(traceFiles[j].ModTime)
		})
		
		var traces []string
		for _, tf := range traceFiles {
			traces = append(traces, tf.Name)
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
		data, err := os.ReadFile(".genkit/traces/" + id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	mux.Handle("/", ui.Handler())

	addr := cfg.Addr()
	log.Printf("Starting server on http://%s", addr)
	log.Printf("Flow available at:  POST http://%s/api/recipeGeneratorFlow", addr)
	log.Printf("Open http://%s/ui for Genkit developer UI", addr)

	log.Fatal(server.Start(ctx, addr, mux))
}
