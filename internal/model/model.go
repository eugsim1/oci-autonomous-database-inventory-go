package model

import (
	"sort"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/database"
)

const (
	AutonomousDatabaseSearchQuery = "query autonomousdatabase resources"
	ComputeInstanceSearchQuery    = "query instance resources"
)

type Region struct {
	Name         string `json:"name"`
	Key          string `json:"key,omitempty"`
	Status       string `json:"status"`
	IsHomeRegion bool   `json:"is_home_region"`
	Scanned      bool   `json:"scanned"`
}

type CollectionError struct {
	Stage      string `json:"stage"`
	Region     string `json:"region,omitempty"`
	ResourceID string `json:"resource_id,omitempty"`
	Message    string `json:"message"`
}

type DatabaseSummary struct {
	Region                         string         `json:"region"`
	ID                             string         `json:"id"`
	CompartmentID                  string         `json:"compartment_id"`
	DisplayName                    string         `json:"display_name"`
	DBName                         string         `json:"db_name"`
	LifecycleState                 string         `json:"lifecycle_state"`
	LifecycleDetails               string         `json:"lifecycle_details,omitempty"`
	Workload                       string         `json:"workload,omitempty"`
	DBVersion                      string         `json:"db_version,omitempty"`
	InfrastructureType             string         `json:"infrastructure_type,omitempty"`
	IsDedicated                    *bool          `json:"is_dedicated,omitempty"`
	IsFreeTier                     *bool          `json:"is_free_tier,omitempty"`
	ComputeModel                   string         `json:"compute_model,omitempty"`
	ComputeCount                   *float64       `json:"compute_count,omitempty"`
	ECPUs                          *float64       `json:"ecpus,omitempty"`
	OCPUs                          *float64       `json:"ocpus,omitempty"`
	LegacyCPUCoreCount             *int           `json:"legacy_cpu_core_count,omitempty"`
	MemoryPerComputeUnitInGBs      *float64       `json:"memory_per_compute_unit_gbs,omitempty"`
	IsComputeAutoScalingEnabled    *bool          `json:"is_compute_auto_scaling_enabled,omitempty"`
	DataStorageSizeInGBs           *int           `json:"data_storage_size_in_gbs,omitempty"`
	DataStorageSizeInTBs           *int           `json:"data_storage_size_in_tbs,omitempty"`
	NormalizedDataStorageSizeInGBs *float64       `json:"normalized_data_storage_size_in_gbs,omitempty"`
	UsedDataStorageSizeInGBs       *int           `json:"used_data_storage_size_in_gbs,omitempty"`
	UsedDataStorageSizeInTBs       *int           `json:"used_data_storage_size_in_tbs,omitempty"`
	AllocatedStorageSizeInTBs      *float64       `json:"allocated_storage_size_in_tbs,omitempty"`
	ActualUsedStorageSizeInTBs     *float64       `json:"actual_used_storage_size_in_tbs,omitempty"`
	IsStorageAutoScalingEnabled    *bool          `json:"is_storage_auto_scaling_enabled,omitempty"`
	LicenseModel                   string         `json:"license_model,omitempty"`
	DatabaseEdition                string         `json:"database_edition,omitempty"`
	Role                           string         `json:"role,omitempty"`
	OpenMode                       string         `json:"open_mode,omitempty"`
	PermissionLevel                string         `json:"permission_level,omitempty"`
	SubnetID                       string         `json:"subnet_id,omitempty"`
	PrivateEndpoint                string         `json:"private_endpoint,omitempty"`
	PublicEndpoint                 string         `json:"public_endpoint,omitempty"`
	IsAccessControlEnabled         *bool          `json:"is_access_control_enabled,omitempty"`
	IsMTLSConnectionRequired       *bool          `json:"is_mtls_connection_required,omitempty"`
	IsLocalDataGuardEnabled        *bool          `json:"is_local_data_guard_enabled,omitempty"`
	IsRemoteDataGuardEnabled       *bool          `json:"is_remote_data_guard_enabled,omitempty"`
	TimeCreated                    string         `json:"time_created,omitempty"`
	OracleTags                     OracleTagAudit `json:"oracle_tags"`
}

