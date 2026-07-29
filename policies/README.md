# OCI IAM policy examples

The collector is read-only. OCI Search itself does not require a separate
Search permission; Search only returns resource metadata the caller is already
allowed to inspect. The Database API lookup and the region-subscription lookup
do require permissions.

For an identity-domain group, adapt the placeholders:

```text
Allow group <identity-domain>/<group-name> to inspect tenancies in tenancy
Allow group <identity-domain>/<group-name> to inspect autonomous-databases in tenancy
```

For an instance principal in a dynamic group:

```text
Allow dynamic-group <dynamic-group-name> to inspect tenancies in tenancy
Allow dynamic-group <dynamic-group-name> to inspect autonomous-databases in tenancy
```

For a resource principal, use the principal form appropriate to the OCI
service hosting the executable and grant the same two resource permissions.
There is deliberately no generic `any-user` statement here because the safe
condition differs for Functions, Container Instances, Data Flow, OKE, and
other runtimes.

These are starting points, not a substitute for your tenancy's IAM review.
Compartment-level Autonomous Database access can be used if the report should
cover only selected compartments, but the region-subscription permission is a
tenancy-level IAM permission.

References:

- [Details for IAM: `ListRegionSubscriptions` requires `TENANCY_INSPECT`](https://docs.oracle.com/en-us/iaas/Content/Identity/policyreference/iampolicyreference.htm)
- [Details for Search](https://docs.oracle.com/en-us/iaas/Content/Identity/Reference/searchpolicyreference.htm)
- [Autonomous Database IAM policy details](https://docs.oracle.com/en/cloud/paas/autonomous-database/adbsa/autonomous-database-iam-policies.html)
