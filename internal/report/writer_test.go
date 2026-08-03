package report

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugsim1/oci-autonomous-database-inventory-go/internal/model"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/database"
)

func TestWriteProducesTimestampedReports(t *testing.T) {
	generated := time.Date(2026, 7, 29, 10, 11, 12, 0, time.UTC)
	adb := database.AutonomousDatabase{
		Id:                   common.String("ocid1.autonomousdatabase.oc1.eu-paris-1.example"),
		CompartmentId:        common.String("ocid1.compartment.oc1..example"),
		DbName:               common.String("SALES"),
		DisplayName:          common.String("Sales"),
		LifecycleState:       database.AutonomousDatabaseLifecycleStateAvailable,
		ComputeModel:         database.AutonomousDatabaseComputeModelEcpu,
		ComputeCount:         common.Float32(8),
		DataStorageSizeInGBs: common.Int(1024),
		TimeCreated: &common.SDKTime{Time: time.Date(
			2026, 7, 1, 10, 11, 12, 0, time.UTC,
		)},
		DefinedTags: map[string]map[string]interface{}{
			"Oracle-Tags": {
				"CreatedOn": "2026-07-09T10:11:12Z",
				"CreatedBy": "database-admin@example.com",
			},
		},
	}
	report := model.Report{
		SchemaVersion:  "2.1",
		GeneratedAt:    generated,
		TenancyOCID:    "ocid1.tenancy.oc1..example",
		Authentication: "api_key",
		SearchQueries: map[string]string{
			"autonomous_databases": model.AutonomousDatabaseSearchQuery,
			"compute_instances":    model.ComputeInstanceSearchQuery,
		},
		SubscribedRegions: []model.Region{
			{Name: "eu-paris-1", Status: "READY", Scanned: true},
		},
		Databases: []model.DatabaseRecord{{
			Summary:       model.NewSummaryAt("eu-paris-1", adb, generated),
			Configuration: adb,
		}},
	}
	instance := core.Instance{
		Id:                 common.String("ocid1.instance.oc1.eu-paris-1.example"),
		CompartmentId:      common.String("ocid1.compartment.oc1..example"),
		DisplayName:        common.String("app-01"),
		AvailabilityDomain: common.String("PARIS-AD-1"),
		Shape:              common.String("VM.Standard.E5.Flex"),
		LifecycleState:     core.InstanceLifecycleStateRunning,
		DefinedTags: map[string]map[string]interface{}{
			"Oracle-Tags": {
				"CreatedOn": "2026-07-19T10:11:12Z",
				"CreatedBy": "alice@example.com",
			},
		},
	}
	bootAttachment := core.BootVolumeAttachment{
		Id:             common.String("ocid1.bootvolumeattachment.oc1.example"),
		BootVolumeId:   common.String("ocid1.bootvolume.oc1.example"),
		LifecycleState: core.BootVolumeAttachmentLifecycleStateAttached,
	}
	boot := model.NewBootVolumeRecord(bootAttachment, core.BootVolume{
		Id:          common.String("ocid1.bootvolume.oc1.example"),
		DisplayName: common.String("app-01 (Boot Volume)"),
		SizeInGBs:   common.Int64(100),
	}, generated)
	blockAttachment := core.ParavirtualizedVolumeAttachment{
		Id:             common.String("ocid1.volumeattachment.oc1.example"),
		VolumeId:       common.String("ocid1.volume.oc1.example"),
		LifecycleState: core.VolumeAttachmentLifecycleStateAttached,
	}
	block := model.NewBlockVolumeRecord(blockAttachment, core.Volume{
		Id:          common.String("ocid1.volume.oc1.example"),
		DisplayName: common.String("app-data"),
		SizeInGBs:   common.Int64(250),
	}, generated)
	report.ComputeInstances = []model.ComputeInstanceRecord{
		model.NewComputeInstanceRecord(
			"eu-paris-1",
			instance,
			[]model.BootVolumeRecord{boot},
			[]model.BlockVolumeRecord{block},
			true,
			true,
			generated,
		),
	}
	retryable := false
	report.Errors = []model.CollectionError{{
		Stage:          "get_autonomous_database",
		Region:         "eu-paris-1",
		ResourceID:     "ocid1.autonomousdatabase.oc1.eu-paris-1.missing",
		HTTPStatusCode: 404,
		ServiceCode:    "NotAuthorizedOrNotFound",
		OPCRequestID:   "example-opc-request-id",
		Retryable:      &retryable,
		Diagnosis:      "The resource is missing or inaccessible.",
		Message:        "Authorization failed or requested resource not found.",
	}}
	report.Finalize()

	paths, err := Write(report, t.TempDir())
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	for _, path := range []string{
		paths.JSON,
		paths.AutonomousDatabaseCSV,
		paths.ComputeInstanceCSV,
		paths.FailedRequestsCSV,
		paths.Markdown,
	} {
		if !strings.Contains(filepath.Base(path), "20260729T101112Z") {
			t.Fatalf("path %q does not contain the UTC timestamp", path)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("report %q: %v", path, err)
		}
	}

	jsonData, err := os.ReadFile(paths.JSON)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	databases := decoded["databases"].([]interface{})
	record := databases[0].(map[string]interface{})
	configuration := record["configuration"].(map[string]interface{})
	if configuration["computeModel"] != "ECPU" {
		t.Fatalf("full configuration computeModel = %v", configuration["computeModel"])
	}
	instances := decoded["compute_instances"].([]interface{})
	computeRecord := instances[0].(map[string]interface{})
	computeConfiguration := computeRecord["configuration"].(map[string]interface{})
	if computeConfiguration["shape"] != "VM.Standard.E5.Flex" {
		t.Fatalf("full Compute configuration shape = %v", computeConfiguration["shape"])
	}

	csvFile, err := os.Open(paths.AutonomousDatabaseCSV)
	if err != nil {
		t.Fatal(err)
	}
	defer csvFile.Close()
	rows, err := csv.NewReader(csvFile).ReadAll()
	if err != nil {
		t.Fatalf("invalid CSV: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("Autonomous Database CSV rows = %d, want 2", len(rows))
	}
	if len(rows[1]) != len(rows[0]) {
		t.Fatalf("Autonomous Database CSV row has %d columns, header has %d",
			len(rows[1]), len(rows[0]))
	}
	if !strings.Contains(strings.Join(rows[1], ","), "database-admin@example.com") {
		t.Fatalf("Autonomous Database CSV does not contain Oracle-Tags.CreatedBy: %#v", rows[1])
	}
	nbCreatedSinceIndex := columnIndex(rows[0], "nb_created_since")
	if nbCreatedSinceIndex == -1 {
		t.Fatalf("Autonomous Database CSV is missing nb_created_since: %#v", rows[0])
	}
	if rows[1][nbCreatedSinceIndex] != "28" {
		t.Fatalf("nb_created_since = %q, want 28", rows[1][nbCreatedSinceIndex])
	}

	computeCSVFile, err := os.Open(paths.ComputeInstanceCSV)
	if err != nil {
		t.Fatal(err)
	}
	defer computeCSVFile.Close()
	computeRows, err := csv.NewReader(computeCSVFile).ReadAll()
	if err != nil {
		t.Fatalf("invalid Compute CSV: %v", err)
	}
	if len(computeRows) != 3 {
		t.Fatalf("Compute CSV rows = %d, want 3", len(computeRows))
	}
	for index, row := range computeRows[1:] {
		if len(row) != len(computeRows[0]) {
			t.Fatalf("Compute CSV row %d has %d columns, header has %d",
				index+1, len(row), len(computeRows[0]))
		}
	}
	if !strings.Contains(strings.Join(computeRows[1], ","), "100") ||
		!strings.Contains(strings.Join(computeRows[2], ","), "250") {
		t.Fatalf("Compute CSV does not contain exact boot and block volume sizes: %#v", computeRows)
	}

	failedCSVFile, err := os.Open(paths.FailedRequestsCSV)
	if err != nil {
		t.Fatal(err)
	}
	defer failedCSVFile.Close()
	failedRows, err := csv.NewReader(failedCSVFile).ReadAll()
	if err != nil {
		t.Fatalf("invalid failed-requests CSV: %v", err)
	}
	if len(failedRows) != 2 || !strings.Contains(strings.Join(failedRows[1], ","), "example-opc-request-id") {
		t.Fatalf("failed-requests CSV does not contain the structured failure: %#v", failedRows)
	}

	markdown, err := os.ReadFile(paths.Markdown)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markdown), "nb_created_since") ||
		!strings.Contains(string(markdown), filepath.Base(paths.FailedRequestsCSV)) {
		t.Fatalf("Markdown does not document age and failed-request outputs: %s", markdown)
	}
}

func columnIndex(header []string, name string) int {
	for index, value := range header {
		if value == name {
			return index
		}
	}
	return -1
}
