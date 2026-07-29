# OCI Autonomous Database Inventory for Go

`oci-adb-inventory` is a read-only Go utility that discovers every Autonomous
Database visible to the caller in every `READY` OCI region subscription,
retrieves the complete configuration from the Database Service API, and
creates timestamped JSON, CSV, and Markdown reports.

> [!IMPORTANT]
> **Disclaimer:** This is an independent personal project. It is not an Oracle
> product and is not affiliated with, sponsored by, supported by, or endorsed
> by Oracle Corporation. Review the source, IAM policies, generated reports,
> and OCI API behavior before using it in a production tenancy. See
> [DISCLAIMER.md](DISCLAIMER.md).

## What it collects

- all `READY` regions returned by Identity `ListRegionSubscriptions`;
- Autonomous Database OCIDs discovered with the exact structured query
  `query autonomousdatabase resources` in each region;
- the full `GetAutonomousDatabase` SDK object for every discovered OCID;
- ECPU/OCPU model and compute count without making a conversion;
- legacy CPU-core and OCPU fields when OCI returns them;
- exact configured storage in GB and/or TB, used and allocated storage fields;
- workload, lifecycle, database version, infrastructure, license, edition,
  scaling, Data Guard, endpoint, access-control, mTLS, and tag configuration;
- partial collection errors, including their region, stage, and resource OCID.

The JSON file is canonical and preserves all fields exposed by the pinned OCI
Go SDK. CSV and Markdown provide normalized, practical views.

## Architecture

The editable diagram uses an Autonomous Database service icon from Oracle's
OCI Architecture Diagram Toolkit.

![OCI Autonomous Database inventory architecture](docs/diagrams/oci-adb-inventory-architecture.svg)

### SDD sequence

The Software Design Description sequence shows authentication, all-region
fan-out, Search pagination, Database API enrichment, normalization, and atomic
report writing.

![OCI Autonomous Database inventory SDD sequence](docs/diagrams/oci-adb-inventory-sdd-sequence.svg)

- [Editable architecture diagram](docs/diagrams/oci-adb-inventory-architecture.drawio)
- [Architecture PNG](docs/diagrams/oci-adb-inventory-architecture.png)
- [Editable SDD sequence diagram](docs/diagrams/oci-adb-inventory-sdd-sequence.drawio)
- [SDD sequence PNG](docs/diagrams/oci-adb-inventory-sdd-sequence.png)
- [Software design description](docs/SDD.md)

## How discovery works

```text
Identity.ListRegionSubscriptions
  -> every READY region
     -> ResourceSearch.SearchResources
        query autonomousdatabase resources
        (full pagination)
        -> Autonomous Database OCIDs
           -> Database.GetAutonomousDatabase
              -> complete SDK configuration + normalized summary
                 -> timestamped JSON + CSV + Markdown
```

OCI Search scopes results to the selected region, which is why the program
executes the query once per `READY` subscription. Results only contain
resources the principal is authorized to inspect.

## Prerequisites

- Go 1.24 or newer;
- an OCI API signing-key configuration, an OCI Compute instance principal, or
  an OCI resource-principal runtime;
- IAM permission to inspect the tenancy and Autonomous Databases;
- network access to the Identity, Resource Search, and Database Service
  endpoints in the subscribed regions.

The project pins `github.com/oracle/oci-go-sdk/v65` at `v65.117.1`.

## IAM policy examples

For an identity-domain group, adapt the placeholders:

```text
Allow group <identity-domain>/<group-name> to inspect tenancies in tenancy
Allow group <identity-domain>/<group-name> to inspect autonomous-databases in tenancy
```

For an instance principal:

```text
Allow dynamic-group <dynamic-group-name> to inspect tenancies in tenancy
Allow dynamic-group <dynamic-group-name> to inspect autonomous-databases in tenancy
```

Search does not require a separate permission. It returns only the metadata
covered by the caller's existing inspect/read permissions. See
[the IAM notes and resource-principal guidance](policies/README.md).

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

Use another config and profile:

```bash
./bin/oci-adb-inventory \
  --auth api_key \
  --config-file /secure/path/oci-config \
  --profile REPORTING \
  --output-dir reports
```

The OCI config contains references to the private key; the private key is
never copied into a report.

## Run with an instance principal

From an OCI Compute instance included in an authorized dynamic group:

```bash
./bin/oci-adb-inventory \
  --auth instance_principal \
  --output-dir reports
```

## Run with a resource principal

From a supported OCI runtime with resource-principal environment variables:

```bash
./bin/oci-adb-inventory \
  --auth resource_principal \
  --output-dir reports
```

The hosting service's resource-principal policy condition must be scoped for
that service. Do not use an unconditioned `any-user` policy.

## Region selection

By default, every `READY` subscribed region is scanned. To request a subset:

```bash
./bin/oci-adb-inventory \
  --regions eu-paris-1,eu-frankfurt-1 \
  --output-dir reports
```

