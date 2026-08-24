# Upgrade Tests Improvement Plan — Task 9

**Branch:** `SRVLOGIC-1138-migration-tests`  
**Status:** 🟡 Planning — awaiting approval before implementation  
**Depends on:** `tasks/upgrade-tests-implementation.md` (base scenario passing)

---

## Goal

Verify that the `logic-operator-builder-config` ConfigMap is **present both
before and after the operator upgrade**. This ConfigMap is created by OLM in
the cluster-wide operator namespace (`openshift-operators`) when it installs or
upgrades the operator CSV. Its presence confirms the operator was fully
reconciled by OLM at both ends of the upgrade, and that the builder
configuration (Dockerfile template used to build preview-profile workflows) was
not lost during the transition.

---

## Background

### What is `logic-operator-builder-config`?

The ConfigMap is named `logic-operator-builder-config` (constructed from
[`logicOperatorBuilderConfigName`](testbdd/installers/sonataflow_installer.go:93)
= `logicOperatorSubscriptionName + "-builder-config"`).

It lives in the **cluster-wide operator namespace** — `openshift-operators` on
OpenShift (returned by
[`framework.GetClusterOperatorNamespace()`](bddframework/pkg/framework/operator.go:495)).

The installation feature already checks it for the YAML-install path:

```gherkin
# testbdd/features/installation/install_logic_operator_yaml.feature
And ConfigMap "logic-operator-builder-config" exists
And ConfigMap "logic-operator-builder-config" contains following strings:
  | FROM ${RELATED_IMAGE_BASE_BUILDER} AS builder                |
  | FROM registry.access.redhat.com/ubi9/openjdk-17-runtime:1.23 |
```

The upgrade scenario must replicate this check at two points:
1. **Pre-upgrade** — after from-version OLM install, before upgrade is triggered.
2. **Post-upgrade** — after `SonataFlow Operator running version matches upgrade
   target`, confirming the new CSV preserved the ConfigMap.

### Namespace routing obstacle

The existing `ConfigMap "X" exists` step resolves namespace via
[`configMapNamespace()`](testbdd/steps/operator.go:95), which returns either
`data.OperatorNamespace` (set only by the YAML/non-upgrade install path) or
`data.Namespace` (the scenario workload namespace). In the upgrade scenario
**neither** resolves to `openshift-operators` — this is intentional so that
platform-level resource checks land in the workload namespace.

A new dedicated step is therefore needed that explicitly targets the cluster
operator namespace.

---

## Task 9 — Validate `logic-operator-builder-config` ConfigMap before and after upgrade

### 9a — New Gherkin step: `ConfigMap "X" exists in cluster operator namespace`

**File:** `testbdd/steps/operator.go`

Register a new step:

```
^ConfigMap "([^"]*)" exists in cluster operator namespace$
```

Implementation `configMapExistsInClusterOperatorNamespace(cmName string)`:
- Resolves namespace to `framework.GetClusterOperatorNamespace()` (i.e.
  `openshift-operators`) unconditionally — independent of
  `data.OperatorNamespace` / `data.Namespace`.
- Delegates to the existing `framework.IsConfigMapExist` helper.

### 9b — New Gherkin step: `ConfigMap "X" in cluster operator namespace contains following strings:`

**File:** `testbdd/steps/operator.go`

Register a new step:

```
^ConfigMap "([^"]*)" in cluster operator namespace contains following strings:$
```

Implementation `configMapInClusterOperatorNamespaceContainsStrings(cmName string, table *godog.Table)`:
- Resolves namespace to `framework.GetClusterOperatorNamespace()`.
- Reuses the same table-scan and placeholder-resolution logic already in
  `configMapContainsStrings` — extract shared helper or call directly with the
  resolved namespace.

### 9c — Feature: pre-upgrade check

**File:** `testbdd/features/operator/operator_upgrade.feature`

Insert immediately **after** `Given SonataFlow Operator at from-version is installed via OLM`:

```gherkin
# ── Pre-upgrade: verify builder-config ConfigMap is present ──────────────
# Confirms OLM reconciled the from-version CSV fully and the builder
# Dockerfile template ConfigMap was created in the cluster operator namespace.
Then ConfigMap "logic-operator-builder-config" exists in cluster operator namespace
Then ConfigMap "logic-operator-builder-config" in cluster operator namespace contains following strings:
  | FROM                          |
  | AS builder                    |
```

> **Note on content strings:** The `FROM … AS builder` lines come from the
> Dockerfile template embedded in the CSV. The exact base image reference
> (`${RELATED_IMAGE_BASE_BUILDER}`) varies per stream — using the static
> fragment `FROM` / `AS builder` keeps the check version-agnostic for the
> from-version side. If the team prefers using `${RELATED_IMAGE_BASE_BUILDER}`
> placeholder (already supported by `configMapContainsStrings`), that can be
> added as-is since the resolver falls back to the default image when the env
> var is unset.

### 9d — Feature: post-upgrade check

**File:** `testbdd/features/operator/operator_upgrade.feature`

Insert immediately **after** `Then SonataFlow Operator running version matches upgrade target`:

```gherkin
# ── Post-upgrade: verify builder-config ConfigMap is still present ────────
# Confirms the to-version CSV preserved (or recreated) the builder ConfigMap
# after OLM applied the upgrade. A missing ConfigMap here would indicate the
# upgrade broke the operator's build configuration.
Then ConfigMap "logic-operator-builder-config" exists in cluster operator namespace
Then ConfigMap "logic-operator-builder-config" in cluster operator namespace contains following strings:
  | FROM                          |
  | AS builder                    |
```

---

## Summary

| Sub-task | File | Change | Risk |
|----------|------|--------|------|
| 9a | `testbdd/steps/operator.go` | New step: ConfigMap exists in cluster operator namespace | Low |
| 9b | `testbdd/steps/operator.go` | New step: ConfigMap in cluster operator namespace contains strings | Low |
| 9c | `operator_upgrade.feature` | Pre-upgrade builder-config check (2 steps) | Low |
| 9d | `operator_upgrade.feature` | Post-upgrade builder-config check (2 steps) | Low |

All sub-tasks are one commit. No new config flags, no new framework changes —
only a new step registration + two implementations in `operator.go`, and four
new Gherkin lines in the feature file.

---

## Open Questions

1. **Content strings for pre-upgrade check:** Should the pre-upgrade check use
   the generic `FROM` / `AS builder` fragments (version-agnostic), or should it
   also assert `${RELATED_IMAGE_BASE_BUILDER}` to verify the exact from-version
   builder image is present?

2. **Post-upgrade content depth:** Should the post-upgrade check additionally
   verify a to-version-specific string (e.g. a known image tag from the new
   CSV), or is presence + `FROM … AS builder` sufficient?
