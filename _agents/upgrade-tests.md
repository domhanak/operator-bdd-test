# Upgrade Tests — Current State and Planned Work

## Branch
`SRVLOGIC-1138-migration-tests`

## Feature file
`testbdd/features/operator/operator_upgrade.feature`  
Tag: `@upgradeTests` — **excluded from all normal CI runs** by default.

## Current scenario flow (as implemented on this branch)

```
Given  SonataFlow Operator at from-version is installed via OLM
Given  Namespace is created
When   Postgres is deployed
When   SonataFlowPlatform with DataIndexAndJobsService using Postgres and DB migration strategy is deployed
When   SonataFlow callbackstatetimeouts example is deployed         ← preview profile
Then   SonataFlow "callbackstatetimeouts" Running within 20 minutes
When   SonataFlow "callbackstatetimeouts" is deleted                ← name conflict avoidance: preview→gitops
When   SonataFlow callbackstatetimeouts gitops at from-version is deployed   ← gitops profile (Task 10)
Then   SonataFlow "callbackstatetimeouts" Running within 5 minutes
When   SonataFlow "callbackstatetimeouts" is deleted                ← pre-upgrade gitops delete (guide §11.6.3)
Then   Service "sonataflow-platform-data-index-service" exists
Then   Service "sonataflow-platform-jobs-service" exists
When   SonataFlow Operator is upgraded to next version
Then   SonataFlow Operator running version matches upgrade target
Then   Deployment "sonataflow-platform-data-index-service" pods log contains text
         'data-index-service-postgresql ${UPGRADE_TO_KOGITO_RUNTIME_VERSION} on JVM (powered by Quarkus ${UPGRADE_TO_QUARKUS_CORE_VERSION}) started in'
         within 5 minutes
Then   Deployment "sonataflow-platform-jobs-service" pods log contains text
         'jobs-service-postgresql ${UPGRADE_TO_KOGITO_RUNTIME_VERSION} on JVM (powered by Quarkus ${UPGRADE_TO_QUARKUS_CORE_VERSION}) started in'
         within 3 minutes
When   SonataFlow callbackstatetimeouts gitops at to-version is redeployed  ← gitops redeploy (Task 10)
Then   SonataFlow "callbackstatetimeouts" Running within 5 minutes
Then   ConfigMap "callbackstatetimeouts-managed-props" exists + contains strings
When   SonataFlow "callbackstatetimeouts" is deleted                ← explicit cleanup before preview redeploy (Task 10)
Then   SonataFlow callbackstatetimeouts example is deployed         ← preview redeploy
Then   SonataFlow "callbackstatetimeouts" Running within 5 minutes
Then   ConfigMap "callbackstatetimeouts-managed-props" exists + contains strings
Then   ConfigMap "sonataflow-platform-jobs-service-props" exists + contains strings
```

## How to run

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

## Key implementation files

| File | Role |
|------|------|
| `testbdd/steps/operator.go` | All upgrade step implementations + `resolveUpgradePlaceholders` |
| `testbdd/steps/kubernetes.go` | `deploymentPodsLogContainsTextWithinMinutes` — calls `resolveUpgradePlaceholders` |
| `testbdd/steps/sonataflow.go` | `sonataFlowIsDeleted` (waits for CR to vanish), gitops deploy steps |
| `testbdd/steps/sonataflowplatform.go` | Platform deploy + waits for DI/JS deployments |
| `bddframework/pkg/config/config.go` | All `--tests.upgrade.*` and `--tests.operator.*` flags |
| `testbdd/installers/sonataflow_installer.go` | `LogicOperatorNamespace`, `LogicOperatorSubscriptionName`, exported constants |
| `testbdd/executor/bdd_executor.go` | `scenarioManagesOperatorInstall()` helper — skips global install for upgrade/platform-OLM scenarios |

## Namespaces used by the upgrade scenario

| Resource | Namespace |
|----------|-----------|
| Operator pod, CSV | `openshift-operators` (`framework.GetClusterOperatorNamespace()`) |
| Operator install (OLM) | `openshift-serverless-logic` (`installers.LogicOperatorNamespace`) |
| Subscription, InstallPlan | `openshift-operators` |
| Workload (platform, workflows) | `bdd-XXXX` (randomly generated per scenario, `data.Namespace`) |
| `logic-operator-builder-config` ConfigMap | `openshift-operators` |

**Critical:** `configMapNamespace()` in `operator.go` returns `data.OperatorNamespace`
(only set by YAML install path) or `data.Namespace` (workload namespace). In the
upgrade scenario `data.OperatorNamespace` is intentionally left empty so all
platform/workflow resource checks resolve to the workload namespace.  
New steps targeting `openshift-operators` must use `framework.GetClusterOperatorNamespace()`
directly — see the `...in cluster operator namespace` step pattern (Task 9).

## OLM upgrade mechanics

