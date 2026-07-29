# Software design description

## Purpose

`oci-adb-inventory` creates a point-in-time, tenancy-wide configuration
inventory of Autonomous Databases visible to an authenticated OCI principal.
It deliberately uses OCI Resource Search for discovery and the Database
Service `GetAutonomousDatabase` operation for authoritative per-resource
details.

## Context

The process is read-only:

1. Select an OCI SDK configuration provider.
2. Resolve the tenancy OCID from the provider unless explicitly supplied.
3. Call Identity `ListRegionSubscriptions`.
4. Keep every `READY` subscription, or validate an explicit region subset.
5. In each selected region, paginate the structured Search query
   `query autonomousdatabase resources`.
6. De-duplicate Search results by `(region, OCID)`.
7. Run bounded-concurrent `GetAutonomousDatabase` requests against the
   Database Service endpoint in the resource's region.
8. Preserve the complete SDK object and derive a stable flat summary.
9. Sort results and atomically write timestamped JSON, CSV, and Markdown.

## Components

| Component | Responsibility |
|---|---|
| CLI/config | Validate flags, authentication mode, region filter, concurrency, timeout, and output directory. |
| Provider factory | Construct API-key, instance-principal, or resource-principal SDK configuration. |
| Region discovery | Retrieve tenancy subscriptions and select only `READY` regions. |
| Search workers | Execute and paginate the structured resource query in each region. |
| Database workers | Resolve every discovered OCID with `GetAutonomousDatabase`. |
| Normalizer | Map `computeModel`/`computeCount` to explicit ECPU or OCPU summary fields and retain exact GB/TB storage fields. |
| Report writer | Write deterministic full-detail JSON, flat CSV, and reader-friendly Markdown. |

## Data model

The JSON report is the canonical artifact. Each database record contains:

- `summary`: stable, normalized fields intended for reporting and automation;
- `configuration`: the complete `database.AutonomousDatabase` object returned
  by the pinned OCI Go SDK.

The summary does not invent a CPU conversion. When `computeModel` is `ECPU`,
`computeCount` is reported as ECPUs. When it is `OCPU`, it is reported as
OCPUs. Legacy `cpuCoreCount` remains a separate field.

Storage is preserved in the unit returned by OCI. A normalized GiB value is
also provided for aggregation: an exact `dataStorageSizeInGBs` value takes
precedence; otherwise `dataStorageSizeInTBs × 1024` is used.

## Concurrency and ordering

The `--workers` limit applies independently to region Search calls and
database-detail calls. OCI clients are cached by region. The OCI SDK default
retry policy is attached to every API request. Final database and error
records are sorted, so concurrent collection does not make reports unstable.

## Error handling

Failure to authenticate or list region subscriptions is fatal. A failure in
one regional Search or one database lookup is recorded in the report while
other work continues. `--strict` writes the partial report and then returns a
non-zero exit status when any collection error occurred.

An overall context timeout bounds the run. Generated files are written through
temporary files in the destination directory and renamed only after a
successful close.

## Security

The tool never creates, updates, starts, stops, or deletes OCI resources. It
does not print private keys or tokens. Full JSON can still contain sensitive
operational metadata such as OCIDs, endpoints, network ACLs, tags, and
customer-contact details. The default `reports/` directory is excluded from
Git.

## Diagrams

- [Editable OCI architecture](diagrams/oci-adb-inventory-architecture.drawio)
- [Editable sequence/SDD diagram](diagrams/oci-adb-inventory-sdd-sequence.drawio)

The architecture diagram uses an Autonomous Database service icon derived from
Oracle's OCI Architecture Diagram Toolkit. Oracle and OCI marks remain the
property of Oracle Corporation and/or its affiliates.
