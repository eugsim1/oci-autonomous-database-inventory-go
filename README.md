# OCI Autonomous Database and Compute Inventory for Go

`oci-adb-inventory` is a read-only Go utility that inventories Autonomous
Databases, Compute instances, and their currently attached boot and block
volumes in every `READY` OCI region subscribed by a tenancy. It uses OCI
Search for discovery, then authoritative service APIs for complete
configuration and exact storage sizes.

> [!IMPORTANT]
> **Disclaimer:** This is an independent personal project. It is not an Oracle
> product and is not affiliated with, sponsored by, supported by, or endorsed
> by Oracle Corporation. Review the source, IAM policies, generated reports,
> and OCI API behavior before using it in a production tenancy. See
> [DISCLAIMER.md](DISCLAIMER.md).

## What it collects

### Autonomous Databases

- all OCIDs found by `query autonomousdatabase resources`;
- the complete `GetAutonomousDatabase` SDK object;
- ECPU/OCPU model, configured compute, auto-scaling, and legacy CPU fields;
- configured, used, and allocated storage fields;
- workload, version, lifecycle, licensing, endpoints, networking, Data Guard,
  access-control, mTLS, tags, and all other fields returned by the pinned SDK.
- normalized `Oracle-Tags.CreatedOn`, elapsed age in days, and
  `Oracle-Tags.CreatedBy`, using the same audit rules as Compute.

### Compute and Block Volume

- all Compute OCIDs found by `query instance resources`;
- the complete `GetInstance` SDK object, including shape configuration;
- shape, OCPUs, VCPUs, memory, availability/fault domain, local disks, and state;
- every boot-volume attachment currently in `ATTACHED` state;
- every data-volume attachment currently in `ATTACHED` state;
- the complete `GetBootVolume` or `GetVolume` object for each attachment;
- exact API `sizeInGBs` and deprecated `sizeInMBs` values without rounding;
- per-instance boot, block, and combined attached-storage totals;
- attachment type, lifecycle, device, read-only/shareable flags, and encryption;
- `Oracle-Tags.CreatedOn`, `Oracle-Tags.CreatedBy`, normalized UTC creation time,
  and whole elapsed age in days as of the report timestamp for instances and
  volumes.

If attachment listing fails, or an attachment is found but its detail lookup
fails, the known attachment is retained where possible, its size is left
unknown, an error is recorded, and the instance's storage total is marked
incomplete. The tool never reports a partial total as complete.

The canonical JSON preserves complete SDK objects. The two CSV files and the
Markdown report provide stable normalized views.

## Architecture

The editable diagram uses an OCI-styled service topology and covers Database,
Compute, and Block Volume enrichment.

![OCI tenancy inventory architecture](docs/diagrams/oci-adb-inventory-architecture.svg)

### SDD sequence

The sequence diagram shows authentication, all-region Search fan-out, database
and compute enrichment, attachment pagination, volume lookups, Oracle tag age
calculation, and atomic report writing.

![OCI tenancy inventory SDD sequence](docs/diagrams/oci-adb-inventory-sdd-sequence.svg)

- [Editable architecture diagram](docs/diagrams/oci-adb-inventory-architecture.drawio)
- [Architecture PNG](docs/diagrams/oci-adb-inventory-architecture.png)
- [Editable SDD sequence diagram](docs/diagrams/oci-adb-inventory-sdd-sequence.drawio)
- [SDD sequence PNG](docs/diagrams/oci-adb-inventory-sdd-sequence.png)
- [Software design description](docs/SDD.md)

## Discovery and enrichment flow

```text
Identity.ListRegionSubscriptions
  -> every READY region
     -> SearchResources("query autonomousdatabase resources")
        -> Database.GetAutonomousDatabase
     -> SearchResources("query instance resources")
        -> Compute.GetInstance
        -> Compute.ListBootVolumeAttachments(instance, AD)
           -> Blockstorage.GetBootVolume
        -> Compute.ListVolumeAttachments(instance)
           -> Blockstorage.GetVolume
     -> normalize sizes + Oracle-Tags audit fields
        -> timestamped JSON + 2 CSV files + Markdown
```

OCI Search is region-scoped, so both paginated queries run in every selected
region. Results are de-duplicated by `(region, OCID)`. The service-specific
`Get` and attachment APIs are the source of truth for the report.

## Prerequisites

- Go 1.25 or newer;
- OCI API signing-key configuration, an OCI Compute instance principal, or an
  OCI resource-principal runtime;
- the IAM permissions below;
- network access to Identity, Resource Search, Database, Compute, and Block
  Volume endpoints in all selected regions.

The project pins `github.com/oracle/oci-go-sdk/v65` at `v65.117.1`.

## IAM policy examples

For an identity-domain group:

