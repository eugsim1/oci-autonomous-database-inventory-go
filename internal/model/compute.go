package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/core"
)

const oracleTagsNamespace = "Oracle-Tags"

type OracleTagAudit struct {
	CreatedOnRaw       string `json:"created_on_raw,omitempty"`
	CreatedOnUTC       string `json:"created_on_utc,omitempty"`
	CreatedBy          string `json:"created_by,omitempty"`
	CreatedByUser      string `json:"created_by_user,omitempty"`
	CreatedBySource    string `json:"created_by_source,omitempty"`
	AgeDaysAsOfReport  *int64 `json:"age_days_as_of_report,omitempty"`
	CreatedOnTagStatus string `json:"created_on_tag_status"`
}

type ComputeInstanceSummary struct {
	Region                            string         `json:"region"`
	ID                                string         `json:"id"`
	CompartmentID                     string         `json:"compartment_id"`
	DisplayName                       string         `json:"display_name"`
	AvailabilityDomain                string         `json:"availability_domain"`
	FaultDomain                       string         `json:"fault_domain,omitempty"`
	LifecycleState                    string         `json:"lifecycle_state"`
	Shape                             string         `json:"shape"`
	OCPUs                             *float64       `json:"ocpus,omitempty"`
	VCPUs                             *int           `json:"vcpus,omitempty"`
	MemoryInGBs                       *float64       `json:"memory_in_gbs,omitempty"`
	BaselineOCPUUtilization           string         `json:"baseline_ocpu_utilization,omitempty"`
	LocalDisks                        *int           `json:"local_disks,omitempty"`
	LocalDisksTotalSizeInGBs          *float64       `json:"local_disks_total_size_in_gbs,omitempty"`
	OCITimeCreated                    string         `json:"oci_time_created,omitempty"`
	OracleTags                        OracleTagAudit `json:"oracle_tags"`
	BootVolumeCount                   int            `json:"boot_volume_count"`
	AttachedBlockVolumeCount          int            `json:"attached_block_volume_count"`
	BootVolumeInventoryComplete       bool           `json:"boot_volume_inventory_complete"`
	BlockVolumeInventoryComplete      bool           `json:"block_volume_inventory_complete"`
	BootVolumeTotalSizeInGBs          *int64         `json:"boot_volume_total_size_in_gbs,omitempty"`
	AttachedBlockVolumeTotalSizeInGBs *int64         `json:"attached_block_volume_total_size_in_gbs,omitempty"`
	AttachedStorageTotalSizeInGBs     *int64         `json:"attached_storage_total_size_in_gbs,omitempty"`
	AttachedStorageSizeComplete       bool           `json:"attached_storage_size_complete"`
}

type VolumeSummary struct {
	Kind                           string         `json:"kind"`
	ID                             string         `json:"id"`
	CompartmentID                  string         `json:"compartment_id"`
	DisplayName                    string         `json:"display_name"`
	AvailabilityDomain             string         `json:"availability_domain"`
	LifecycleState                 string         `json:"lifecycle_state"`
	SizeInGBs                      *int64         `json:"size_in_gbs,omitempty"`
	SizeInMBs                      *int64         `json:"size_in_mbs,omitempty"`
	VPUsPerGB                      *int64         `json:"vpus_per_gb,omitempty"`
	AutoTunedVPUsPerGB             *int64         `json:"auto_tuned_vpus_per_gb,omitempty"`
	IsAutoTuneEnabled              *bool          `json:"is_auto_tune_enabled,omitempty"`
	OCITimeCreated                 string         `json:"oci_time_created,omitempty"`
	OracleTags                     OracleTagAudit `json:"oracle_tags"`
	AttachmentID                   string         `json:"attachment_id"`
	AttachmentType                 string         `json:"attachment_type"`
	AttachmentLifecycleState       string         `json:"attachment_lifecycle_state"`
	Device                         string         `json:"device,omitempty"`
	IsReadOnly                     *bool          `json:"is_read_only,omitempty"`
	IsShareable                    *bool          `json:"is_shareable,omitempty"`
	IsPvEncryptionInTransitEnabled *bool          `json:"is_pv_encryption_in_transit_enabled,omitempty"`
}

type BootVolumeRecord struct {
	Summary       VolumeSummary             `json:"summary"`
	Attachment    core.BootVolumeAttachment `json:"attachment"`
	Configuration core.BootVolume           `json:"configuration"`
}

