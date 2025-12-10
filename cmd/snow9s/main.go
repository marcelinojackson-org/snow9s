package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourusername/snow9s/internal/config"
	"github.com/yourusername/snow9s/internal/snowflake"
	"github.com/yourusername/snow9s/internal/ui"
)

var cfgOverrides config.Config

func main() {
	rootCmd := buildRootCmd()
	ctx := context.Background()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func buildRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "snow9s",
		Short: "k9s-style TUI for Snowflake Snowpark Container Services",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI(cmd.Context())
		},
	}

	flags := rootCmd.PersistentFlags()
	flags.StringVar(&cfgOverrides.Account, "account", "", "Snowflake account (or SNOWFLAKE_ACCOUNT)")
	flags.StringVar(&cfgOverrides.User, "user", "", "Snowflake user")
	flags.StringVar(&cfgOverrides.Password, "password", "", "Snowflake password")
	flags.StringVar(&cfgOverrides.Database, "database", "", "Database name")
	flags.StringVar(&cfgOverrides.Schema, "schema", "", "Schema (namespace)")
	flags.StringVar(&cfgOverrides.Warehouse, "warehouse", "", "Warehouse name")
	flags.StringVar(&cfgOverrides.Context, "context", "", "Config context name")
	flags.BoolVar(&cfgOverrides.Debug, "debug", false, "Enable debug Snowflake logging")

	listCmd := &cobra.Command{Use: "list", Short: "List resources"}
	servicesCmd := &cobra.Command{Use: "services", Short: "List Snowpark services", RunE: runListServices}
	listCmd.AddCommand(servicesCmd)

	rootCmd.AddCommand(listCmd)
	return rootCmd
}

func runTUI(ctx context.Context) error {
	cfg, logger, err := loadConfigAndLogger()
	if err != nil {
		return err
	}

	client, err := snowflake.NewClient(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer client.Close()

	spcs := snowflake.NewSPCS(client, cfg)
	uiApp := ui.NewApp(cfg, spcs, cfg.Debug)
	if cfg.Debug {
		if w := uiApp.DebugWriter(); w != nil {
			logger.SetOutput(io.MultiWriter(os.Stdout, w))
		}
	}

	return uiApp.Run(ctx)
}

func runListServices(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
	defer cancel()

	cfg, logger, err := loadConfigAndLogger()
	if err != nil {
		return err
	}

	client, err := snowflake.NewClient(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer client.Close()

	spcs := snowflake.NewSPCS(client, cfg)
	services, err := spcs.ListServices(ctx)
	if err != nil {
		return err
	}
	ui.PrintTable(services)
	return nil
}

func loadConfigAndLogger() (config.Config, *log.Logger, error) {
	cfgFile, err := config.LoadConfig(cfgOverrides.Context)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return config.Config{}, nil, err
	}
	cfg := config.MergeOverrides(cfgFile, cfgOverrides)
	if err := cfg.Validate(); err != nil {
		return config.Config{}, nil, err
	}

	logger := log.New(os.Stdout, "snow9s ", log.LstdFlags)
	if !cfg.Debug {
		logger.SetOutput(io.Discard)
	}
	return cfg, logger, nil
}
