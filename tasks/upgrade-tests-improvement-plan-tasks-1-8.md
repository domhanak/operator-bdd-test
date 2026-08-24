# Upgrade Tests Improvement Plan — Tasks 1–8

**Branch:** `SRVLOGIC-1138-migration-tests`  
**Status:** ✅ Tasks 1–8 implemented and validated (`go vet` passing)  
**Depends on:** `tasks/upgrade-tests-implementation.md` (base scenario passing)

---

## Goals

1. **Robustness** — gate each phase of the upgrade on observable cluster state
   instead of relying only on timing/timeouts.
2. **CSV phase monitoring** — wait for the target CSV to reach `Succeeded`
   before checking operator resources.
3. **KOGITO_RUNTIME + Quarkus version checks** — after upgrade, verify that DI
   and JS pods are running the specific `kogito_runtime_version` and
   `quarkus_core_version` strings embedded in their startup log line.  
   This is **critical** because the current `'started in'` check can
   inadvertently pass on the **old (from-version) pod** that is still running
   alongside the new one immediately after upgrade (confirmed from run logs —
   see Evidence section below).
4. **Two new test parameters:**
   - `--tests.upgrade.to_kogito_runtime_version` (default: `9.106.0.redhat-00002`)
   - `--tests.upgrade.to_quarkus_core_version` (default: `3.33.2.redhat-00008`)

---

## Evidence from Real Run Logs (bdd-7f78, 2026-08-18)

The upgrade step left two DI pods and two JS pods running simultaneously.
`WaitForAnyPodsByDeploymentToContainTextInLog` accepts the **first** pod whose
log contains the search text — which can be the old pod:

| Pod | Startup log line |
|-----|-----------------|
| `sonataflow-platform-data-index-service-6f79f5bf84-vmhmz` (**old**) | `data-index-service-postgresql 9.105.0.redhat-00003 on JVM (powered by Quarkus 3.27.3.redhat-00002) started in 37.304s` |
| `sonataflow-platform-data-index-service-75dc659d5c-757bl` (**new**) | `data-index-service-postgresql 9.106.0.redhat-00002 on JVM (powered by Quarkus 3.33.2.redhat-00008) started in 58.204s` |
| `sonataflow-platform-jobs-service-868754b5cf-mxwqk` (**old**) | `jobs-service-postgresql 9.105.0.redhat-00003 on JVM (powered by Quarkus 3.27.3.redhat-00002) started in 17.387s` |
| `sonataflow-platform-jobs-service-7bb4688f8b-w4k28` (**new**) | `jobs-service-postgresql 9.106.0.redhat-00002 on JVM (powered by Quarkus 3.33.2.redhat-00008) started in 14.595s` |

The exact to-version startup strings that must be matched are:
```
data-index-service-postgresql 9.106.0.redhat-00002 on JVM (powered by Quarkus 3.33.2.redhat-00008) started in
jobs-service-postgresql 9.106.0.redhat-00002 on JVM (powered by Quarkus 3.33.2.redhat-00008) started in
```

---

## Tasks

### Task 1 — Add `upgrade.to_kogito_runtime_version` config flag

**Files:** `bddframework/pkg/config/config.go`, `hack/run-tests.sh`

Add field `upgradeToKogitoRuntimeVersion string` to `TestConfig`. Register CLI
flag `--tests.upgrade.to_kogito_runtime_version` in `BindFlags` with default
`"9.106.0.redhat-00002"`. Add getter:

```go
func GetUpgradeToKogitoRuntimeVersion() string {
    return env.upgradeToKogitoRuntimeVersion
}
```

Add `"upgrade.to_kogito_runtime_version"` to `STRING_TEST_PARAMS` in
`hack/run-tests.sh`.

---

### Task 2 — Add `upgrade.to_quarkus_core_version` config flag

**Files:** `bddframework/pkg/config/config.go`, `hack/run-tests.sh`

Add field `upgradeToQuarkusCoreVersion string` to `TestConfig`. Register CLI
flag `--tests.upgrade.to_quarkus_core_version` in `BindFlags` with default
`"3.33.2.redhat-00008"`. Add getter:

```go
func GetUpgradeToQuarkusCoreVersion() string {
    return env.upgradeToQuarkusCoreVersion
}
```

Add `"upgrade.to_quarkus_core_version"` to `STRING_TEST_PARAMS` in
`hack/run-tests.sh`.

---

### Task 3 — Extend `resolveUpgradePlaceholders` + wire into `kubernetes.go`

**Files:** `testbdd/steps/operator.go`, `testbdd/steps/kubernetes.go`

Extend `resolveUpgradePlaceholders` with two new substitutions:

```go
if strings.Contains(s, "${UPGRADE_TO_KOGITO_RUNTIME_VERSION}") {
    s = strings.ReplaceAll(s, "${UPGRADE_TO_KOGITO_RUNTIME_VERSION}", config.GetUpgradeToKogitoRuntimeVersion())
}
if strings.Contains(s, "${UPGRADE_TO_QUARKUS_CORE_VERSION}") {
    s = strings.ReplaceAll(s, "${UPGRADE_TO_QUARKUS_CORE_VERSION}", config.GetUpgradeToQuarkusCoreVersion())
}
```

In `deploymentPodsLogContainsTextWithinMinutes` (`kubernetes.go`) add
`logText = resolveUpgradePlaceholders(logText)` before the framework call so
placeholders in feature file log-check steps are resolved at runtime.

---

### Task 4 — Poll for CSV `Succeeded` phase after from-version install

