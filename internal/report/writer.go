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
	JSON                  string
	AutonomousDatabaseCSV string
	ComputeInstanceCSV    string
	FailedRequestsCSV     string
	Markdown              string
}

func Write(report model.Report, outputDir string) (Paths, error) {
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return Paths{}, fmt.Errorf("create output directory: %w", err)
	}

	stamp := report.GeneratedAt.UTC().Format("20060102T150405Z")
	base := "oci-tenancy-inventory-" + stamp
	paths := Paths{
		JSON:                  filepath.Join(outputDir, base+".json"),
		AutonomousDatabaseCSV: filepath.Join(outputDir, base+"-autonomous-databases.csv"),
		ComputeInstanceCSV:    filepath.Join(outputDir, base+"-compute-instances.csv"),
		FailedRequestsCSV:     filepath.Join(outputDir, base+"-failed-requests.csv"),
		Markdown:              filepath.Join(outputDir, base+".md"),
	}

	jsonData, err := marshalJSON(report)
	if err != nil {
		return Paths{}, err
	}
	databaseCSVData, err := marshalDatabaseCSV(report)
	if err != nil {
		return Paths{}, err
	}
	computeCSVData, err := marshalComputeCSV(report)
	if err != nil {
		return Paths{}, err
	}
	failedRequestsCSVData, err := marshalFailedRequestsCSV(report)
	if err != nil {
		return Paths{}, err
	}
	markdownData := marshalMarkdown(
		report,
		filepath.Base(paths.JSON),
		filepath.Base(paths.AutonomousDatabaseCSV),
		filepath.Base(paths.ComputeInstanceCSV),
		filepath.Base(paths.FailedRequestsCSV),
	)

	for _, item := range []struct {
		path string
		data []byte
	}{
		{paths.JSON, jsonData},
		{paths.AutonomousDatabaseCSV, databaseCSVData},
		{paths.ComputeInstanceCSV, computeCSVData},
		{paths.FailedRequestsCSV, failedRequestsCSVData},
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

func marshalDatabaseCSV(report model.Report) ([]byte, error) {
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
		"nb_created_since",
		"oracle_created_on_raw", "oracle_created_on_utc", "age_days_as_of_report",
		"oracle_created_by", "created_on_tag_status",
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
			int64Pointer(s.NBCreatedSince),
			s.OracleTags.CreatedOnRaw,
			s.OracleTags.CreatedOnUTC,
			int64Pointer(s.OracleTags.AgeDaysAsOfReport),
			s.OracleTags.CreatedBy,
			s.OracleTags.CreatedOnTagStatus,
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

func marshalFailedRequestsCSV(report model.Report) ([]byte, error) {
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	header := []string{
		"generated_at", "tenancy_ocid", "authentication", "stage", "region", "resource_ocid",
		"search_compartment_ocid", "search_display_name", "search_lifecycle_state", "search_time_created",
		"http_status_code", "service_code", "retryable", "target_service", "operation_name",
		"opc_request_id", "request_timestamp", "request_endpoint", "client_version", "diagnosis",
		"suggested_actions", "troubleshooting_link", "operation_reference_link", "message",
	}
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("write failed-requests CSV header: %w", err)
	}
	for _, item := range report.Errors {
		row := []string{
			report.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z"),
			report.TenancyOCID,
			report.Authentication,
			item.Stage,
			item.Region,
			item.ResourceID,
			item.SearchCompartmentID,
			item.SearchDisplayName,
			item.SearchLifecycleState,
			item.SearchTimeCreated,
			intValue(item.HTTPStatusCode),
			item.ServiceCode,
			boolPointer(item.Retryable),
			item.TargetService,
			item.OperationName,
			item.OPCRequestID,
			item.RequestTimestamp,
			item.RequestEndpoint,
			item.ClientVersion,
			item.Diagnosis,
			strings.Join(item.SuggestedActions, " | "),
			item.TroubleshootingLink,
			item.OperationReferenceLink,
			item.Message,
		}
		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("write failed-requests CSV row for %s: %w", item.ResourceID, err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush failed-requests CSV report: %w", err)
	}
	return output.Bytes(), nil
}

func marshalComputeCSV(report model.Report) ([]byte, error) {
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	header := []string{
		"generated_at", "tenancy_ocid", "region", "availability_domain", "compartment_ocid",
		"instance_ocid", "instance_display_name", "instance_lifecycle_state", "shape", "ocpus",
		"vcpus", "memory_gbs", "baseline_ocpu_utilization", "instance_oci_time_created",
		"instance_oracle_created_on_raw", "instance_oracle_created_on_utc",
		"instance_age_days_as_of_report", "instance_oracle_created_by",
		"instance_created_on_tag_status", "boot_volume_count", "attached_block_volume_count",
		"boot_volume_inventory_complete", "block_volume_inventory_complete",
		"boot_volume_total_size_gbs", "attached_block_volume_total_size_gbs",
		"attached_storage_total_size_gbs", "attached_storage_size_complete",
		"volume_kind", "volume_ocid", "volume_display_name", "volume_lifecycle_state",
		"attachment_ocid", "attachment_type", "attachment_lifecycle_state", "device",
		"volume_size_gbs", "volume_size_mbs", "volume_vpus_per_gb",
		"volume_auto_tuned_vpus_per_gb", "volume_oci_time_created",
		"volume_oracle_created_on_raw", "volume_oracle_created_on_utc",
		"volume_age_days_as_of_report", "volume_oracle_created_by",
		"volume_created_on_tag_status", "is_read_only", "is_shareable",
		"pv_encryption_in_transit_enabled",
	}
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("write Compute CSV header: %w", err)
	}

	for _, record := range report.ComputeInstances {
		volumeCount := len(record.BootVolumes) + len(record.BlockVolumes)
		if volumeCount == 0 {
			if err := writer.Write(computeCSVRow(report, record.Summary, nil)); err != nil {
				return nil, fmt.Errorf("write Compute CSV row for %s: %w", record.Summary.ID, err)
			}
			continue
		}
		for _, volume := range record.BootVolumes {
			summary := volume.Summary
			if err := writer.Write(computeCSVRow(report, record.Summary, &summary)); err != nil {
				return nil, fmt.Errorf("write Compute CSV boot-volume row for %s: %w", summary.ID, err)
			}
		}
		for _, volume := range record.BlockVolumes {
			summary := volume.Summary
			if err := writer.Write(computeCSVRow(report, record.Summary, &summary)); err != nil {
				return nil, fmt.Errorf("write Compute CSV block-volume row for %s: %w", summary.ID, err)
			}
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush Compute CSV report: %w", err)
	}
	return output.Bytes(), nil
}

func computeCSVRow(
	report model.Report,
	instance model.ComputeInstanceSummary,
	volume *model.VolumeSummary,
) []string {
	row := []string{
		report.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z"),
		report.TenancyOCID,
		instance.Region,
		instance.AvailabilityDomain,
		instance.CompartmentID,
		instance.ID,
		instance.DisplayName,
		instance.LifecycleState,
		instance.Shape,
		floatPointer(instance.OCPUs),
		intPointer(instance.VCPUs),
		floatPointer(instance.MemoryInGBs),
		instance.BaselineOCPUUtilization,
		instance.OCITimeCreated,
		instance.OracleTags.CreatedOnRaw,
		instance.OracleTags.CreatedOnUTC,
		int64Pointer(instance.OracleTags.AgeDaysAsOfReport),
		instance.OracleTags.CreatedBy,
		instance.OracleTags.CreatedOnTagStatus,
		strconv.Itoa(instance.BootVolumeCount),
		strconv.Itoa(instance.AttachedBlockVolumeCount),
		strconv.FormatBool(instance.BootVolumeInventoryComplete),
		strconv.FormatBool(instance.BlockVolumeInventoryComplete),
		int64Pointer(instance.BootVolumeTotalSizeInGBs),
		int64Pointer(instance.AttachedBlockVolumeTotalSizeInGBs),
		int64Pointer(instance.AttachedStorageTotalSizeInGBs),
		strconv.FormatBool(instance.AttachedStorageSizeComplete),
	}
	if volume == nil {
		return append(row, make([]string, 21)...)
	}
	return append(row,
		volume.Kind,
		volume.ID,
		volume.DisplayName,
		volume.LifecycleState,
		volume.AttachmentID,
		volume.AttachmentType,
		volume.AttachmentLifecycleState,
		volume.Device,
		int64Pointer(volume.SizeInGBs),
		int64Pointer(volume.SizeInMBs),
		int64Pointer(volume.VPUsPerGB),
		int64Pointer(volume.AutoTunedVPUsPerGB),
		volume.OCITimeCreated,
		volume.OracleTags.CreatedOnRaw,
		volume.OracleTags.CreatedOnUTC,
		int64Pointer(volume.OracleTags.AgeDaysAsOfReport),
		volume.OracleTags.CreatedBy,
		volume.OracleTags.CreatedOnTagStatus,
		boolPointer(volume.IsReadOnly),
		boolPointer(volume.IsShareable),
		boolPointer(volume.IsPvEncryptionInTransitEnabled),
	)
}

type regionTotal struct {
	Databases              int
	ECPUs                  float64
	OCPUs                  float64
	DatabaseStorageGB      float64
	ComputeInstances       int
	BootVolumes            int
	BlockVolumes           int
	ComputeStorageGB       int64
	ComputeStorageComplete bool
	Errors                 int
}

func marshalMarkdown(
	report model.Report,
	jsonName string,
	databaseCSVName string,
	computeCSVName string,
	failedRequestsCSVName string,
) []byte {
	totals := map[string]*regionTotal{}
	for _, region := range report.SubscribedRegions {
		if region.Scanned {
			totals[region.Name] = &regionTotal{ComputeStorageComplete: true}
		}
	}
	for _, record := range report.Databases {
		total := totals[record.Summary.Region]
		if total == nil {
			total = &regionTotal{ComputeStorageComplete: true}
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
			total.DatabaseStorageGB += *record.Summary.NormalizedDataStorageSizeInGBs
		}
	}
	for _, record := range report.ComputeInstances {
		total := totals[record.Summary.Region]
		if total == nil {
			total = &regionTotal{ComputeStorageComplete: true}
			totals[record.Summary.Region] = total
		}
		total.ComputeInstances++
		total.BootVolumes += record.Summary.BootVolumeCount
		total.BlockVolumes += record.Summary.AttachedBlockVolumeCount
		if record.Summary.AttachedStorageSizeComplete {
			total.ComputeStorageGB += *record.Summary.AttachedStorageTotalSizeInGBs
		} else {
			total.ComputeStorageComplete = false
		}
	}
	for _, item := range report.Errors {
		if item.Region != "" {
			total := totals[item.Region]
			if total == nil {
				total = &regionTotal{ComputeStorageComplete: true}
				totals[item.Region] = total
			}
			total.Errors++
		}
	}

	var output strings.Builder
	fmt.Fprintln(&output, "# OCI Autonomous Database and Compute inventory")
	fmt.Fprintln(&output)
	fmt.Fprintf(&output, "- Generated (UTC): `%s`\n", report.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(&output, "- Tenancy: `%s`\n", report.TenancyOCID)
	fmt.Fprintf(&output, "- Autonomous Database Search query: `%s`\n",
		report.SearchQueries["autonomous_databases"])
	fmt.Fprintf(&output, "- Compute Search query: `%s`\n",
		report.SearchQueries["compute_instances"])
	fmt.Fprintf(&output, "- Autonomous Databases retrieved: **%d**\n", report.DatabaseCount)
	fmt.Fprintf(&output, "- Compute instances retrieved: **%d**\n", report.ComputeInstanceCount)
	fmt.Fprintf(&output, "- Attached boot volumes retrieved: **%d**\n", report.BootVolumeCount)
	fmt.Fprintf(&output, "- Attached block volumes retrieved: **%d**\n", report.BlockVolumeCount)
	fmt.Fprintf(&output, "- Collection errors: **%d**\n", report.ErrorCount)
	fmt.Fprintf(&output, "- Full configuration: [%s](%s)\n", jsonName, jsonName)
	fmt.Fprintf(&output, "- Autonomous Database CSV: [%s](%s)\n", databaseCSVName, databaseCSVName)
	fmt.Fprintf(&output, "- Compute and attached-volume CSV: [%s](%s)\n", computeCSVName, computeCSVName)
	fmt.Fprintf(&output, "- Failed-request diagnostics CSV: [%s](%s)\n",
		failedRequestsCSVName, failedRequestsCSVName)
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "> The JSON file contains complete SDK objects returned by the Database, Compute,")
	fmt.Fprintln(&output, "> and Block Volume APIs and can contain sensitive tenancy, network, tag, ACL,")
	fmt.Fprintln(&output, "> instance metadata, and contact metadata.")
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "## Regional totals")
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "| Region | Databases | ECPUs | OCPUs | DB storage (GiB) | Instances | Boot volumes | Block volumes | Attached storage (GB) | Errors |")
	fmt.Fprintln(&output, "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|")
	regions := make([]string, 0, len(totals))
	for region := range totals {
		regions = append(regions, region)
	}
	sort.Strings(regions)
	for _, region := range regions {
		total := totals[region]
		computeStorage := ""
		if total.ComputeStorageComplete {
			computeStorage = strconv.FormatInt(total.ComputeStorageGB, 10)
		}
		fmt.Fprintf(&output, "| %s | %d | %s | %s | %s | %d | %d | %d | %s | %d |\n",
			markdownEscape(region),
			total.Databases,
			number(total.ECPUs),
			number(total.OCPUs),
			number(total.DatabaseStorageGB),
			total.ComputeInstances,
			total.BootVolumes,
			total.BlockVolumes,
			computeStorage,
			total.Errors,
		)
	}

	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "## Compute instances")
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "| Region | Instance | State | Shape | OCPUs | Memory (GB) | Oracle-Tags.CreatedOn | Age (days) | Created by | Boot (GB) | Block (GB) | Total (GB) |")
	fmt.Fprintln(&output, "|---|---|---|---|---:|---:|---|---:|---|---:|---:|---:|")
	for _, record := range report.ComputeInstances {
		s := record.Summary
		fmt.Fprintf(&output, "| %s | %s | %s | %s | %s | %s | `%s` | %s | %s | %s | %s | %s |\n",
			markdownEscape(s.Region),
			markdownEscape(s.DisplayName),
			markdownEscape(s.LifecycleState),
			markdownEscape(s.Shape),
			displayFloat(s.OCPUs),
			displayFloat(s.MemoryInGBs),
			markdownEscape(s.OracleTags.CreatedOnRaw),
			displayInt64(s.OracleTags.AgeDaysAsOfReport),
			markdownEscape(s.OracleTags.CreatedBy),
			displayInt64(s.BootVolumeTotalSizeInGBs),
			displayInt64(s.AttachedBlockVolumeTotalSizeInGBs),
			displayInt64(s.AttachedStorageTotalSizeInGBs),
		)
	}

	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "## Attached Compute storage")
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "| Region | Instance | Kind | Volume | Attachment | Device | Size (GB) | VPUs/GB | Oracle-Tags.CreatedOn | Age (days) | Created by |")
	fmt.Fprintln(&output, "|---|---|---|---|---|---|---:|---:|---|---:|---|")
	for _, record := range report.ComputeInstances {
		for _, volume := range record.BootVolumes {
			writeVolumeMarkdownRow(&output, record.Summary, volume.Summary)
		}
		for _, volume := range record.BlockVolumes {
			writeVolumeMarkdownRow(&output, record.Summary, volume.Summary)
		}
	}

	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "## Autonomous Databases")
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "| Region | Display name | DB name | State | Workload | Model | Compute | ECPUs | OCPUs | Storage | OCI time_created | nb_created_since | Oracle-Tags.CreatedOn | Tag age (days) | Created by |")
	fmt.Fprintln(&output, "|---|---|---|---|---|---|---:|---:|---:|---:|---|---:|---|---:|---|")
	for _, record := range report.Databases {
		s := record.Summary
		fmt.Fprintf(&output, "| %s | %s | `%s` | %s | %s | %s | %s | %s | %s | %s | `%s` | %s | `%s` | %s | %s |\n",
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
			markdownEscape(s.TimeCreated),
			displayInt64(s.NBCreatedSince),
			markdownEscape(s.OracleTags.CreatedOnRaw),
			displayInt64(s.OracleTags.AgeDaysAsOfReport),
			markdownEscape(s.OracleTags.CreatedBy),
		)
	}

	if len(report.Errors) > 0 {
		fmt.Fprintln(&output)
		fmt.Fprintln(&output, "## Collection errors")
		fmt.Fprintln(&output)
		fmt.Fprintln(&output, "The linked failed-request CSV contains the full SDK request metadata, diagnosis, suggested actions, and original error text.")
		fmt.Fprintln(&output)
		fmt.Fprintln(&output, "| Stage | Region | Resource | HTTP | Code | Retryable | OPC request ID | Diagnosis |")
		fmt.Fprintln(&output, "|---|---|---|---:|---|---|---|---|")
		for _, item := range report.Errors {
			fmt.Fprintf(&output, "| %s | %s | `%s` | %s | %s | %s | `%s` | %s |\n",
				markdownEscape(item.Stage),
				markdownEscape(item.Region),
				markdownEscape(item.ResourceID),
				intValue(item.HTTPStatusCode),
				markdownEscape(item.ServiceCode),
				boolPointer(item.Retryable),
				markdownEscape(item.OPCRequestID),
				markdownEscape(item.Diagnosis),
			)
		}
	}
	return []byte(output.String())
}

func writeVolumeMarkdownRow(
	output *strings.Builder,
	instance model.ComputeInstanceSummary,
	volume model.VolumeSummary,
) {
	fmt.Fprintf(output, "| %s | %s | %s | %s | %s | `%s` | %s | %s | `%s` | %s | %s |\n",
		markdownEscape(instance.Region),
		markdownEscape(instance.DisplayName),
		markdownEscape(volume.Kind),
		markdownEscape(volume.DisplayName),
		markdownEscape(volume.AttachmentType),
		markdownEscape(volume.Device),
		displayInt64(volume.SizeInGBs),
		displayInt64(volume.VPUsPerGB),
		markdownEscape(volume.OracleTags.CreatedOnRaw),
		displayInt64(volume.OracleTags.AgeDaysAsOfReport),
		markdownEscape(volume.OracleTags.CreatedBy),
	)
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

func intValue(value int) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func int64Pointer(value *int64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(*value, 10)
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

func displayInt64(value *int64) string {
	return int64Pointer(value)
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