type DatabaseRecord struct {
	Summary       DatabaseSummary             `json:"summary"`
	Configuration database.AutonomousDatabase `json:"configuration"`
}

type Report struct {
	SchemaVersion        string                  `json:"schema_version"`
	GeneratedAt          time.Time               `json:"generated_at"`
	TenancyOCID          string                  `json:"tenancy_ocid"`
	Authentication       string                  `json:"authentication"`
	SearchQueries        map[string]string       `json:"search_queries"`
	SubscribedRegions    []Region                `json:"subscribed_regions"`
	DatabaseCount        int                     `json:"database_count"`
	ComputeInstanceCount int                     `json:"compute_instance_count"`
	BootVolumeCount      int                     `json:"boot_volume_count"`
	BlockVolumeCount     int                     `json:"attached_block_volume_count"`
	ErrorCount           int                     `json:"error_count"`
	Databases            []DatabaseRecord        `json:"databases"`
	ComputeInstances     []ComputeInstanceRecord `json:"compute_instances"`
	Errors               []CollectionError       `json:"errors,omitempty"`
}

func NewSummary(region string, adb database.AutonomousDatabase) DatabaseSummary {
	return NewSummaryAt(region, adb, time.Now().UTC())
}

func NewSummaryAt(region string, adb database.AutonomousDatabase, asOf time.Time) DatabaseSummary {
	computeCount := float64Pointer(adb.ComputeCount)
	memory := float64Pointer(adb.MemoryPerComputeUnitInGBs)
	summary := DatabaseSummary{
		Region:                      region,
		ID:                          stringValue(adb.Id),
		CompartmentID:               stringValue(adb.CompartmentId),
		DisplayName:                 stringValue(adb.DisplayName),
		DBName:                      stringValue(adb.DbName),
		LifecycleState:              string(adb.LifecycleState),
		LifecycleDetails:            stringValue(adb.LifecycleDetails),
		Workload:                    string(adb.DbWorkload),
		DBVersion:                   stringValue(adb.DbVersion),
		InfrastructureType:          string(adb.InfrastructureType),
		IsDedicated:                 adb.IsDedicated,
		IsFreeTier:                  adb.IsFreeTier,
		ComputeModel:                string(adb.ComputeModel),
		ComputeCount:                computeCount,
		LegacyCPUCoreCount:          adb.CpuCoreCount,
		MemoryPerComputeUnitInGBs:   memory,
		IsComputeAutoScalingEnabled: adb.IsAutoScalingEnabled,
		DataStorageSizeInGBs:        adb.DataStorageSizeInGBs,
		DataStorageSizeInTBs:        adb.DataStorageSizeInTBs,
		UsedDataStorageSizeInGBs:    adb.UsedDataStorageSizeInGBs,
		UsedDataStorageSizeInTBs:    adb.UsedDataStorageSizeInTBs,
		AllocatedStorageSizeInTBs:   adb.AllocatedStorageSizeInTBs,
		ActualUsedStorageSizeInTBs:  adb.ActualUsedDataStorageSizeInTBs,
		IsStorageAutoScalingEnabled: adb.IsAutoScalingForStorageEnabled,
		LicenseModel:                string(adb.LicenseModel),
		DatabaseEdition:             string(adb.DatabaseEdition),
		Role:                        string(adb.Role),
		OpenMode:                    string(adb.OpenMode),
		PermissionLevel:             string(adb.PermissionLevel),
		SubnetID:                    stringValue(adb.SubnetId),
		PrivateEndpoint:             stringValue(adb.PrivateEndpoint),
		PublicEndpoint:              stringValue(adb.PublicEndpoint),
		IsAccessControlEnabled:      adb.IsAccessControlEnabled,
		IsMTLSConnectionRequired:    adb.IsMtlsConnectionRequired,
		IsLocalDataGuardEnabled:     adb.IsLocalDataGuardEnabled,
		IsRemoteDataGuardEnabled:    adb.IsRemoteDataGuardEnabled,
		OracleTags:                  NewOracleTagAudit(adb.DefinedTags, asOf),
	}

	switch strings.ToUpper(summary.ComputeModel) {
	case "ECPU":
		summary.ECPUs = computeCount
	case "OCPU":
		summary.OCPUs = computeCount
	}
	if summary.OCPUs == nil {
		summary.OCPUs = float64Pointer(adb.OcpuCount)
	}
	if adb.DataStorageSizeInGBs != nil {
		value := float64(*adb.DataStorageSizeInGBs)
		summary.NormalizedDataStorageSizeInGBs = &value
	} else if adb.DataStorageSizeInTBs != nil {
		value := float64(*adb.DataStorageSizeInTBs) * 1024
		summary.NormalizedDataStorageSizeInGBs = &value
	}
	if adb.TimeCreated != nil {
		summary.TimeCreated = adb.TimeCreated.UTC().Format(time.RFC3339)
	}
	return summary
}

