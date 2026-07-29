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
	}
	report := model.Report{
		SchemaVersion:  "1.0",
		GeneratedAt:    generated,
		TenancyOCID:    "ocid1.tenancy.oc1..example",
		Authentication: "api_key",
		SearchQuery:    model.SearchQuery,
		SubscribedRegions: []model.Region{
			{Name: "eu-paris-1", Status: "READY", Scanned: true},
		},
		Databases: []model.DatabaseRecord{{
			Summary:       model.NewSummary("eu-paris-1", adb),
			Configuration: adb,
		}},
	}
	report.Finalize()

	paths, err := Write(report, t.TempDir())
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	for _, path := range []string{paths.JSON, paths.CSV, paths.Markdown} {
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

	csvFile, err := os.Open(paths.CSV)
	if err != nil {
		t.Fatal(err)
	}
	defer csvFile.Close()
	rows, err := csv.NewReader(csvFile).ReadAll()
	if err != nil {
		t.Fatalf("invalid CSV: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("CSV rows = %d, want 2", len(rows))
	}
}