```text
Allow group <identity-domain>/<group-name> to inspect tenancies in tenancy
Allow group <identity-domain>/<group-name> to inspect autonomous-databases in tenancy
Allow group <identity-domain>/<group-name> to read instance-family in tenancy
Allow group <identity-domain>/<group-name> to inspect volume-family in tenancy
```

For an instance principal:

```text
Allow dynamic-group <dynamic-group-name> to inspect tenancies in tenancy
Allow dynamic-group <dynamic-group-name> to inspect autonomous-databases in tenancy
Allow dynamic-group <dynamic-group-name> to read instance-family in tenancy
Allow dynamic-group <dynamic-group-name> to inspect volume-family in tenancy
```

`read instance-family` is required because `GetInstance` needs
`INSTANCE_READ`. `inspect volume-family` provides the inspect permissions used
by the volume and attachment read operations. OCI Search uses the caller's
existing permissions and needs no separate Search policy. See
[policies/README.md](policies/README.md) for least-privilege notes.

## Build and test

```bash
go mod download
go test ./...
go vet ./...
go build -trimpath -o bin/oci-adb-inventory ./cmd/oci-adb-inventory
```

On Windows:

```powershell
go build -trimpath -o bin/oci-adb-inventory.exe ./cmd/oci-adb-inventory
```

## Run with API keys

The default mode reads the `DEFAULT` profile from `~/.oci/config` or the file
named by `OCI_CONFIG_FILE`:

```bash
./bin/oci-adb-inventory \
  --auth api_key \
  --profile DEFAULT \
  --output-dir reports
```

Use a different configuration and profile:

```bash
./bin/oci-adb-inventory \
  --auth api_key \
  --config-file /secure/path/oci-config \
  --profile REPORTING \
  --output-dir reports
```

The private key is only used by the SDK signer and is never copied into a
report.

## Run with an instance principal

```bash
./bin/oci-adb-inventory \
  --auth instance_principal \
  --output-dir reports
```

## Run with a resource principal

```bash
./bin/oci-adb-inventory \
  --auth resource_principal \
  --output-dir reports
```

The hosting service's resource-principal policy condition must be scoped to
that service. Do not use an unconditioned `any-user` policy.

## Region selection

All `READY` subscribed regions are scanned by default. To scan a subset:

```bash
./bin/oci-adb-inventory \
  --regions eu-paris-1,eu-frankfurt-1 \
  --output-dir reports
```

The tool rejects regions that are not subscribed or not `READY`.
`--bootstrap-region` controls the initial Identity endpoint when needed.

## Reports

A run at `2026-07-29T10:11:12Z` writes:

```text
reports/
  oci-tenancy-inventory-20260729T101112Z.json
  oci-tenancy-inventory-20260729T101112Z-autonomous-databases.csv
  oci-tenancy-inventory-20260729T101112Z-compute-instances.csv
  oci-tenancy-inventory-20260729T101112Z.md
```

### JSON

The JSON report has schema version `2.0`. Each Compute record contains the
complete instance object and nested boot/data volume objects:

```json
{
  "summary": {
    "display_name": "app-01",
    "shape": "VM.Standard.E5.Flex",
    "oracle_tags": {
      "created_on_raw": "2026-07-01T08:30:00Z",
      "created_on_utc": "2026-07-01T08:30:00Z",
      "created_by": "alice@example.com",
      "age_days_as_of_report": 28,
      "created_on_tag_status": "parsed"
    },
    "boot_volume_total_size_in_gbs": 100,
    "attached_block_volume_total_size_in_gbs": 500,
    "attached_storage_total_size_in_gbs": 600,
    "attached_storage_size_complete": true
  },
  "configuration": {
    "id": "ocid1.instance.oc1...",
    "shape": "VM.Standard.E5.Flex"
  },
  "boot_volumes": [],
  "attached_block_volumes": []
}
```

`configuration` and the nested volume records contain complete SDK objects,
not the abbreviated example above.

### Oracle-Tags creation semantics

Creation attribution for Autonomous Databases, Compute instances, boot
volumes, and block volumes is taken only from the defined-tag namespace
`Oracle-Tags`:

- `CreatedOn` is retained verbatim in `created_on_raw`;
- valid RFC3339 values are normalized to UTC in `created_on_utc`;
- `age_days_as_of_report` is the number of complete elapsed 24-hour periods
  between the tag timestamp and the report's `generated_at`;
- `CreatedBy` is retained verbatim;
- `created_on_tag_status` is `parsed`, `missing`, `invalid`, or `unavailable`
  when a volume detail lookup failed.

OCI's separate API `timeCreated` field is retained as `oci_time_created` but is
not substituted for a missing `Oracle-Tags.CreatedOn`. This makes the requested
tag provenance auditable. Tenancies without the automatic tag defaults, older
resources, or resources whose tags were removed can legitimately report
`missing`.