func (r *Report) Finalize() {
	sort.Slice(r.SubscribedRegions, func(i, j int) bool {
		return r.SubscribedRegions[i].Name < r.SubscribedRegions[j].Name
	})
	sort.Slice(r.Databases, func(i, j int) bool {
		left, right := r.Databases[i].Summary, r.Databases[j].Summary
		if left.Region != right.Region {
			return left.Region < right.Region
		}
		if left.DisplayName != right.DisplayName {
			return strings.ToLower(left.DisplayName) < strings.ToLower(right.DisplayName)
		}
		return left.ID < right.ID
	})
	sort.Slice(r.ComputeInstances, func(i, j int) bool {
		left, right := r.ComputeInstances[i].Summary, r.ComputeInstances[j].Summary
		if left.Region != right.Region {
			return left.Region < right.Region
		}
		if left.DisplayName != right.DisplayName {
			return strings.ToLower(left.DisplayName) < strings.ToLower(right.DisplayName)
		}
		return left.ID < right.ID
	})
	bootVolumeCount := 0
	blockVolumeCount := 0
	for i := range r.ComputeInstances {
		sort.Slice(r.ComputeInstances[i].BootVolumes, func(left, right int) bool {
			return r.ComputeInstances[i].BootVolumes[left].Summary.ID <
				r.ComputeInstances[i].BootVolumes[right].Summary.ID
		})
		sort.Slice(r.ComputeInstances[i].BlockVolumes, func(left, right int) bool {
			return r.ComputeInstances[i].BlockVolumes[left].Summary.ID <
				r.ComputeInstances[i].BlockVolumes[right].Summary.ID
		})
		bootVolumeCount += len(r.ComputeInstances[i].BootVolumes)
		blockVolumeCount += len(r.ComputeInstances[i].BlockVolumes)
	}
	sort.Slice(r.Errors, func(i, j int) bool {
		left, right := r.Errors[i], r.Errors[j]
		if left.Region != right.Region {
			return left.Region < right.Region
		}
		if left.Stage != right.Stage {
			return left.Stage < right.Stage
		}
		return left.ResourceID < right.ResourceID
	})
	r.DatabaseCount = len(r.Databases)
	r.ComputeInstanceCount = len(r.ComputeInstances)
	r.BootVolumeCount = bootVolumeCount
	r.BlockVolumeCount = blockVolumeCount
	r.ErrorCount = len(r.Errors)
}

func float64Pointer(value *float32) *float64 {
	if value == nil {
		return nil
	}
	result := float64(*value)
	return &result
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