type BlockVolumeRecord struct {
	Summary       VolumeSummary         `json:"summary"`
	Attachment    core.VolumeAttachment `json:"attachment"`
	Configuration core.Volume           `json:"configuration"`
}

type ComputeInstanceRecord struct {
	Summary       ComputeInstanceSummary `json:"summary"`
	Configuration core.Instance          `json:"configuration"`
	BootVolumes   []BootVolumeRecord     `json:"boot_volumes"`
	BlockVolumes  []BlockVolumeRecord    `json:"attached_block_volumes"`
}

func NewOracleTagAudit(definedTags map[string]map[string]interface{}, asOf time.Time) OracleTagAudit {
	return NewOracleTagAuditFromTags(definedTags, nil, asOf)
}

func NewOracleTagAuditFromTags(
	definedTags map[string]map[string]interface{},
	freeformTags map[string]string,
	asOf time.Time,
) OracleTagAudit {
	namespaceName, namespace := findTagNamespace(definedTags, oracleTagsNamespace)
	createdOn := findTagValue(namespace, "CreatedOn")
	createdBy := findTagValue(namespace, "CreatedBy")
	createdBySource := ""
	if createdBy != "" {
		createdBySource = "defined_tags:" + namespaceName + ".CreatedBy"
	}
	if createdOn == "" {
		_, createdOn = findFreeformTagValue(freeformTags, "CreatedOn")
	}
	if createdBy == "" {
		var key string
		key, createdBy = findFreeformTagValue(freeformTags, "CreatedBy")
		if createdBy != "" {
			createdBySource = "freeform_tags:" + key
		}
	}
	audit := OracleTagAudit{
		CreatedOnRaw:       createdOn,
		CreatedBy:          createdBy,
		CreatedByUser:      createdBy,
		CreatedBySource:    createdBySource,
		CreatedOnTagStatus: "missing",
	}
	if audit.CreatedOnRaw == "" {
		return audit
	}
	created, err := time.Parse(time.RFC3339Nano, audit.CreatedOnRaw)
	if err != nil {
		audit.CreatedOnTagStatus = "invalid"
		return audit
	}
	created = created.UTC()
	audit.CreatedOnUTC = created.Format(time.RFC3339Nano)
	ageDays := int64(asOf.UTC().Sub(created) / (24 * time.Hour))
	audit.AgeDaysAsOfReport = &ageDays
	audit.CreatedOnTagStatus = "parsed"
	return audit
}

func NewComputeInstanceRecord(
	region string,
	instance core.Instance,
	bootVolumes []BootVolumeRecord,
	blockVolumes []BlockVolumeRecord,
	bootVolumeInventoryComplete bool,
	blockVolumeInventoryComplete bool,
	asOf time.Time,
) ComputeInstanceRecord {
	summary := ComputeInstanceSummary{
		Region:                       region,
		ID:                           stringValue(instance.Id),
		CompartmentID:                stringValue(instance.CompartmentId),
		DisplayName:                  stringValue(instance.DisplayName),
		AvailabilityDomain:           stringValue(instance.AvailabilityDomain),
		FaultDomain:                  stringValue(instance.FaultDomain),
		LifecycleState:               string(instance.LifecycleState),
		Shape:                        stringValue(instance.Shape),
		OracleTags:                   NewOracleTagAuditFromTags(instance.DefinedTags, instance.FreeformTags, asOf),
		BootVolumeCount:              len(bootVolumes),
		AttachedBlockVolumeCount:     len(blockVolumes),
		BootVolumeInventoryComplete:  bootVolumeInventoryComplete,
		BlockVolumeInventoryComplete: blockVolumeInventoryComplete,
	}
	if instance.TimeCreated != nil {
		summary.OCITimeCreated = instance.TimeCreated.UTC().Format(time.RFC3339Nano)
	}
	if instance.ShapeConfig != nil {
		summary.OCPUs = float64Pointer(instance.ShapeConfig.Ocpus)
		summary.VCPUs = instance.ShapeConfig.Vcpus
		summary.MemoryInGBs = float64Pointer(instance.ShapeConfig.MemoryInGBs)
		summary.BaselineOCPUUtilization = string(instance.ShapeConfig.BaselineOcpuUtilization)
		summary.LocalDisks = instance.ShapeConfig.LocalDisks
		summary.LocalDisksTotalSizeInGBs = float64Pointer(instance.ShapeConfig.LocalDisksTotalSizeInGBs)
	}

	if bootVolumeInventoryComplete {
		summary.BootVolumeTotalSizeInGBs = sumBootVolumeSizes(bootVolumes)
	}
	if blockVolumeInventoryComplete {
		summary.AttachedBlockVolumeTotalSizeInGBs = sumBlockVolumeSizes(blockVolumes)
	}
	if summary.BootVolumeTotalSizeInGBs != nil && summary.AttachedBlockVolumeTotalSizeInGBs != nil {
		total := *summary.BootVolumeTotalSizeInGBs + *summary.AttachedBlockVolumeTotalSizeInGBs
		summary.AttachedStorageTotalSizeInGBs = &total
		summary.AttachedStorageSizeComplete = true
	}

	return ComputeInstanceRecord{
		Summary:       summary,
		Configuration: instance,
		BootVolumes:   bootVolumes,
		BlockVolumes:  blockVolumes,
	}
}