### Storage semantics

Compute storage values use the exact integer `sizeInGBs` returned by
`GetBootVolume` and `GetVolume`. The tool does not estimate guest filesystem
usage or partition sizes. Only attachments whose lifecycle state is
`ATTACHED` are counted.

Instance totals include:

| Field | Meaning |
|---|---|
| `boot_volume_total_size_in_gbs` | Sum of attached boot-volume API sizes |
| `attached_block_volume_total_size_in_gbs` | Sum of attached data-volume API sizes |
| `attached_storage_total_size_in_gbs` | Boot plus attached data volumes |
| `boot_volume_inventory_complete` | Boot-attachment listing completed |
| `block_volume_inventory_complete` | Block-attachment listing completed |
| `attached_storage_size_complete` | `true` only when both attachment listings complete and every discovered attachment has a known API size |

Local NVMe/storage supplied by a Compute shape is retained separately in shape
configuration and is not added to Block Volume totals.

Autonomous Database storage retains OCI's GB and TB fields exactly. Its
normalized aggregation uses the GB value when present, otherwise `TB * 1024`.

### CSV files

The Autonomous Database CSV contains one row per database. The Compute CSV
contains one row per attached boot or data volume, repeating the parent
instance fields so it can be filtered or pivoted directly. An instance with no
retrievable attachments still receives an instance-only row.

### Markdown

The Markdown report provides regional totals, Autonomous Database details,
Compute instance/tag-age/storage totals, individual attached-volume sizes, and
collection errors.

## Failure behavior

Authentication and region-subscription failures stop the run. Regional Search,
individual resource, attachment-list, and volume-detail errors are recorded
while independent regions and resources continue.

Strict mode writes the partial report and then returns non-zero if any
collection error occurred:

```bash
./bin/oci-adb-inventory --strict --output-dir reports
```

The timeout and bounded worker pool apply to the entire collection:

```bash
./bin/oci-adb-inventory --timeout 45m --workers 12
```

## Options

```text
--auth string               api_key, instance_principal, resource_principal
--bootstrap-region string   region for the initial Identity request
--config-file string        OCI SDK config path
--output-dir string         report directory (default "reports")
--profile string            OCI config profile (default "DEFAULT")
--regions string            comma-separated READY region subset
--strict                    fail after writing a partial report on errors
--tenancy-id string         optional explicit tenancy OCID
--timeout duration          overall timeout (default 30m)
--version                   print version
--workers int               concurrent OCI operations
```

## Container

```bash
docker build -t oci-adb-inventory .
docker run --rm \
  -v "$HOME/.oci:/home/nonroot/.oci:ro" \
  -v "$PWD/reports:/reports" \
  oci-adb-inventory \
  --auth api_key \
  --output-dir /reports
```

For principal authentication, use the supported identity integration of the
OCI runtime rather than placing credentials in the image.

## Security notes

- All OCI operations are reads.
- Reports use restricted file permissions where supported and are written
  through a temporary file followed by an atomic rename.
- `reports/` is excluded from Git.
- Full JSON can contain instance metadata, OCIDs, IP/network data, tags,
  endpoints, ACLs, and customer-contact metadata. Store it as sensitive data.
- OCI Search is index-backed and can be eventually consistent immediately
  after a resource change.

## Project layout

```text
cmd/oci-adb-inventory/   CLI entry point
internal/config/         flags and validation
internal/model/          canonical report model, tag audit, normalization
internal/oci/            auth, Search, Database, Compute, Block Volume clients
internal/report/         JSON, two CSV files, Markdown, atomic writes
docs/                    SDD and editable diagrams
policies/                IAM and resource-principal guidance
```

## Oracle documentation

- [Search query language](https://docs.oracle.com/en-us/iaas/Content/Search/Concepts/querysyntax.htm)
- [Querying resources](https://docs.oracle.com/en-us/iaas/Content/Search/Tasks/queryingresources.htm)
- [Automatic Oracle-Tags defaults](https://docs.oracle.com/en-us/iaas/Content/Tagging/Concepts/understandingautomaticdefaulttags.htm)
- [Core Services IAM policy reference](https://docs.oracle.com/en-us/iaas/Content/Identity/Reference/corepolicyreference.htm)
- [Getting boot-volume details](https://docs.oracle.com/en-us/iaas/Content/Block/Tasks/get-bv-boot-volume.htm)
- [Getting block-volume details](https://docs.oracle.com/en-us/iaas/Content/Block/Tasks/get-bv-volume.htm)
- [OCI Go SDK authentication](https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/gosdkgettingstarted.htm)

## License

MIT. See [LICENSE](LICENSE).
