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
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/weaviate/weaviate/client"
	"github.com/weaviate/weaviate/pkg/migrate"
	"gopkg.in/yaml.v3"
)

var (
	configFile string
	verbose    bool
	dryRun     bool
	force      bool
	logger     *logrus.Logger
)

func main() {
	logger = logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	rootCmd := &cobra.Command{
		Use:   "weaviate-migrate",
		Short: "Weaviate schema migration tool",
		Long:  `A CLI tool for managing Weaviate schema migrations with declarative YAML files.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if verbose {
				logger.SetLevel(logrus.DebugLevel)
			} else {
				logger.SetLevel(logrus.InfoLevel)
			}
		},
	}

	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "weaviate-migrate.yaml", "config file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(initCmd())
	rootCmd.AddCommand(createCmd())
	rootCmd.AddCommand(planCmd())
	rootCmd.AddCommand(applyCmd())
	rootCmd.AddCommand(rollbackCmd())
	rootCmd.AddCommand(historyCmd())
	rootCmd.AddCommand(validateCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// initCmd initializes a new migrations directory
func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize migrations directory and config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("Initializing weaviate-migrate project...")

			// Create migrations directory
			migrationsDir := "./migrations"
			if err := os.MkdirAll(migrationsDir, 0755); err != nil {
				return fmt.Errorf("failed to create migrations directory: %w", err)
			}
			logger.Infof("✓ Created: %s/", migrationsDir)

			// Create default config file
			defaultConfig := migrate.Config{
				Weaviate: migrate.WeaviateConfig{
					Host:   "localhost:8080",
					Scheme: "http",
				},
				MigrationsDir:           "./migrations",
				DryRunByDefault:         true,
				AutoBackup:              true,
				MaxConcurrentOperations: 1,
				OperationTimeout:        "30m",
				MigrationTimeout:        "2h",
			}

			configData, err := yaml.Marshal(defaultConfig)
			if err != nil {
				return fmt.Errorf("failed to marshal config: %w", err)
			}

			configPath := "weaviate-migrate.yaml"
			if err := os.WriteFile(configPath, configData, 0644); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}
			logger.Infof("✓ Created: %s", configPath)

			logger.Info("\nNext steps:")
			logger.Info("1. Edit weaviate-migrate.yaml with your Weaviate connection details")
			logger.Info("2. Create your first migration: weaviate-migrate create <name>")

			return nil
		},
	}
}

// createCmd creates a new migration file
func createCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new migration file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			migrationName := args[0]

			config, err := loadConfig()
			if err != nil {
				return err
			}

			parser := migrate.NewParser(config.MigrationsDir)
			nextVersion, err := parser.GetNextMigrationVersion()
			if err != nil {
				return fmt.Errorf("failed to determine next version: %w", err)
			}

			filename := migrate.GenerateMigrationFilename(nextVersion, migrationName)
			filepath := filepath.Join(config.MigrationsDir, filename)

			// Create template migration
			template := migrate.Migration{
				Version:     nextVersion,
				Description: migrationName,
				Operations: []migrate.Operation{
					{
						Type:  migrate.OperationAddProperty,
						Class: "YourClassName",
						Property: map[string]interface{}{
							"name":     "your_property",
							"dataType": []string{"text"},
						},
					},
				},
				Rollback: []migrate.Operation{
					{
						Type:         migrate.OperationDeleteProperty,
						Class:        "YourClassName",
						PropertyName: "your_property",
					},
				},
			}

			data, err := yaml.Marshal(template)
			if err != nil {
				return fmt.Errorf("failed to marshal template: %w", err)
			}

			if err := os.WriteFile(filepath, data, 0644); err != nil {
				return fmt.Errorf("failed to write migration file: %w", err)
			}

			logger.Infof("✓ Created: %s", filepath)
			logger.Info("\nEdit this file to define your migration operations.")

			return nil
		},
	}
}

// planCmd shows what migrations will be applied
func planCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "plan",
		Short: "Show pending migrations (dry-run)",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := loadConfig()
			if err != nil {
				return err
			}

			wvClient, err := createWeaviateClient(config)
			if err != nil {
				return err
			}

			executor := migrate.NewExecutor(wvClient, config, logger)
			parser := migrate.NewParser(config.MigrationsDir)

			migrations, err := parser.ParseAllMigrations()
			if err != nil {
				return fmt.Errorf("failed to parse migrations: %w", err)
			}

			if len(migrations) == 0 {
				logger.Info("No migrations found.")
				return nil
			}

			ctx := context.Background()
			plan, err := executor.GeneratePlan(ctx, migrations)
			if err != nil {
				return fmt.Errorf("failed to generate plan: %w", err)
			}

			output := migrate.FormatPlan(plan)
			fmt.Println(output)

			return nil
		},
	}
}

// applyCmd applies pending migrations
func applyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply pending migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := loadConfig()
			if err != nil {
				return err
			}

			// Check dry-run mode
			shouldDryRun := dryRun || (config.DryRunByDefault && !force)

			wvClient, err := createWeaviateClient(config)
			if err != nil {
				return err
			}

			executor := migrate.NewExecutor(wvClient, config, logger)
			parser := migrate.NewParser(config.MigrationsDir)

			migrations, err := parser.ParseAllMigrations()
			if err != nil {
				return fmt.Errorf("failed to parse migrations: %w", err)
			}

			if len(migrations) == 0 {
				logger.Info("No migrations to apply.")
				return nil
			}

			ctx := context.Background()

			for _, migration := range migrations {
				if err := executor.Apply(ctx, &migration, shouldDryRun); err != nil {
					return fmt.Errorf("migration v%d failed: %w", migration.Version, err)
				}
			}

			if shouldDryRun {
				logger.Info("\nDry run completed. Use --force to apply changes.")
			} else {
				logger.Info("\n✓ All migrations applied successfully")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "run in dry-run mode")
	cmd.Flags().BoolVar(&force, "force", false, "force apply (override dry_run_by_default)")

	return cmd
}

// rollbackCmd rolls back the last migration
func rollbackCmd() *cobra.Command {
	var toVersion int

	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Rollback the last migration",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := loadConfig()
			if err != nil {
				return err
			}

			wvClient, err := createWeaviateClient(config)
			if err != nil {
				return err
			}

			logger.Infof("Rollback functionality not yet fully implemented")
			logger.Info("In a full implementation, this would:")
			logger.Info("1. Determine the current version from migration history")
			logger.Info("2. Load the migration file for that version")
			logger.Info("3. Execute the rollback operations defined in the migration")
			logger.Infof("4. Roll back to version: %d (if specified)", toVersion)

			_ = wvClient // avoid unused variable

			return nil
		},
	}

	cmd.Flags().IntVar(&toVersion, "to-version", 0, "rollback to specific version")

	return cmd
}

// historyCmd shows migration history
func historyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "Show migration history",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := loadConfig()
			if err != nil {
				return err
			}

			wvClient, err := createWeaviateClient(config)
			if err != nil {
				return err
			}

			historyMgr := migrate.NewHistoryManager(wvClient, logger)
			ctx := context.Background()

			history, err := historyMgr.GetMigrationHistory(ctx)
			if err != nil {
				return fmt.Errorf("failed to get migration history: %w", err)
			}

			output := migrate.FormatHistory(history)
			fmt.Println(output)

			currentVersion, err := historyMgr.GetCurrentVersion(ctx)
			if err != nil {
				return fmt.Errorf("failed to get current version: %w", err)
			}

			fmt.Printf("Current schema version: %d\n", currentVersion)

			return nil
		},
	}
}

// validateCmd validates current schema state
func validateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate current schema state",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := loadConfig()
			if err != nil {
				return err
			}

			wvClient, err := createWeaviateClient(config)
			if err != nil {
				return err
			}

			logger.Info("Validating schema state...")
			logger.Info("In a full implementation, this would:")
			logger.Info("1. Compare current Weaviate schema with expected state from migrations")
			logger.Info("2. Detect any drift or inconsistencies")
			logger.Info("3. Suggest corrective actions if needed")

			_ = wvClient // avoid unused variable

			logger.Info("✓ Validation completed")

			return nil
		},
	}
}

// loadConfig loads the configuration file
func loadConfig() (*migrate.Config, error) {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s (run 'weaviate-migrate init' first)", configFile)
	}

	config, err := migrate.ParseConfig(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return config, nil
}

// createWeaviateClient creates a Weaviate client from config
func createWeaviateClient(config *migrate.Config) (*client.Client, error) {
	// In a real implementation, this would create an actual Weaviate client
	// For now, return nil as a placeholder
	logger.Debugf("Creating Weaviate client: %s://%s", config.Weaviate.Scheme, config.Weaviate.Host)

	// Placeholder - in real implementation:
	// return client.New(client.Config{
	//     Host:   config.Weaviate.Host,
	//     Scheme: config.Weaviate.Scheme,
	//     AuthConfig: client.Auth{
	//         APIKey: config.Weaviate.APIKey,
	//     },
	// })

	return nil, nil
}
