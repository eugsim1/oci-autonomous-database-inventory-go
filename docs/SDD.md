# Software Design Description

## 1. Purpose

This utility produces a read-only, point-in-time tenancy inventory of:

1. Autonomous Databases and their full Database Service configuration.
2. Compute instances and their full Compute Service configuration.
3. Boot and block volumes currently attached to each Compute instance.
4. Creation audit fields sourced from `Oracle-Tags`.

The collector scans all `READY` subscribed OCI regions unless the operator
provides an explicit subset.

## 2. Design goals

- Use OCI Search for tenancy-wide, region-scoped discovery.
- Use authoritative service APIs for configuration and storage sizes.
- Preserve complete SDK responses in canonical JSON.
- Make `Oracle-Tags.CreatedOn` provenance explicit and independently auditable.
- Calculate Autonomous Database `nb_created_since` from the authoritative OCI
  `timeCreated` field at the fixed report timestamp.
- Never present a partial storage total as complete.
- Continue across independent failures and make every error visible.
- Bound concurrency and total runtime.
- Write deterministic timestamped reports without partially published files.

## 3. Components

### CLI and configuration

`cmd/oci-adb-inventory` parses authentication, profile, region selection,
worker count, timeout, strict mode, and output path. It owns the process exit
contract.

### Authentication provider

`internal/oci/provider.go` selects one OCI SDK configuration provider:

- API signing key;
- Compute instance principal;
- resource principal;
- enhanced OKE workload identity, using the projected Kubernetes service-account
  token and OCI SDK workload-identity exchange.

The tenancy OCID comes from `--tenancy-id` or the provider.

### Region discovery

The Identity client calls `ListRegionSubscriptions`. Only subscriptions in
`READY` state are scanned. User-requested regions are validated against that
set.

### Resource discovery

Each selected region receives two fully paginated Search requests:

```text
query autonomousdatabase resources
query instance resources
```

Results are de-duplicated by `(region, OCID)`. Search is used for discovery;
its compartment, display name, lifecycle, and creation timestamp are retained
for diagnosing a later detail-lookup failure.

### Autonomous Database enrichment

The regional Database client calls `GetAutonomousDatabase` for each database
OCID. The response is stored in full and normalized into stable CPU, storage,
lifecycle, licensing, network, and Oracle tag audit fields.

### Compute enrichment

The regional Compute client calls `GetInstance` for every discovered instance.
The complete instance object is preserved. Shape configuration is normalized
into OCPU, VCPU, memory, local-disk, and baseline-utilization fields.

### Attached storage enrichment

For each retrieved instance:

1. `ListBootVolumeAttachments` uses the instance OCID, compartment, and
   availability domain. Pagination is complete.
2. Attachments not in `ATTACHED` state are excluded.
3. `GetBootVolume` returns the authoritative boot-volume object and
   `sizeInGBs`.
4. `ListVolumeAttachments` uses the instance OCID and compartment. Pagination
   is complete.
5. Data-volume attachments not in `ATTACHED` state are excluded.
6. `GetVolume` returns the authoritative block-volume object and `sizeInGBs`.

The attachment object and corresponding volume object are both retained.

### Oracle tag audit

The normalizer performs a case-insensitive lookup of the `Oracle-Tags`
namespace and its `CreatedOn` and `CreatedBy` keys.

`CreatedOn` handling:

- preserve the original value;
- parse RFC3339/RFC3339Nano;
- normalize valid values to UTC;
- calculate complete elapsed 24-hour periods at `report.generated_at`;
- label the value `parsed`, `missing`, `invalid`, or `unavailable` when a
  volume detail call failed.

OCI `timeCreated` is stored separately. For Autonomous Databases it is used to
calculate `nb_created_since` as complete elapsed 24-hour periods at
`report.generated_at`. It is never used as a silent fallback for the requested
tag-derived age.

### Report writer

The writer creates five files with one shared UTC timestamp:

- canonical combined JSON;
- Autonomous Database CSV;
- Compute/attached-volume CSV;
- failed-request diagnostics CSV;
- Markdown summary.

Each file is written with restricted permissions to a temporary file in the
destination directory and then atomically renamed.

## 4. Concurrency model

The configured worker count bounds:

- concurrent regional Search calls;
- concurrent Autonomous Database detail calls;
- concurrent Compute instance enrichment tasks.

Within one Compute task, attachment pages and volume details are retrieved in a
deterministic sequence. Independent instances still run concurrently. SDK
clients are cached per region and protected during cache creation.

The root context applies the configured timeout to the complete operation.

## 5. Data model

The JSON schema version is `2.1`.

`Report` contains:

- run metadata and authentication mode;
- both exact Search queries;
- region subscription state;
- counts;
- `DatabaseRecord[]`;
- `ComputeInstanceRecord[]`;
- `CollectionError[]`.

Each `ComputeInstanceRecord` contains:

- normalized `ComputeInstanceSummary`;
- complete `core.Instance`;
- `BootVolumeRecord[]`;
- `BlockVolumeRecord[]`.

Each volume record contains:

- normalized exact size, tag audit, and attachment metadata;
- complete attachment object;
- complete `core.BootVolume` or `core.Volume`.

Per-instance totals are pointers. A nil total means an attachment listing did
not complete or at least one size is unknown. Separate boot/block inventory
completeness flags identify list failures. `attached_storage_size_complete` is
true only when both attachment listings completed and all discovered attached
volumes have a size.

## 6. Error contract

The following failures are fatal before reports can be meaningfully produced:

- configuration or authentication provider creation;
- inability to resolve the tenancy;
- inability to list region subscriptions;
- invalid requested region selection.

The following are retained as `CollectionError` values:

- regional Search failure;
- database or instance detail failure;
- boot/block attachment list failure;
- boot/block volume detail failure.

OCI SDK service errors are normalized into HTTP status, service code,
retryability, operation, request endpoint/timestamp, client version, OPC request
ID, documentation links, diagnosis, and suggested actions. A dedicated CSV
contains the complete structured failure record. In particular, a 404
`NotAuthorizedOrNotFound` is described as an ambiguous missing-resource or IAM
failure and is not treated as retryable by the default SDK policy.

When a volume-detail call fails, the attachment remains in JSON/CSV with an
unknown size. In strict mode, partial reports are still written before the CLI
returns a non-zero status.

## 7. Security

- The application invokes only read operations.
- API private keys remain with the SDK configuration provider.
- Reports are excluded from Git and use restricted permissions where supported.
- Canonical JSON is sensitive because full instance metadata, tags, network
  identifiers, endpoints, and database configuration are retained.
- The recommended IAM policy grants `read instance-family` because
  `GetInstance` needs `INSTANCE_READ`, and `inspect volume-family` for volume
  and attachment inspection.

## 8. Verification

The project verification gate is:

```text
go test ./...
go vet ./...
go build -trimpath ./cmd/oci-adb-inventory
```

Unit tests cover region selection, ECPU/OCPU normalization, exact storage
totals, OCI and Oracle tag age calculation, rich OCI error extraction, JSON
preservation, all three CSV formats, Markdown generation, and timestamped
filenames.
