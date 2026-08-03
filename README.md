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
- OCI `timeCreated` plus `nb_created_since`, calculated as complete elapsed
  24-hour periods between that service timestamp and the report timestamp.

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

The canonical JSON preserves complete SDK objects. Three CSV files (Autonomous
Database, Compute/storage, and failed requests) and the Markdown report provide
stable normalized views.

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
        -> timestamped JSON + 3 CSV files + Markdown
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
  oci-tenancy-inventory-20260729T101112Z-failed-requests.csv
  oci-tenancy-inventory-20260729T101112Z.md
```

### JSON

The JSON report has schema version `2.1`. Each Compute record contains the
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

For Autonomous Databases, `time_created` is the value returned by
`GetAutonomousDatabase`, while `nb_created_since` is the number of complete
elapsed 24-hour periods from `time_created` to the run's fixed `generated_at`.
It is intentionally independent of `age_days_as_of_report`, which continues to
measure the `Oracle-Tags.CreatedOn` value. A blank `nb_created_since` means the
Database API did not return `timeCreated`.

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

The Autonomous Database CSV contains one row per database, including
`time_created` and `nb_created_since`. The Compute CSV contains one row per
attached boot or data volume, repeating the parent instance fields so it can be
filtered or pivoted directly. An instance with no retrievable attachments still
receives an instance-only row.

The failed-request CSV always has a header and contains one row per collection
error. For OCI service errors it preserves the HTTP status, service code,
retryability, operation, endpoint, client version, request timestamp, OPC
request ID, documentation links, a plain-language diagnosis, and suggested
actions. Resource `Get` failures also retain the compartment, display name,
lifecycle state, and creation time originally returned by Search, making a
stale index result easier to identify. Treat this file as sensitive tenancy
metadata.

### Markdown

The Markdown report provides regional totals, Autonomous Database details and
OCI creation age, Compute instance/tag-age/storage totals, individual
attached-volume sizes, and a concise error table linked to the full
failed-request CSV.

## Understanding `404 NotAuthorizedOrNotFound`

For `GetAutonomousDatabase`, OCI uses the same 404 response when either the
OCID no longer resolves or the caller is not allowed to inspect it. The
response does not tell the client which case occurred. Oracle classifies this
error as non-retryable; the SDK's normal retry policy therefore does not solve
it by repeating the unchanged call.

The reported Frankfurt requests used an `eu-frankfurt-1` Autonomous Database
OCID and the matching endpoint
`database.eu-frankfurt-1.oraclecloud.com`. That rules out an obvious regional
endpoint mismatch. It also confirms that request signing reached the Database
Service: invalid or absent authentication normally produces `401
NotAuthenticated`, not this resource-level 404.

Both Search visibility for `autonomous-databases` and
`GetAutonomousDatabase` require `AUTONOMOUS_DATABASE_INSPECT`. Therefore:

- if only a few Search results fail while other databases are retrieved, first
  investigate deleted/terminated or recently moved resources, stale
  index-backed Search metadata, and compartment/tag-conditional IAM;
- if every Autonomous Database `Get` fails, first investigate the active
  profile or principal, identity-domain/dynamic-group membership, tenancy
  override, and tenancy-wide versus compartment-scoped policies;
- if Search and `Get` were run under different credentials, a Search hit does
  not prove that the identity used for `Get` has access.

Use the same authentication mode and profile as the utility for a direct test:

```bash
ADB_OCID='ocid1.autonomousdatabase.oc1.eu-frankfurt-1.example'
oci db autonomous-database get \
  --autonomous-database-id "$ADB_OCID" \
  --region eu-frankfurt-1 \
  --profile DEFAULT
```

Then work through this checklist:

1. Confirm the OCID exists in the expected tenancy and region and was not
   terminated or moved after Search indexed it.
2. Confirm the caller is the intended API-key user, instance principal, or
   resource principal. If `--tenancy-id` is present, ensure it identifies the
   same tenancy as the signer.
3. Confirm an effective policy grants `inspect autonomous-databases` in the
   resource's current compartment or in the tenancy. Check identity-domain
   group names, dynamic-group rules, and all tag or request conditions.
4. Inspect the generated failed-request CSV. Compare
   `search_compartment_ocid`, `search_lifecycle_state`, and
   `search_time_created` with the live resource and retain `opc_request_id`.
5. Rerun after OCI Search has converged. If the direct `Get` still fails and
   the resource and policy are verified, open an Oracle Support request with
   the complete failed-request row and OPC request ID.

For maximum SDK diagnostics during a controlled troubleshooting run:

```bash
OCI_GO_SDK_DEBUG=info ./bin/oci-adb-inventory \
  --auth api_key \
  --profile DEFAULT \
  --regions eu-frankfurt-1 \
  --output-dir reports
