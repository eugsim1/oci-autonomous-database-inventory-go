package model

import (
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
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

func TestNewSummaryCalculatesDaysSinceOCITimeCreated(t *testing.T) {
	created := time.Date(2026, 7, 1, 10, 11, 12, 0, time.UTC)
	asOf := time.Date(2026, 7, 29, 22, 11, 11, 0, time.UTC)
	adb := database.AutonomousDatabase{
		Id:          common.String("ocid1.autonomousdatabase.oc1.eu-paris-1.example"),
		TimeCreated: &common.SDKTime{Time: created},
	}

	got := NewSummaryAt("eu-paris-1", adb, asOf)
	if got.TimeCreated != "2026-07-01T10:11:12Z" {
		t.Fatalf("TimeCreated = %q", got.TimeCreated)
	}
	if got.NBCreatedSince == nil || *got.NBCreatedSince != 28 {
		t.Fatalf("NBCreatedSince = %v, want 28", got.NBCreatedSince)
	}
}

func TestOracleTagAuditUsesCreatedOnAndCreatedBy(t *testing.T) {
	asOf := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	tags := map[string]map[string]interface{}{
		"Oracle-Tags": {
			"CreatedOn": "2026-07-19T11:00:00Z",
			"CreatedBy": "alice@example.com",
		},
	}
	got := NewOracleTagAudit(tags, asOf)
	if got.CreatedOnTagStatus != "parsed" {
		t.Fatalf("CreatedOnTagStatus = %q, want parsed", got.CreatedOnTagStatus)
	}
	if got.CreatedOnUTC != "2026-07-19T11:00:00Z" {
		t.Fatalf("CreatedOnUTC = %q", got.CreatedOnUTC)
	}
	if got.AgeDaysAsOfReport == nil || *got.AgeDaysAsOfReport != 10 {
		t.Fatalf("AgeDaysAsOfReport = %v, want 10", got.AgeDaysAsOfReport)
	}
	if got.CreatedBy != "alice@example.com" {
		t.Fatalf("CreatedBy = %q", got.CreatedBy)
	}
}

func TestOracleTagAuditReportsInvalidCreatedOn(t *testing.T) {
	got := NewOracleTagAudit(map[string]map[string]interface{}{
		"oracle-tags": {
			"createdon": "not-a-date",
		},
	}, time.Now())
	if got.CreatedOnTagStatus != "invalid" {
		t.Fatalf("CreatedOnTagStatus = %q, want invalid", got.CreatedOnTagStatus)
	}
	if got.AgeDaysAsOfReport != nil {
		t.Fatalf("AgeDaysAsOfReport = %v, want nil", got.AgeDaysAsOfReport)
	}
}

func TestComputeInstanceSummaryTotalsAttachedStorage(t *testing.T) {
	asOf := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	instance := core.Instance{
		Id:                 common.String("ocid1.instance.oc1.eu-paris-1.example"),
		CompartmentId:      common.String("ocid1.compartment.oc1..example"),
		DisplayName:        common.String("app-01"),
		AvailabilityDomain: common.String("PARIS-AD-1"),
		Shape:              common.String("VM.Standard.E5.Flex"),
		ShapeConfig: &core.InstanceShapeConfig{
			Ocpus:       common.Float32(4),
			MemoryInGBs: common.Float32(64),
		},
	}
	bootAttachment := core.BootVolumeAttachment{
		Id:           common.String("ocid1.bootvolumeattachment.oc1.example"),
		BootVolumeId: common.String("ocid1.bootvolume.oc1.example"),
	}
	boot := NewBootVolumeRecord(bootAttachment, core.BootVolume{
		Id:        common.String("ocid1.bootvolume.oc1.example"),
		SizeInGBs: common.Int64(100),
	}, asOf)
	blockAttachment := core.ParavirtualizedVolumeAttachment{
		Id:       common.String("ocid1.volumeattachment.oc1.example"),
		VolumeId: common.String("ocid1.volume.oc1.example"),
	}
	block := NewBlockVolumeRecord(blockAttachment, core.Volume{
		Id:        common.String("ocid1.volume.oc1.example"),
		SizeInGBs: common.Int64(250),
	}, asOf)

	got := NewComputeInstanceRecord(
		"eu-paris-1",
		instance,
		[]BootVolumeRecord{boot},
		[]BlockVolumeRecord{block},
		true,
		true,
		asOf,
	)
	if got.Summary.AttachedStorageTotalSizeInGBs == nil ||
		*got.Summary.AttachedStorageTotalSizeInGBs != 350 {
		t.Fatalf("AttachedStorageTotalSizeInGBs = %v, want 350", got.Summary.AttachedStorageTotalSizeInGBs)
	}
	if !got.Summary.AttachedStorageSizeComplete {
		t.Fatal("AttachedStorageSizeComplete = false, want true")
	}
}

func TestComputeInstanceSummaryDoesNotClaimCompleteStorageAfterListFailure(t *testing.T) {
	got := NewComputeInstanceRecord(
		"eu-paris-1",
		core.Instance{Id: common.String("ocid1.instance.oc1.eu-paris-1.example")},
		nil,
		nil,
		false,
		true,
		time.Now(),
	)
	if got.Summary.BootVolumeTotalSizeInGBs != nil {
		t.Fatalf("BootVolumeTotalSizeInGBs = %v, want nil", got.Summary.BootVolumeTotalSizeInGBs)
	}
	if got.Summary.AttachedStorageSizeComplete {
		t.Fatal("AttachedStorageSizeComplete = true after boot attachment list failure")
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
