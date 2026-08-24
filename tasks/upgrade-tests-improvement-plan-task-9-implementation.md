# Task 9 — Implementation Plan: `logic-operator-builder-config` ConfigMap Checks

**Branch:** `SRVLOGIC-1138-migration-tests`  
**Status:** 🟡 Ready to implement  
**Depends on:** Task 10 ✅ (upgrade scenario base passing)

---

## Summary

Add `logic-operator-builder-config` ConfigMap checks **before** and **after** the
operator upgrade to confirm OLM reconciled both the from-version and to-version CSVs
fully, and that the builder Dockerfile template ConfigMap was not lost during the
transition.

---

## Sub-tasks

### 9a — New step: `ConfigMap "X" exists in cluster operator namespace`

**File:** `testbdd/steps/operator.go`

**Registration** (add inside `registerOperatorSteps`):

```go
ctx.Step(`^ConfigMap "([^"]*)" exists in cluster operator namespace$`,
    data.configMapExistsInClusterOperatorNamespace)
```

**Implementation** (new function, place after `configMapExists`):

```go
// configMapExistsInClusterOperatorNamespace checks that the named ConfigMap
// exists in the cluster-wide operator namespace (openshift-operators on OpenShift).
// This is independent of data.OperatorNamespace and data.Namespace — it always
// targets the OLM-managed namespace where the operator CSV deploys its config.
func (data *Data) configMapExistsInClusterOperatorNamespace(cmName string) error {
    ns := framework.GetClusterOperatorNamespace()
    framework.GetLogger(ns).Info("Checking if ConfigMap exists in cluster operator namespace",
        "configMap", cmName, "namespace", ns)

    exists, err := framework.IsConfigMapExist(types.NamespacedName{Name: cmName, Namespace: ns})
    if err != nil {
        return fmt.Errorf("error while checking if ConfigMap %s exists in %s: %v", cmName, ns, err)
    }
    if !exists {
        return fmt.Errorf("ConfigMap %s does not exist in namespace %s", cmName, ns)
    }
    return nil
}
```

---

### 9b — New step: `ConfigMap "X" in cluster operator namespace contains following strings:`

**File:** `testbdd/steps/operator.go`

**Registration** (add inside `registerOperatorSteps`):

```go
ctx.Step(`^ConfigMap "([^"]*)" in cluster operator namespace contains following strings:$`,
    data.configMapInClusterOperatorNamespaceContainsStrings)
```

**Implementation** (new function, place after `configMapExistsInClusterOperatorNamespace`):

```go
// configMapInClusterOperatorNamespaceContainsStrings validates that the named ConfigMap
// in the cluster-wide operator namespace contains all strings from the Gherkin table.
// Reuses the same fetch + scan logic as configMapContainsStrings but unconditionally
// targets framework.GetClusterOperatorNamespace() instead of configMapNamespace().
func (data *Data) configMapInClusterOperatorNamespaceContainsStrings(cmName string, table *godog.Table) error {
    ns := framework.GetClusterOperatorNamespace()
    framework.GetLogger(ns).Info("Validating ConfigMap contains strings in cluster operator namespace",
        "configMap", cmName, "namespace", ns)

    cm := &corev1.ConfigMap{}
    exists, err := framework.GetObjectWithKey(types.NamespacedName{Name: cmName, Namespace: ns}, cm)
    if err != nil {
        return fmt.Errorf("error fetching ConfigMap %s in %s: %v", cmName, ns, err)
    }
    if !exists {
        return fmt.Errorf("ConfigMap %s does not exist in namespace %s", cmName, ns)
    }

    var allDataValues strings.Builder
    for _, value := range cm.Data {
        allDataValues.WriteString(value)
    }

    for _, row := range table.Rows {
        if len(row.Cells) == 0 {
            continue
        }
        expectedString := resolveUpgradePlaceholders(row.Cells[0].Value)
        if strings.Contains(expectedString, "${RELATED_IMAGE_BASE_BUILDER}") {
            builderImage := config.GetRelatedImage("RELATED_IMAGE_BASE_BUILDER")
            if builderImage == "" {
                builderImage = "registry.redhat.io/openshift-serverless-1/logic-swf-builder-rhel9:1.38.0"
            }
            expectedString = strings.ReplaceAll(expectedString, "${RELATED_IMAGE_BASE_BUILDER}", builderImage)
        }
        if !strings.Contains(allDataValues.String(), expectedString) {
            return fmt.Errorf("ConfigMap '%s' in %s does not contain the expected string:\n'%s'",
                cmName, ns, expectedString)
        }
    }
    return nil
}
```

> **Note:** The `${RELATED_IMAGE_BASE_BUILDER}` resolution block is copied from
> `configMapContainsStrings` — both functions share the same placeholder logic.
> If the fallback default (`1.38.0`) needs updating, change it in both places.
> Consider extracting to a shared `resolveConfigMapString(s string) string` helper
> if drift becomes a maintenance concern.

---

### 9c — Feature: pre-upgrade builder-config check

**File:** `testbdd/features/operator/operator_upgrade.feature`

Insert immediately **after** `Given SonataFlow Operator at from-version is installed via OLM` (currently line 16):

```gherkin
    # ── Pre-upgrade: verify builder-config ConfigMap is present ──────────────
    # Confirms OLM reconciled the from-version CSV fully and the builder
    # Dockerfile template ConfigMap was created in the cluster operator namespace.
    Then ConfigMap "logic-operator-builder-config" exists in cluster operator namespace
    Then ConfigMap "logic-operator-builder-config" in cluster operator namespace contains following strings:
      | FROM       |
      | AS builder |