**File:** `testbdd/steps/operator.go`

Add private helper:

```go
func waitForCSVSucceeded(cli, namespace, csvName string, timeoutInMin int) error
```

Polls `oc get csv <csvName> -n <namespace> -o jsonpath={.status.phase}` via
`framework.WaitForOnOpenshift` until output equals `"Succeeded"`.

Call `waitForCSVSucceeded(cli, operatorNS, fromCSV, 10)` at the end of
`sonataFlowOperatorAtFromVersionIsInstalledViaOLM`, after
`fromInstaller.Install()`. This is a stronger gate than pod readiness — it
confirms OLM has fully reconciled all RBAC/webhook resources.

---

### Task 5 — Poll for CSV `Succeeded` phase after to-version upgrade

**File:** `testbdd/steps/operator.go`

In `sonataFlowOperatorIsUpgradedToNextVersion`, unwrap the final
`return framework.WaitForPodsWithLabel(...)` into a two-step sequence:

1. `framework.WaitForPodsWithLabel(...)` (existing)
2. `waitForCSVSucceeded(cli, operatorNS, targetCSV, 5)` (new)

Prevents the DI/JS log checks from starting before OLM has fully reconciled
the upgraded operator.

---

### Task 6 — Replace DI log check with version-specific startup string

**Files:** `testbdd/features/operator/operator_upgrade.feature`,
`testbdd/steps/kubernetes.go`

Replace:
```gherkin
Then Deployment "sonataflow-platform-data-index-service" pods log contains text 'started in' within 3 minutes
```

With (timeout raised 3 → 5 min — DI took 58s in bdd-7f78):
```gherkin
Then Deployment "sonataflow-platform-data-index-service" pods log contains text 'data-index-service-postgresql ${UPGRADE_TO_KOGITO_RUNTIME_VERSION} on JVM (powered by Quarkus ${UPGRADE_TO_QUARKUS_CORE_VERSION}) started in' within 5 minutes
```

The version-pinned string can only match the new-version pod, not the old one
that coexists during rollout. `resolveUpgradePlaceholders` (Task 3) resolves
the placeholders at runtime.

---

### Task 7 — Replace JS log check with version-specific startup string

**File:** `testbdd/features/operator/operator_upgrade.feature`

Replace:
```gherkin
Then Deployment "sonataflow-platform-jobs-service" pods log contains text 'started in' within 3 minutes
```

With:
```gherkin
Then Deployment "sonataflow-platform-jobs-service" pods log contains text 'jobs-service-postgresql ${UPGRADE_TO_KOGITO_RUNTIME_VERSION} on JVM (powered by Quarkus ${UPGRADE_TO_QUARKUS_CORE_VERSION}) started in' within 3 minutes
```

---

### Task 8 — Update `tasks/upgrade-tests-implementation.md` documentation

Update "Files Changed" table, "How to Run" section, "Post-Upgrade DI/JS Log
Check" design decision, and "Known Limitations" table to document:

- New flags `--tests.upgrade.to_kogito_runtime_version` and
  `--tests.upgrade.to_quarkus_core_version`.
- `waitForCSVSucceeded` CSV-phase gate for both from- and to-version installs.
- Stronger DI/JS version-string log checks replacing the generic `'started in'`.
- Evidence of the old-pod false-positive risk from real run logs.

Updated example run command:

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

---

## Summary Table

| Task | Files | Description | Risk |
|------|-------|-------------|------|
| 1 | `config.go`, `run-tests.sh` | Add `to_kogito_runtime_version` config flag | Low |
| 2 | `config.go`, `run-tests.sh` | Add `to_quarkus_core_version` config flag | Low |
| 3 | `operator.go` (steps), `kubernetes.go` | Placeholder resolution for both new vars + log-step wiring | Low |
| 4 | `operator.go` (steps) | `waitForCSVSucceeded` helper + gate on from-version install | Low |
| 5 | `operator.go` (steps) | CSV `Succeeded` gate on to-version upgrade | Low |
| 6 | `operator_upgrade.feature`, `kubernetes.go` | Replace DI log check with version-specific startup string | Medium |
| 7 | `operator_upgrade.feature` | Replace JS log check with version-specific startup string | Low |
| 8 | `upgrade-tests-implementation.md` | Update documentation | Low |

---

## Implementation Commits

| Commit | Tasks | Description |
|--------|-------|-------------|
| 1 | 1 + 2 + 3 | Config flags + placeholder resolution |
| 2 | 4 + 5 | CSV `Succeeded` gate for from- and to-version |
| 3 | 6 + 7 | Version-specific DI/JS startup log checks |
| 4 | 8 | Documentation update |

---

## Verified Run (bdd-66d8, 2026-08-18) — Tasks 6 + 7 confirmed working

```
from-version : 1.38.1  (redhat-operators)
to-version   : 1.39.0  (IIB)
duration     : 8m46s
result       : PASSED ✅
```

Log evidence that placeholders resolved correctly and matched only the new pods:

```
Wait 5m0s for Pods for deployment 'sonataflow-platform-data-index-service'
  contain text 'data-index-service-postgresql 9.106.0.redhat-00002 on JVM
  (powered by Quarkus 3.33.2.redhat-00008) started in'
→ successful (DI new pod started in 52.996s)

Wait 3m0s for Pods for deployment 'sonataflow-platform-jobs-service'
  contain text 'jobs-service-postgresql 9.106.0.redhat-00002 on JVM
  (powered by Quarkus 3.33.2.redhat-00008) started in'
→ successful (JS new pod started in 9.221s)
```
