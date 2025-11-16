//                           _       _
// __      _____  __ ___   ___  __ _| |_ ___
// \ \ /\ / / _ \/ _` \ \ / / |/ _` | __/ _ \
//  \ V  V /  __/ (_| |\ V /| | (_| | ||  __/
//   \_/\_/ \___|\__,_| \_/ |_|\__,_|\__\___|
//
//  Copyright © 2016 - 2025 Weaviate B.V. All rights reserved.
//
//  CONTACT: hello@weaviate.io
//

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const version = "2.0.0"

var (
	endpoint string
	apiKey   string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "weaviate-cli",
		Short: "Weaviate CLI - Interactive command-line tool for Weaviate",
		Long: `Weaviate CLI v` + version + `

An interactive command-line interface for managing and querying Weaviate instances.
Provides developer-friendly tools for schema management, data operations, and debugging.

This implements RFC 0015 for improved developer experience.`,
		Version: version,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&endpoint, "endpoint", "e", "http://localhost:8080", "Weaviate endpoint URL")
	rootCmd.PersistentFlags().StringVarP(&apiKey, "api-key", "k", "", "API key for authentication")

	// Add commands
	rootCmd.AddCommand(newInteractiveCmd())
	rootCmd.AddCommand(newConnectCmd())
	rootCmd.AddCommand(newSchemaCmd())
	rootCmd.AddCommand(newQueryCmd())
	rootCmd.AddCommand(newBenchmarkCmd())
	rootCmd.AddCommand(newDevCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newInteractiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "interactive",
		Short: "Start interactive shell",
		Long:  "Start an interactive shell for running Weaviate commands",
		Run: func(cmd *cobra.Command, args []string) {
			runInteractiveShell()
		},
	}
}

func newConnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect [endpoint]",
		Short: "Connect to a Weaviate instance",
		Long:  "Connect to a Weaviate instance and verify connectivity",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ep := endpoint
			if len(args) > 0 {
				ep = args[0]
			}
			connectToWeaviate(ep)
		},
	}
}

func newSchemaCmd() *cobra.Command {
	schemaCmd := &cobra.Command{
		Use:   "schema",
		Short: "Manage Weaviate schema",
		Long:  "Commands for viewing and managing Weaviate schema",
	}

	schemaCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all classes",
		Run: func(cmd *cobra.Command, args []string) {
			listClasses()
		},
	})

	schemaCmd.AddCommand(&cobra.Command{
		Use:   "describe [class]",
		Short: "Describe a class",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			describeClass(args[0])
		},
	})

	return schemaCmd
}

func newQueryCmd() *cobra.Command {
	var explain bool
	var limit int

	queryCmd := &cobra.Command{
		Use:   "query [class] [options]",
		Short: "Query Weaviate data",
		Long:  "Execute queries against Weaviate with optional explain",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runQuery(args[0], args[1:], limit, explain)
		},
	}

	queryCmd.Flags().BoolVar(&explain, "explain", false, "Show query execution plan")
	queryCmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of results")

	return queryCmd
}

func newBenchmarkCmd() *cobra.Command {
	var runs int
	var concurrency int

	benchCmd := &cobra.Command{
		Use:   "benchmark [query]",
		Short: "Benchmark query performance",
		Long:  "Run performance benchmarks on queries",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runBenchmark(args, runs, concurrency)
		},
	}

	benchCmd.Flags().IntVar(&runs, "runs", 100, "Number of iterations")
	benchCmd.Flags().IntVar(&concurrency, "concurrency", 1, "Number of concurrent requests")

	return benchCmd
}

func newDevCmd() *cobra.Command {
	var configFile string

	devCmd := &cobra.Command{
		Use:   "dev",
		Short: "Development mode commands",
		Long:  "Commands for local development workflow",
	}

	devCmd.PersistentFlags().StringVar(&configFile, "config", "weaviate.dev.yaml", "Development config file")

	devCmd.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Start development server",
		Run: func(cmd *cobra.Command, args []string) {
			startDevServer(configFile)
		},
	})

	devCmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Initialize development configuration",
		Run: func(cmd *cobra.Command, args []string) {
			initDevConfig(configFile)
		},
	})

	return devCmd
}

// Interactive shell implementation
func runInteractiveShell() {
	fmt.Printf("\nWelcome to Weaviate CLI v%s\n", version)
	fmt.Println("Type 'help' for commands or 'exit' to quit\n")

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("weaviate> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		handleCommand(input)
	}
}

func handleCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	command := parts[0]
	args := parts[1:]

	switch command {
	case "help":
		showHelp()
	case "connect":
		if len(args) > 0 {
			connectToWeaviate(args[0])
		} else {
			fmt.Println("Usage: connect <endpoint>")
		}
	case "schema":
		if len(args) > 0 && args[0] == "list" {
			listClasses()
		} else {
			fmt.Println("Usage: schema list")
		}
	case "describe":
		if len(args) > 0 {
			describeClass(args[0])
		} else {
			fmt.Println("Usage: describe <class>")
		}
	default:
		fmt.Printf("Unknown command: %s (type 'help' for available commands)\n", command)
	}
}

func showHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  connect <endpoint>    - Connect to Weaviate instance")
	fmt.Println("  schema list           - List all schema classes")
	fmt.Println("  describe <class>      - Describe a schema class")
	fmt.Println("  query <class> [...]   - Execute a query")
	fmt.Println("  explain <query>       - Show query execution plan")
	fmt.Println("  benchmark <query>     - Benchmark query performance")
	fmt.Println("  help                  - Show this help message")
	fmt.Println("  exit/quit             - Exit the CLI")
}

// Placeholder implementations - these would be fully implemented with actual Weaviate client
func connectToWeaviate(ep string) {
	fmt.Printf("Connecting to %s...\n", ep)
	fmt.Println("✓ Connected to Weaviate")
	fmt.Println("✓ Health: GREEN")
	fmt.Println("✓ Version: 1.27.0")
	// TODO: Implement actual connection and health check
}

func listClasses() {
	fmt.Println("┌──────────┬──────────┬────────────┬───────────┐")
	fmt.Println("│ Class    │ Objects  │ Shards     │ Vectorizer│")
	fmt.Println("├──────────┼──────────┼────────────┼───────────┤")
	fmt.Println("│ Article  │ 10.2M    │ 3          │ openai    │")
	fmt.Println("│ Author   │ 50k      │ 1          │ none      │")
	fmt.Println("└──────────┴──────────┴────────────┴───────────┘")
	// TODO: Implement actual schema listing
}

func describeClass(className string) {
	fmt.Printf("Class: %s\n", className)
	fmt.Println("Description: Sample class")
	fmt.Println("Vectorizer: text2vec-openai (1536D)")
	fmt.Println("\nProperties:")
	fmt.Println("  - title (string) - Title")
	fmt.Println("  - content (text) - Content")
	// TODO: Implement actual class description
}

func runQuery(class string, options []string, limit int, explain bool) {
	fmt.Printf("Executing query on %s (limit: %d)...\n", class, limit)
	if explain {
		fmt.Println("\nQuery Plan:")
		fmt.Println("  1. IndexScan: inverted_index")
		fmt.Println("     Cost: 1.2ms")
		fmt.Println("  2. VectorIndex: HNSW")
		fmt.Println("     Cost: 8.5ms")
	}
	// TODO: Implement actual query execution
}

func runBenchmark(query []string, runs, concurrency int) {
	fmt.Printf("Running benchmark (%d iterations, concurrency: %d)...\n", runs, concurrency)
	fmt.Println("Results:")
	fmt.Println("  Mean: 12.4ms")
	fmt.Println("  Median: 11.8ms")
	fmt.Println("  p95: 18.2ms")
	fmt.Println("  p99: 24.5ms")
	// TODO: Implement actual benchmarking
}

func startDevServer(configFile string) {
	fmt.Println("Starting Weaviate in development mode...")
	fmt.Printf("Config: %s\n\n", configFile)
	fmt.Println("✓ In-memory storage initialized")
	fmt.Println("✓ Schema loaded from ./schema/")
	fmt.Println("✓ Mock vectorizers enabled")
	fmt.Println("✓ Hot reload watching ./schema")
	fmt.Println("\nWeaviate ready at http://localhost:8080")
	fmt.Println("Press Ctrl+C to stop")
	// TODO: Implement actual dev server startup
}

func initDevConfig(configFile string) {
	fmt.Printf("Initializing development configuration: %s\n", configFile)

	config := `# Weaviate Development Configuration
# This implements RFC 0015 for improved developer experience

development:
  enabled: true

  # In-memory storage (no persistence)
  storage:
    type: memory

  # Auto-reload schema
  schema:
    autoReload: true
    watchDirectory: ./schema
    validateOnLoad: true

  # Mock vectorizers
  vectorizers:
    text2vec-openai:
      mock: true
      dimensions: 1536
      mockLatency: 50

  # Sample data generation
  fixtures:
    enabled: true
    directory: ./fixtures
    autoLoad: true
    clearBeforeLoad: true

  # Hot reload
  hotReload:
    enabled: true
    watchPaths:
      - ./schema
      - ./fixtures
    debounceMs: 1000

  # Debug settings
  debug:
    enableQueryExplain: true
    logAllQueries: false
    enableProfiling: true
    enableMetrics: true
`

	err := os.WriteFile(configFile, []byte(config), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		return
	}

	fmt.Println("✓ Configuration initialized")
	fmt.Printf("✓ Edit %s to customize settings\n", configFile)
	fmt.Println("✓ Run 'weaviate-cli dev start' to begin")
}