```

---

### 9d — Feature: post-upgrade builder-config check

**File:** `testbdd/features/operator/operator_upgrade.feature`

Insert immediately **after** `Then SonataFlow Operator running version matches upgrade target` (currently line 60):

```gherkin
    # ── Post-upgrade: verify builder-config ConfigMap is still present ────────
    # Confirms the to-version CSV preserved (or recreated) the builder ConfigMap
    # after OLM applied the upgrade. A missing ConfigMap here would indicate the
    # upgrade broke the operator's build configuration.
    Then ConfigMap "logic-operator-builder-config" exists in cluster operator namespace
    Then ConfigMap "logic-operator-builder-config" in cluster operator namespace contains following strings:
      | FROM       |
      | AS builder |
```

---

## Resulting scenario insertion points

```
Given  SonataFlow Operator at from-version is installed via OLM
Then   ConfigMap "logic-operator-builder-config" exists in cluster operator namespace    ← NEW (9c)
Then   ConfigMap "logic-operator-builder-config" in cluster operator namespace contains  ← NEW (9c)
         | FROM | AS builder |
Given  Namespace is created
...
Then   SonataFlow Operator running version matches upgrade target
Then   ConfigMap "logic-operator-builder-config" exists in cluster operator namespace    ← NEW (9d)
Then   ConfigMap "logic-operator-builder-config" in cluster operator namespace contains  ← NEW (9d)
         | FROM | AS builder |
Then   Deployment "sonataflow-platform-data-index-service" pods log contains ...
```

---

## No new config flags needed

Both steps hardwire to `framework.GetClusterOperatorNamespace()`. No CLI flags, no
new config fields, no changes to `hack/run-tests.sh`.

---

## Verification

After implementation, run:

```bash
cd testbdd
make run-tests \
  feature=features/operator/operator_upgrade.feature \
  cr_deployment_only=true \
  operator_installation_source=olm \
  "operator_catalog_image=registry-proxy.engineering.redhat.com/rh-osbs/iib:<IIB_INDEX>" \
  "upgrade.from_version=1.38.1" \
  "upgrade.to_version=1.39.0" \
  "upgrade.to_kogito_runtime_version=9.106.0.redhat-00002" \
  "upgrade.to_quarkus_core_version=3.33.2.redhat-00008" \
  tags="@upgradeTests"
```

Expect the two new `Then ConfigMap` lines to appear in the scenario output before
`Given Namespace is created` and immediately after `Then SonataFlow Operator running
version matches upgrade target`.

---

## Summary table

| Sub-task | File | Change | Risk |
|----------|------|--------|------|
| 9a | `testbdd/steps/operator.go` | Register + implement `configMapExistsInClusterOperatorNamespace` | Low |
| 9b | `testbdd/steps/operator.go` | Register + implement `configMapInClusterOperatorNamespaceContainsStrings` | Low |
| 9c | `operator_upgrade.feature` | 2 lines after from-version install | Low |
| 9d | `operator_upgrade.feature` | 2 lines after version-match check | Low |
