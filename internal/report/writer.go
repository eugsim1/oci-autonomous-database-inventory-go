package report

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/eugsim1/oci-autonomous-database-inventory-go/internal/model"
)

type Paths struct {
	JSON     string
	CSV      string
	Markdown string
}

func Write(report model.Report, outputDir string) (Paths, error) {
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return Paths{}, fmt.Errorf("create output directory: %w", err)
	}

	stamp := report.GeneratedAt.UTC().Format("20060102T150405Z")
	base := "oci-autonomous-database-inventory-" + stamp
	paths := Paths{
		JSON:     filepath.Join(outputDir, base+".json"),
		CSV:      filepath.Join(outputDir, base+".csv"),
		Markdown: filepath.Join(outputDir, base+".md"),
	}

	jsonData, err := marshalJSON(report)
	if err != nil {
		return Paths{}, err
	}
	csvData, err := marshalCSV(report)
	if err != nil {
		return Paths{}, err
	}
	markdownData := marshalMarkdown(report, filepath.Base(paths.JSON), filepath.Base(paths.CSV))

	for _, item := range []struct {
		path string
		data []byte
	}{
		{paths.JSON, jsonData},
		{paths.CSV, csvData},
		{paths.Markdown, markdownData},
	} {
		if err := writeFileAtomically(item.path, item.data); err != nil {
			return Paths{}, err
		}
	}
	return paths, nil
}

func marshalJSON(report model.Report) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return nil, fmt.Errorf("encode JSON report: %w", err)
	}
	return output.Bytes(), nil
}

func marshalCSV(report model.Report) ([]byte, error) {
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	header := []string{
		"generated_at", "tenancy_ocid", "region", "compartment_ocid", "autonomous_database_ocid",
		"display_name", "db_name", "workload", "lifecycle_state", "lifecycle_details", "db_version",
		"infrastructure_type", "is_dedicated", "is_free_tier", "compute_model", "compute_count",
		"ecpus", "ocpus", "legacy_cpu_core_count", "memory_per_compute_unit_gbs",
		"compute_auto_scaling", "data_storage_size_gbs", "data_storage_size_tbs",
		"normalized_data_storage_size_gbs", "used_data_storage_size_gbs",
		"used_data_storage_size_tbs", "allocated_storage_size_tbs",
		"actual_used_storage_size_tbs", "storage_auto_scaling", "license_model",
		"database_edition", "role", "open_mode", "permission_level", "subnet_ocid",
		"private_endpoint", "public_endpoint", "access_control_enabled", "mtls_required",
		"local_data_guard_enabled", "remote_data_guard_enabled", "time_created",
	}
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("write CSV header: %w", err)
	}

	for _, record := range report.Databases {
		s := record.Summary
		row := []string{
			report.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z"),
			report.TenancyOCID,
			s.Region,
			s.CompartmentID,
			s.ID,
			s.DisplayName,
			s.DBName,
			s.Workload,
			s.LifecycleState,
			s.LifecycleDetails,
			s.DBVersion,
			s.InfrastructureType,
			boolPointer(s.IsDedicated),
			boolPointer(s.IsFreeTier),
			s.ComputeModel,
			floatPointer(s.ComputeCount),
			floatPointer(s.ECPUs),
			floatPointer(s.OCPUs),
			intPointer(s.LegacyCPUCoreCount),
			floatPointer(s.MemoryPerComputeUnitInGBs),
			boolPointer(s.IsComputeAutoScalingEnabled),
			intPointer(s.DataStorageSizeInGBs),
			intPointer(s.DataStorageSizeInTBs),
			floatPointer(s.NormalizedDataStorageSizeInGBs),
			intPointer(s.UsedDataStorageSizeInGBs),
			intPointer(s.UsedDataStorageSizeInTBs),
			floatPointer(s.AllocatedStorageSizeInTBs),
			floatPointer(s.ActualUsedStorageSizeInTBs),
			boolPointer(s.IsStorageAutoScalingEnabled),
			s.LicenseModel,
			s.DatabaseEdition,
			s.Role,
			s.OpenMode,
			s.PermissionLevel,
			s.SubnetID,
			s.PrivateEndpoint,
			s.PublicEndpoint,
			boolPointer(s.IsAccessControlEnabled),
			boolPointer(s.IsMTLSConnectionRequired),
			boolPointer(s.IsLocalDataGuardEnabled),
			boolPointer(s.IsRemoteDataGuardEnabled),
			s.TimeCreated,
		}
		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("write CSV row for %s: %w", s.ID, err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush CSV report: %w", err)
	}
	return output.Bytes(), nil
}

type regionTotal struct {
	Databases int
	ECPUs     float64
	OCPUs     float64
	StorageGB float64
	Errors    int
}