func NewBootVolumeRecord(
	attachment core.BootVolumeAttachment,
	volume core.BootVolume,
	asOf time.Time,
) BootVolumeRecord {
	summary := VolumeSummary{
		Kind:                           "boot",
		ID:                             stringValue(volume.Id),
		CompartmentID:                  stringValue(volume.CompartmentId),
		DisplayName:                    stringValue(volume.DisplayName),
		AvailabilityDomain:             stringValue(volume.AvailabilityDomain),
		LifecycleState:                 string(volume.LifecycleState),
		SizeInGBs:                      volume.SizeInGBs,
		SizeInMBs:                      volume.SizeInMBs,
		VPUsPerGB:                      volume.VpusPerGB,
		AutoTunedVPUsPerGB:             volume.AutoTunedVpusPerGB,
		IsAutoTuneEnabled:              volume.IsAutoTuneEnabled,
		OracleTags:                     NewOracleTagAuditFromTags(volume.DefinedTags, volume.FreeformTags, asOf),
		AttachmentID:                   stringValue(attachment.Id),
		AttachmentType:                 "boot",
		AttachmentLifecycleState:       string(attachment.LifecycleState),
		IsPvEncryptionInTransitEnabled: attachment.IsPvEncryptionInTransitEnabled,
	}
	if volume.TimeCreated != nil {
		summary.OCITimeCreated = volume.TimeCreated.UTC().Format(time.RFC3339Nano)
	}
	return BootVolumeRecord{
		Summary:       summary,
		Attachment:    attachment,
		Configuration: volume,
	}
}

func NewUnavailableBootVolumeRecord(attachment core.BootVolumeAttachment) BootVolumeRecord {
	return BootVolumeRecord{
		Summary: VolumeSummary{
			Kind:                     "boot",
			ID:                       stringValue(attachment.BootVolumeId),
			CompartmentID:            stringValue(attachment.CompartmentId),
			AvailabilityDomain:       stringValue(attachment.AvailabilityDomain),
			AttachmentID:             stringValue(attachment.Id),
			AttachmentType:           "boot",
			AttachmentLifecycleState: string(attachment.LifecycleState),
			OracleTags: OracleTagAudit{
				CreatedOnTagStatus: "unavailable",
			},
		},
		Attachment: attachment,
	}
}

func NewBlockVolumeRecord(
	attachment core.VolumeAttachment,
	volume core.Volume,
	asOf time.Time,
) BlockVolumeRecord {
	summary := VolumeSummary{
		Kind:                           "block",
		ID:                             stringValue(volume.Id),
		CompartmentID:                  stringValue(volume.CompartmentId),
		DisplayName:                    stringValue(volume.DisplayName),
		AvailabilityDomain:             stringValue(volume.AvailabilityDomain),
		LifecycleState:                 string(volume.LifecycleState),
		SizeInGBs:                      volume.SizeInGBs,
		SizeInMBs:                      volume.SizeInMBs,
		VPUsPerGB:                      volume.VpusPerGB,
		AutoTunedVPUsPerGB:             volume.AutoTunedVpusPerGB,
		IsAutoTuneEnabled:              volume.IsAutoTuneEnabled,
		OracleTags:                     NewOracleTagAuditFromTags(volume.DefinedTags, volume.FreeformTags, asOf),
		AttachmentID:                   stringValue(attachment.GetId()),
		AttachmentType:                 volumeAttachmentType(attachment),
		AttachmentLifecycleState:       string(attachment.GetLifecycleState()),
		Device:                         stringValue(attachment.GetDevice()),
		IsReadOnly:                     attachment.GetIsReadOnly(),
		IsShareable:                    attachment.GetIsShareable(),
		IsPvEncryptionInTransitEnabled: attachment.GetIsPvEncryptionInTransitEnabled(),
	}
	if volume.TimeCreated != nil {
		summary.OCITimeCreated = volume.TimeCreated.UTC().Format(time.RFC3339Nano)
	}
	return BlockVolumeRecord{
		Summary:       summary,
		Attachment:    attachment,
		Configuration: volume,
	}
}

