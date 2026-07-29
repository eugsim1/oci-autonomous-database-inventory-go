package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	AuthAPIKey            = "api_key"
	AuthInstancePrincipal = "instance_principal"
	AuthResourcePrincipal = "resource_principal"
)

type Config struct {
	AuthMode        string
	ConfigFile      string
	Profile         string
	TenancyOCID     string
	BootstrapRegion string
	Regions         []string
	OutputDir       string
	Workers         int
	Timeout         time.Duration
	Strict          bool
	ShowVersion     bool
}

func Parse(args []string, output io.Writer) (Config, error) {
	workers := runtime.NumCPU()
	if workers < 4 {
		workers = 4
	}
	if workers > 16 {
		workers = 16
	}

	var cfg Config
	var regions string
	fs := flag.NewFlagSet("oci-adb-inventory", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.StringVar(&cfg.AuthMode, "auth", AuthAPIKey, "authentication: api_key, instance_principal, or resource_principal")
	fs.StringVar(&cfg.ConfigFile, "config-file", "", "OCI SDK config path; empty uses ~/.oci/config or OCI_CONFIG_FILE")
	fs.StringVar(&cfg.Profile, "profile", "DEFAULT", "OCI SDK config profile for api_key authentication")
	fs.StringVar(&cfg.TenancyOCID, "tenancy-id", "", "tenancy OCID; empty resolves it from the authentication provider")
	fs.StringVar(&cfg.BootstrapRegion, "bootstrap-region", "", "region for the initial Identity call; empty uses the provider region")
	fs.StringVar(&regions, "regions", "", "comma-separated READY subscribed regions; empty scans all")
	fs.StringVar(&cfg.OutputDir, "output-dir", "reports", "directory for timestamped JSON, CSV, and Markdown reports")
	fs.IntVar(&cfg.Workers, "workers", workers, "maximum concurrent OCI API operations")
	fs.DurationVar(&cfg.Timeout, "timeout", 30*time.Minute, "overall collection timeout")
	fs.BoolVar(&cfg.Strict, "strict", false, "return a non-zero status when any region or database lookup fails")
	fs.BoolVar(&cfg.ShowVersion, "version", false, "print version and exit")
	fs.Usage = func() {
		fmt.Fprintln(output, "Usage: oci-adb-inventory [options]")
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Discovers Autonomous Databases with OCI Search in every READY subscribed")
		fmt.Fprintln(output, "region, enriches each result with GetAutonomousDatabase, and writes reports.")
		fmt.Fprintln(output)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if fs.NArg() != 0 {
		return Config{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}

	cfg.AuthMode = strings.ToLower(strings.TrimSpace(cfg.AuthMode))
	cfg.Profile = strings.TrimSpace(cfg.Profile)
	cfg.TenancyOCID = strings.TrimSpace(cfg.TenancyOCID)
	cfg.BootstrapRegion = strings.ToLower(strings.TrimSpace(cfg.BootstrapRegion))
	cfg.OutputDir = strings.TrimSpace(cfg.OutputDir)
	cfg.Regions = parseRegions(regions)

	if cfg.ShowVersion {
		return cfg, nil
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	switch c.AuthMode {
	case AuthAPIKey, AuthInstancePrincipal, AuthResourcePrincipal:
	default:
		return fmt.Errorf("unsupported --auth %q", c.AuthMode)
	}
	if c.Profile == "" {
		return errors.New("--profile cannot be empty")
	}
	if c.OutputDir == "" {
		return errors.New("--output-dir cannot be empty")
	}
	if c.Workers < 1 || c.Workers > 128 {
		return errors.New("--workers must be between 1 and 128")
	}
	if c.Timeout <= 0 {
		return errors.New("--timeout must be greater than zero")
	}
	if c.TenancyOCID != "" && !strings.HasPrefix(c.TenancyOCID, "ocid1.tenancy.") {
		return errors.New("--tenancy-id must be a tenancy OCID")
	}
	return nil
}

func parseRegions(value string) []string {
	unique := map[string]struct{}{}
	for _, item := range strings.Split(value, ",") {
		region := strings.ToLower(strings.TrimSpace(item))
		if region != "" {
			unique[region] = struct{}{}
		}
	}
	result := make([]string, 0, len(unique))
	for region := range unique {
		result = append(result, region)
	}
	sort.Strings(result)
	return result
}