The program rejects a requested region that is not subscribed or not `READY`.
`--bootstrap-region` can override the region used for the initial Identity
call when the authentication provider does not supply the desired one.

## Reports

A run at `2026-07-29T10:11:12Z` writes:

```text
reports/
  oci-autonomous-database-inventory-20260729T101112Z.json
  oci-autonomous-database-inventory-20260729T101112Z.csv
  oci-autonomous-database-inventory-20260729T101112Z.md
```

### JSON

The canonical report contains run metadata, region subscriptions, errors, and
one record per Autonomous Database:

```json
{
  "summary": {
    "region": "eu-paris-1",
    "compute_model": "ECPU",
    "compute_count": 4,
    "ecpus": 4,
    "data_storage_size_in_gbs": 1024
  },
  "configuration": {
    "id": "ocid1.autonomousdatabase.oc1.eu-paris-1...",
    "computeModel": "ECPU",
    "computeCount": 4,
    "dataStorageSizeInGBs": 1024
  }
}
```

`configuration` is the complete SDK object, not the abbreviated example above.

### CPU semantics

The program follows the API fields:

| OCI fields | Summary behavior |
|---|---|
| `computeModel = ECPU`, `computeCount = N` | `ecpus = N` |
| `computeModel = OCPU`, `computeCount = N` | `ocpus = N` |
| legacy `ocpuCount` | retained as OCPUs when the preferred OCPU compute count is absent |
| legacy `cpuCoreCount` | retained separately as `legacy_cpu_core_count` |

No ECPU-to-OCPU conversion is attempted. Regional totals are base configured
compute values; they are not multiplied by an auto-scaling factor.

### Storage semantics

`dataStorageSizeInGBs` and `dataStorageSizeInTBs` are both retained exactly as
returned. For a common aggregation column, the program uses the exact GB value
when present; otherwise it computes `TB × 1024`.

The report also retains `usedDataStorageSizeInGBs`,
`usedDataStorageSizeInTBs`, `allocatedStorageSizeInTBs`, and
`actualUsedDataStorageSizeInTBs` when the API supplies them.

### CSV

The CSV contains one row per database and stable summary columns suitable for
Excel, databases, or FinOps processing. Nested SDK configuration stays in
JSON, where its structure and types can be preserved.

### Markdown

The Markdown report gives per-region totals, a compact database table, links
to the sibling JSON/CSV files, and a collection-error table.

## Failure behavior

Authentication and region-subscription failures stop the run. Regional Search
and individual database-detail failures are recorded while other regions and
resources continue.

Use strict mode for scheduling or CI:

```bash
./bin/oci-adb-inventory --strict --output-dir reports
```

Strict mode still writes the partial report, then returns a non-zero status if
any collection error was recorded. `--timeout` bounds the complete run:

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

API keys require the config and private key to be mounted read-only:

```bash
docker build -t oci-adb-inventory .
docker run --rm \
  -v "$HOME/.oci:/home/nonroot/.oci:ro" \
  -v "$PWD/reports:/reports" \
  oci-adb-inventory \
  --auth api_key \
  --output-dir /reports
```

For OCI principal authentication, use the runtime integration provided by the
hosting service instead of placing credentials in the image.

## Security notes

- The collector performs only Identity list, Resource Search, and Database get
  operations.
- Reports are created with restricted file permissions where supported.
- `reports/` is excluded from Git.
- Full JSON can contain OCIDs, endpoint names, ACLs, network metadata, tags,
  and customer-contact metadata. Store and share it accordingly.
- OCI Search is index-backed and can be eventually consistent immediately
  after a resource change.

## Project layout

```text
cmd/oci-adb-inventory/   CLI entry point
internal/config/         flag parsing and validation
internal/oci/            authentication, regions, Search, Database API
internal/model/          report and normalization model
internal/report/         atomic JSON, CSV, Markdown writers
docs/                    SDD and editable/rendered diagrams
policies/                IAM guidance
```

## Official references

- [OCI Search language syntax](https://docs.oracle.com/en-us/iaas/Content/Search/Concepts/querysyntax.htm)
- [Querying resources and regional scope](https://docs.oracle.com/en-us/iaas/Content/Search/Tasks/queryingresources.htm)
- [OCI Go SDK Resource Search package](https://docs.oracle.com/en-us/iaas/tools/go/latest/resourcesearch/index.html)
- [OCI Go SDK Database package](https://docs.oracle.com/en-us/iaas/tools/go/latest/database/index.html)
- [OCI Go SDK Identity package](https://docs.oracle.com/en-us/iaas/tools/go/latest/identity/index.html)
- [OCI SDK authentication methods](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/sdk_authentication_methods.htm)
- [IAM details for Search](https://docs.oracle.com/en-us/iaas/Content/Identity/Reference/searchpolicyreference.htm)

## License

Source code is available under the [MIT License](LICENSE). Oracle trademarks,
documentation, APIs, and OCI Architecture Diagram Toolkit assets remain
subject to their respective owners' terms.