func NewUnavailableBlockVolumeRecord(attachment core.VolumeAttachment) BlockVolumeRecord {
	return BlockVolumeRecord{
		Summary: VolumeSummary{
			Kind:                           "block",
			ID:                             stringValue(attachment.GetVolumeId()),
			CompartmentID:                  stringValue(attachment.GetCompartmentId()),
			AvailabilityDomain:             stringValue(attachment.GetAvailabilityDomain()),
			AttachmentID:                   stringValue(attachment.GetId()),
			AttachmentType:                 volumeAttachmentType(attachment),
			AttachmentLifecycleState:       string(attachment.GetLifecycleState()),
			Device:                         stringValue(attachment.GetDevice()),
			IsReadOnly:                     attachment.GetIsReadOnly(),
			IsShareable:                    attachment.GetIsShareable(),
			IsPvEncryptionInTransitEnabled: attachment.GetIsPvEncryptionInTransitEnabled(),
			OracleTags: OracleTagAudit{
				CreatedOnTagStatus: "unavailable",
			},
		},
		Attachment: attachment,
	}
}

func findTagNamespace(
	definedTags map[string]map[string]interface{},
	name string,
) (string, map[string]interface{}) {
	// Prefer the canonical namespace when both forms exist, then accept common
	// separator variants such as Oracle_Tags returned by some tag definitions.
	for namespace, values := range definedTags {
		if strings.EqualFold(namespace, name) {
			return namespace, values
		}
	}
	target := normalizeTagNamespace(name)
	for namespace, values := range definedTags {
		if normalizeTagNamespace(namespace) == target {
			return namespace, values
		}
	}
	return "", nil
}

func normalizeTagNamespace(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, " ", "")
	return value
}

func findTagValue(values map[string]interface{}, name string) string {
	for key, value := range values {
		if strings.EqualFold(key, name) && value != nil {
			if text, ok := value.(string); ok {
				return text
			}
			return fmt.Sprint(value)
		}
	}
	return ""
}

func findFreeformTagValue(values map[string]string, name string) (string, string) {
	target := normalizeQualifiedTagName(oracleTagsNamespace + "." + name)
	for key, value := range values {
		if normalizeQualifiedTagName(key) == target && value != "" {
			return key, value
		}
	}
	return "", ""
}

func normalizeQualifiedTagName(value string) string {
	replacer := strings.NewReplacer(
		"-", "",
		"_", "",
		" ", "",
		"\t", "",
		".", "",
		"/", "",
		":", "",
	)
	return strings.ToLower(replacer.Replace(strings.TrimSpace(value)))
}

func volumeAttachmentType(attachment core.VolumeAttachment) string {
	switch attachment.(type) {
	case core.IScsiVolumeAttachment:
		return "iscsi"
	case core.ParavirtualizedVolumeAttachment:
		return "paravirtualized"
	case core.EmulatedVolumeAttachment:
		return "emulated"
	default:
		return "unknown"
	}
}

func sumBootVolumeSizes(volumes []BootVolumeRecord) *int64 {
	var total int64
	for _, volume := range volumes {
		if volume.Summary.SizeInGBs == nil {
			return nil
		}
		total += *volume.Summary.SizeInGBs
	}
	return &total
}

func sumBlockVolumeSizes(volumes []BlockVolumeRecord) *int64 {
	var total int64
	for _, volume := range volumes {
		if volume.Summary.SizeInGBs == nil {
			return nil
		}
		total += *volume.Summary.SizeInGBs
	}
	return &total
}