func marshalMarkdown(report model.Report, jsonName, csvName string) []byte {
	totals := map[string]*regionTotal{}
	for _, region := range report.SubscribedRegions {
		if region.Scanned {
			totals[region.Name] = &regionTotal{}
		}
	}
	for _, record := range report.Databases {
		total := totals[record.Summary.Region]
		if total == nil {
			total = &regionTotal{}
			totals[record.Summary.Region] = total
		}
		total.Databases++
		if record.Summary.ECPUs != nil {
			total.ECPUs += *record.Summary.ECPUs
		}
		if record.Summary.OCPUs != nil {
			total.OCPUs += *record.Summary.OCPUs
		}
		if record.Summary.NormalizedDataStorageSizeInGBs != nil {
			total.StorageGB += *record.Summary.NormalizedDataStorageSizeInGBs
		}
	}
	for _, item := range report.Errors {
		if item.Region != "" {
			total := totals[item.Region]
			if total == nil {
				total = &regionTotal{}
				totals[item.Region] = total
			}
			total.Errors++
		}
	}

	var output strings.Builder
	fmt.Fprintln(&output, "# OCI Autonomous Database inventory")
	fmt.Fprintln(&output)
	fmt.Fprintf(&output, "- Generated (UTC): `%s`\n", report.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(&output, "- Tenancy: `%s`\n", report.TenancyOCID)
	fmt.Fprintf(&output, "- Search query: `%s`\n", report.SearchQuery)
	fmt.Fprintf(&output, "- Autonomous Databases retrieved: **%d**\n", report.DatabaseCount)
	fmt.Fprintf(&output, "- Collection errors: **%d**\n", report.ErrorCount)
	fmt.Fprintf(&output, "- Full configuration: [%s](%s)\n", jsonName, jsonName)
	fmt.Fprintf(&output, "- Flat summary: [%s](%s)\n", csvName, csvName)
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "> The JSON file contains the complete `GetAutonomousDatabase` SDK object for each")
	fmt.Fprintln(&output, "> resource and can contain sensitive tenancy, network, tag, ACL, and contact metadata.")
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "## Regional totals")
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "| Region | Databases | Base ECPUs | Base OCPUs | Configured storage (GiB) | Errors |")
	fmt.Fprintln(&output, "|---|---:|---:|---:|---:|---:|")
	regions := make([]string, 0, len(totals))
	for region := range totals {
		regions = append(regions, region)
	}
	sort.Strings(regions)
	for _, region := range regions {
		total := totals[region]
		fmt.Fprintf(&output, "| %s | %d | %s | %s | %s | %d |\n",
			markdownEscape(region),
			total.Databases,
			number(total.ECPUs),
			number(total.OCPUs),
			number(total.StorageGB),
			total.Errors,
		)
	}

	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "## Autonomous Databases")
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "| Region | Display name | DB name | State | Workload | Model | Compute | ECPUs | OCPUs | Storage |")
	fmt.Fprintln(&output, "|---|---|---|---|---|---|---:|---:|---:|---:|")
	for _, record := range report.Databases {
		s := record.Summary
		fmt.Fprintf(&output, "| %s | %s | `%s` | %s | %s | %s | %s | %s | %s | %s |\n",
			markdownEscape(s.Region),
			markdownEscape(s.DisplayName),
			markdownEscape(s.DBName),
			markdownEscape(s.LifecycleState),
			markdownEscape(s.Workload),
			markdownEscape(s.ComputeModel),
			displayFloat(s.ComputeCount),
			displayFloat(s.ECPUs),
			displayFloat(s.OCPUs),
			displayStorage(s),
		)
	}

	if len(report.Errors) > 0 {
		fmt.Fprintln(&output)
		fmt.Fprintln(&output, "## Collection errors")
		fmt.Fprintln(&output)
		fmt.Fprintln(&output, "| Stage | Region | Resource | Error |")
		fmt.Fprintln(&output, "|---|---|---|---|")
		for _, item := range report.Errors {
			fmt.Fprintf(&output, "| %s | %s | `%s` | %s |\n",
				markdownEscape(item.Stage),
				markdownEscape(item.Region),
				markdownEscape(item.ResourceID),
				markdownEscape(item.Message),
			)
		}
	}
	return []byte(output.String())
}

func displayStorage(summary model.DatabaseSummary) string {
	if summary.DataStorageSizeInGBs != nil {
		return fmt.Sprintf("%d GiB", *summary.DataStorageSizeInGBs)
	}
	if summary.DataStorageSizeInTBs != nil {
		return fmt.Sprintf("%d TiB", *summary.DataStorageSizeInTBs)
	}
	return ""
}

func writeFileAtomically(path string, data []byte) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".report-*")
	if err != nil {
		return fmt.Errorf("create temporary report for %s: %w", path, err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o640); err != nil {
		temp.Close()
		return fmt.Errorf("set permissions on temporary report: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary report: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary report: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("publish report %s: %w", path, err)
	}
	return nil
}

func boolPointer(value *bool) string {
	if value == nil {
		return ""
	}
	return strconv.FormatBool(*value)
}

func intPointer(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

func floatPointer(value *float64) string {
	if value == nil {
		return ""
	}
	return number(*value)
}

func displayFloat(value *float64) string {
	if value == nil {
		return ""
	}
	return number(*value)
}

func number(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func markdownEscape(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}