```

Debug output can contain request metadata. Redirect it to a protected location
and do not publish it without review.

## Failure behavior

Authentication and region-subscription failures stop the run. Regional Search,
individual resource, attachment-list, and volume-detail errors are recorded
while independent regions and resources continue. Every run creates the
timestamped failed-request CSV, even when it contains only the header.

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

### Why the Dockerfile exists

Docker is optional; the native Go binary remains the simplest deployment when
Go or a prebuilt binary is available. The Dockerfile provides a reproducible
Linux build and a small runtime for servers where installing Go is undesirable.
It uses two stages:

1. `golang:1.25` downloads the pinned modules and builds a static,
   CGO-disabled Linux executable.
2. `gcr.io/distroless/static-debian12:nonroot` runs only that executable as a
   non-root user. The final image contains no Go toolchain, shell, or package
   manager.

The program is a one-shot CLI: it listens on no port, needs no `EXPOSE`, and
exits after writing the reports. The image does not include OCI credentials.
`.dockerignore` excludes reports, local caches, OCI configuration directories,
private-key formats, and environment files from the build context.

### Build the Linux image

```bash
docker build --pull -t oci-adb-inventory:2.1.0 .
docker image inspect oci-adb-inventory:2.1.0
```

The build requires access to the Go module proxy (or your configured private
proxy) and to the two base-image registries. Rebuild the image to pick up base
image security updates.

### Linux host with API-key authentication

Create the output directory first. Mount the OCI directory at the same absolute
path used on the Linux host so an absolute `key_file` in `~/.oci/config`
continues to resolve inside the container:

```bash
mkdir -p "$PWD/reports"
docker run --rm \
  --user "$(id -u):$(id -g)" \
  --env "HOME=$HOME" \
  --mount "type=bind,src=$HOME/.oci,dst=$HOME/.oci,readonly" \
  --mount "type=bind,src=$PWD/reports,dst=/reports" \
  oci-adb-inventory:2.1.0 \
  --auth api_key \
  --config-file "$HOME/.oci/config" \
  --profile DEFAULT \
  --output-dir /reports
```

If the config uses a relative or `~` key path, change it to the mounted
container path or supply a separate protected container-specific config. The
key and config must be readable by the numeric UID used for the container, and
the reports directory must be writable by it. Never `COPY` the key into the
project or image.

### Windows Docker Desktop with API keys

A Windows `key_file=C:\...` path is not valid inside a Linux container. Create
`%USERPROFILE%\.oci\config.docker` outside this repository with the same
tenancy, user, fingerprint, and region values as the normal profile, but use a
Linux container key path, for example:

```text
[DEFAULT]
user=ocid1.user.oc1..example
fingerprint=aa:bb:cc:dd:example
tenancy=ocid1.tenancy.oc1..example
region=eu-frankfurt-1
key_file=/oci/oci_api_key.pem
```

Assuming the private key is
`%USERPROFILE%\.oci\oci_api_key.pem`, run from PowerShell:

```powershell
New-Item -ItemType Directory -Force -Path .\reports | Out-Null
$reportDir = (Resolve-Path .\reports).Path
$ociDir = (Resolve-Path "$env:USERPROFILE\.oci").Path

docker run --rm `
  --mount "type=bind,source=$ociDir,target=/oci,readonly" `
  --mount "type=bind,source=$reportDir,target=/reports" `
  oci-adb-inventory:2.1.0 `
  --auth api_key `
  --config-file /oci/config.docker `
  --profile DEFAULT `
  --output-dir /reports
```

Docker Desktop must be allowed to share the two host directories. Keep
`config.docker` and the private key outside Git and restrict their Windows ACLs.

### OCI Compute instance principal

Run this only on an OCI Compute instance that belongs to the intended dynamic
group and has the policies shown above. No API-key mount is needed:

```bash
mkdir -p "$PWD/reports"
docker run --rm \
  --network host \
  --user "$(id -u):$(id -g)" \
  --mount "type=bind,src=$PWD/reports,dst=/reports" \
  oci-adb-inventory:2.1.0 \
  --auth instance_principal \
  --output-dir /reports
```

Host networking is shown so the SDK can reliably reach the OCI instance
metadata endpoint. Apply the host's container-network policy and firewall
standards; do not use instance-principal mode away from the intended OCI
instance.

### Resource principal

Resource-principal environment variables, session token, and private-key path
are supplied by the hosting OCI service and differ by runtime. Pass only the
service-provided variables and bind-mounted token/key paths at `docker run`
time. Do not bake them into an image or a Dockerfile layer. If the hosting
service does not support custom containers or forwarding its resource-principal
material, use the native binary or instance principal instead.

### Read reports and clean up

The five timestamped report files are under the mounted `reports` directory.
The container is removed by `--rm`; the reports remain on the host. To remove
only the locally built image after the reports have been secured:

```bash
docker image rm oci-adb-inventory:2.1.0
```

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
internal/report/         JSON, three CSV files, Markdown, atomic writes
docs/                    SDD and editable diagrams
policies/                IAM and resource-principal guidance
```

## Oracle documentation

- [Search query language](https://docs.oracle.com/en-us/iaas/Content/Search/Concepts/querysyntax.htm)
- [Querying resources](https://docs.oracle.com/en-us/iaas/Content/Search/Tasks/queryingresources.htm)
- [OCI API errors](https://docs.oracle.com/en-us/iaas/Content/API/References/apierrors.htm)
- [Database Service IAM reference](https://docs.oracle.com/en-us/iaas/Content/Identity/Reference/databasepolicyreference.htm)
- [Search IAM permissions](https://docs.oracle.com/en-us/iaas/Content/Identity/policyreference/searchpolicyreference.htm)
- [Automatic Oracle-Tags defaults](https://docs.oracle.com/en-us/iaas/Content/Tagging/Concepts/understandingautomaticdefaulttags.htm)
- [Core Services IAM policy reference](https://docs.oracle.com/en-us/iaas/Content/Identity/Reference/corepolicyreference.htm)
- [Getting boot-volume details](https://docs.oracle.com/en-us/iaas/Content/Block/Tasks/get-bv-boot-volume.htm)
- [Getting block-volume details](https://docs.oracle.com/en-us/iaas/Content/Block/Tasks/get-bv-volume.htm)
- [OCI Go SDK authentication](https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/gosdkgettingstarted.htm)

## License

MIT. See [LICENSE](LICENSE).
