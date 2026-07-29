package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/eugsim1/oci-autonomous-database-inventory-go/internal/config"
	"github.com/eugsim1/oci-autonomous-database-inventory-go/internal/oci"
	"github.com/eugsim1/oci-autonomous-database-inventory-go/internal/report"
)

var version = "1.0.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	cfg, err := config.Parse(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}
	if cfg.ShowVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}

	collector, err := oci.NewCollector(cfg, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	inventory, err := collector.Collect(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	paths, err := report.Write(inventory, cfg.OutputDir)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Autonomous Databases: %d\n", inventory.DatabaseCount)
	fmt.Fprintf(stdout, "Collection errors: %d\n", inventory.ErrorCount)
	fmt.Fprintf(stdout, "JSON: %s\n", paths.JSON)
	fmt.Fprintf(stdout, "CSV: %s\n", paths.CSV)
	fmt.Fprintf(stdout, "Markdown: %s\n", paths.Markdown)

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		fmt.Fprintf(stderr, "error: collection exceeded the %s timeout\n", cfg.Timeout)
		return 1
	}
	if cfg.Strict && inventory.ErrorCount > 0 {
		fmt.Fprintln(stderr, "error: strict mode detected collection errors; partial reports were written")
		return 1
	}
	return 0
}
