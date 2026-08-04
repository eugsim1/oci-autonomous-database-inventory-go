# IAM policy guidance

The collector is read-only, but `GetInstance` requires a read-level instance
permission. Volume size lookups and attachment listings use inspect-level
Block Volume permissions.

## Identity-domain group

```text
Allow group <identity-domain>/<group-name> to inspect tenancies in tenancy
Allow group <identity-domain>/<group-name> to inspect autonomous-databases in tenancy
Allow group <identity-domain>/<group-name> to read instance-family in tenancy
Allow group <identity-domain>/<group-name> to inspect volume-family in tenancy
```

## Instance principal

Put the reporting Compute instance in a dynamic group, then grant:

```text
Allow dynamic-group <dynamic-group-name> to inspect tenancies in tenancy
Allow dynamic-group <dynamic-group-name> to inspect autonomous-databases in tenancy
Allow dynamic-group <dynamic-group-name> to read instance-family in tenancy
Allow dynamic-group <dynamic-group-name> to inspect volume-family in tenancy
```

## Why these verbs

- `inspect tenancies` permits `ListRegionSubscriptions`.
- `inspect autonomous-databases` permits Autonomous Database detail reads.
- `read instance-family` includes the `INSTANCE_READ` permission required by
  `GetInstance` and attachment read permissions.
- `inspect volume-family` covers volume inspection and volume-attachment
  inspection used by `GetBootVolume`, `GetVolume`, and attachment listing.
- OCI Search has no separate resource permission; results reflect the
  principal's permissions for each indexed resource.

If your security standard avoids aggregate resource families, replace the
family statements with individual resource-type policies after validating all
required permissions in Oracle's Core Services IAM reference.

## Resource principals

Resource-principal policy syntax depends on the hosting OCI service. Scope an
`any-user` statement with `request.principal.type` and any available
service-specific conditions. Do not use an unconditioned `any-user` grant.

## OKE workload identity

OKE workload identity is supported only by enhanced clusters. It identifies a
specific cluster, Kubernetes namespace, and service account without an API key
or dynamic group. Repeat the following condition for each required statement:

```text
Allow any-user to inspect tenancies in tenancy where all {
  request.principal.type = 'workload',
  request.principal.namespace = '<namespace>',
  request.principal.service_account = '<service-account>',
  request.principal.cluster_id = '<cluster-ocid>'
}
```

Use the same condition with `inspect autonomous-databases`, `read
instance-family`, and `inspect volume-family`. The separate OKE deployment
project generates these four policy statements. OKE workload identities cannot
be placed in a dynamic group.

## Compartment scope

The examples use `in tenancy` because the requested inventory spans every
compartment. A compartment-scoped policy intentionally produces a partial
inventory and Search will omit resources outside that scope.

## References

- [Core Services IAM reference](https://docs.oracle.com/en-us/iaas/Content/Identity/Reference/corepolicyreference.htm)
- [Database Service IAM reference](https://docs.oracle.com/en-us/iaas/Content/Identity/Reference/databasepolicyreference.htm)
- [Search permissions](https://docs.oracle.com/en-us/iaas/Content/Search/Concepts/querypermissions.htm)
- [Automatic Oracle-Tags defaults](https://docs.oracle.com/en-us/iaas/Content/Tagging/Concepts/understandingautomaticdefaulttags.htm)
- [OKE workload identity](https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/contenggrantingworkloadaccesstoresources.htm)