1. From-version installs from `redhat-operators` (catalog: `GetProductCatalog()`).
   - Always uses product catalog — from-version is a GA release.
   - After install: `waitForCSVSucceeded` with 10 min timeout.
2. To-version: patch subscription `source` → `bdd-tests-kogito-catalog` (`GetCustomKogitoOperatorCatalog()`),
   `startingCSV` → `logic-operator.v<toVersion>`.
3. Find pending InstallPlan containing target CSV → approve it.
4. Wait for pod ready (`WaitForPodsWithLabel`) then CSV `Succeeded` (`waitForCSVSucceeded`, 5 min).

## `resolveUpgradePlaceholders` — supported placeholders

| Placeholder | Config flag | Default |
|-------------|-------------|---------|
| `${UPGRADE_FROM_VERSION}` | `--tests.upgrade.from_version` | — |
| `${UPGRADE_TO_VERSION}` | `--tests.upgrade.to_version` | — |
| `${UPGRADE_TO_KOGITO_RUNTIME_VERSION}` | `--tests.upgrade.to_kogito_runtime_version` | `9.106.0.redhat-00002` |
| `${UPGRADE_TO_QUARKUS_CORE_VERSION}` | `--tests.upgrade.to_quarkus_core_version` | `3.33.2.redhat-00008` |

Called automatically in:
- `configMapContainsStrings` (operator.go)
- `configMapInClusterOperatorNamespaceContainsStrings` (operator.go) ← Task 9b
- `deploymentPodsLogContainsTextWithinMinutes` (kubernetes.go)

The gitops YAML template `${UPGRADE_GITOPS_IMAGE_VERSION}` is **not** a feature-file
placeholder — it is substituted in Go inside the step implementation before `oc apply`
(see `applyCallbackstateTimeoutsGitops` in `sonataflow.go`).

## GitOps workflow migration (Task 10 — implemented)

**YAML:** `test/testdata/callbackstate-timeouts/gitops/callbackstatetimeouts_gitops.yaml`  
Contains `${UPGRADE_GITOPS_IMAGE_VERSION}` placeholder in `spec.podTemplate.container.image`.

**Steps:**
- `SonataFlow callbackstatetimeouts gitops at from-version is deployed`
  → substitutes `config.GetUpgradeFromVersion()`, runs `oc apply`
- `SonataFlow callbackstatetimeouts gitops at to-version is redeployed`
  → substitutes `config.GetUpgradeToVersion()`, runs `oc apply`

**CR name collision:** Both the preview and gitops CRs share `name: callbackstatetimeouts`.
Sequence in feature file avoids conflicts:
1. preview deploy → Running
2. `SonataFlow "callbackstatetimeouts" is deleted` (wait for CR to vanish — required!)
3. gitops deploy (from-version) → Running
4. `SonataFlow "callbackstatetimeouts" is deleted` (pre-upgrade)
5. … upgrade …
6. gitops redeploy (to-version) → Running + ConfigMap check
7. `SonataFlow "callbackstatetimeouts" is deleted`
8. preview redeploy → Running + ConfigMap check

`sonataFlowIsDeleted` waits up to 2 minutes for the CR to fully disappear before
returning — prevents `oc apply` racing against a terminating resource.

## Planned tasks status

| Task | Description | Status |
|------|-------------|--------|
| 1–8 | Config flags, CSV gate, DI/JS version log checks | ✅ Implemented |
| 9 | `logic-operator-builder-config` ConfigMap check before/after upgrade | 🟡 Planned (see `tasks/upgrade-tests-improvement-plan-task-9-implementation.md`) |
| 10 | GitOps workflow migration (deploy + delete pre-upgrade; redeploy + verify post-upgrade) | ✅ Implemented |

## Verified runs

| Run | from→to | Duration | Result |
|-----|---------|----------|--------|
| bdd-7f78 | 1.38.1→1.39.0 | 7m02s | ✅ |
| bdd-1183 | 1.38.1→1.39.0 | ~5m | ✅ |
| bdd-66d8 | 1.38.1→1.39.0 | 8m46s | ✅ (with Tasks 1–8 DI/JS version checks) |
| bdd-391a | 1.38.1→1.39.0 | 9m17s | ✅ (with Task 10 gitops steps) |

### DI/JS startup log pattern (bdd-66d8 / bdd-391a, confirmed)
```
data-index-service-postgresql 9.106.0.redhat-00002 on JVM (powered by Quarkus 3.33.2.redhat-00008) started in
jobs-service-postgresql       9.106.0.redhat-00002 on JVM (powered by Quarkus 3.33.2.redhat-00008) started in
```
Old pods (from-version) logged `9.105.0.redhat-00003 / Quarkus 3.27.3.redhat-00002` —
the generic `'started in'` check would have matched them first (false positive).
The version-pinned check was added in Task 6/7 to avoid this race.
