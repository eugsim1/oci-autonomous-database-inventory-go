package model

import (
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/database"
)

func TestNewSummaryECPUAndGBStorage(t *testing.T) {
	adb := database.AutonomousDatabase{
		Id:                   common.String("ocid1.autonomousdatabase.oc1.eu-paris-1.example"),
		CompartmentId:        common.String("ocid1.compartment.oc1..example"),
		DbName:               common.String("SALES"),
		DisplayName:          common.String("sales"),
		ComputeModel:         database.AutonomousDatabaseComputeModelEcpu,
		ComputeCount:         common.Float32(6),
		DataStorageSizeInGBs: common.Int(512),
	}
	got := NewSummary("eu-paris-1", adb)
	if got.ECPUs == nil || *got.ECPUs != 6 {
		t.Fatalf("ECPUs = %v, want 6", got.ECPUs)
	}
	if got.OCPUs != nil {
		t.Fatalf("OCPUs = %v, want nil", got.OCPUs)
	}
	if got.NormalizedDataStorageSizeInGBs == nil || *got.NormalizedDataStorageSizeInGBs != 512 {
		t.Fatalf("NormalizedDataStorageSizeInGBs = %v, want 512", got.NormalizedDataStorageSizeInGBs)
	}
}

func TestNewSummaryOCPUAndTBStorage(t *testing.T) {
	adb := database.AutonomousDatabase{
		Id:                   common.String("ocid1.autonomousdatabase.oc1.us-ashburn-1.example"),
		CompartmentId:        common.String("ocid1.compartment.oc1..example"),
		DbName:               common.String("DW"),
		ComputeModel:         database.AutonomousDatabaseComputeModelOcpu,
		ComputeCount:         common.Float32(4),
		DataStorageSizeInTBs: common.Int(3),
	}
	got := NewSummary("us-ashburn-1", adb)
	if got.OCPUs == nil || *got.OCPUs != 4 {
		t.Fatalf("OCPUs = %v, want 4", got.OCPUs)
	}
	if got.NormalizedDataStorageSizeInGBs == nil || *got.NormalizedDataStorageSizeInGBs != 3072 {
		t.Fatalf("NormalizedDataStorageSizeInGBs = %v, want 3072", got.NormalizedDataStorageSizeInGBs)
	}
}
