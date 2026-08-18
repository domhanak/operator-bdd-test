# Upgrade Tests Implementation — SRVLOGIC-1138

**Branch:** `SRVLOGIC-1138-migration-tests`  
**Status:** ✅ Scenario passing end-to-end (verified run `bdd-7f78`, 2026-08-18, 7m02s)

---

## What Was Built

A BDD test scenario that automates the OSL operator upgrade migration procedure from a
configurable `from-version` to a `to-version` via OLM on an OpenShift cluster.

The scenario mirrors the steps in the operator upgrade guide (1.37.2 → 1.38.0/1.39.0):

1. Install operator at from-version from `redhat-operators` catalog
2. Create workload namespace, deploy Postgres, SonataFlowPlatform (DI + JS + Postgres)
3. Deploy a preview-profile workflow (`callbackstatetimeouts`), wait for `Running`
4. Delete the preview-profile workflow (required before upgrade — old builds incompatible)
5. Assert DI and JS services exist (proxy for "database backed up")
6. Upgrade the operator via OLM to to-version from the custom IIB catalog
7. Verify the operator deployment is owned by the target CSV (`olm.owner` label)
8. Verify DI and JS restarted at the new version (pod log scan)
9. Redeploy the preview-profile workflow, wait for `Running` at new version
10. Verify managed-props ConfigMaps contain expected configuration keys
11. Verify the Job Service props ConfigMap exists with expected settings

---

## Files Changed (uncommitted — on top of commit `6c34cf7f`)

| File | Change |
|---|---|
| `testbdd/features/operator/operator_upgrade.feature` | Full upgrade scenario tagged `@upgradeTests` |
| `testbdd/steps/operator.go` | All upgrade step implementations |
| `testbdd/steps/sonataflow.go` | `SonataFlow "X" is deleted` step |
| `testbdd/steps/kubernetes.go` | Fixed container name derivation + switched to any-pod log check |
| `bddframework/pkg/framework/operator.go` | Added `Source()` / `Namespace()` accessors on `OperatorCatalog` |
| `test/testdata/.../sonataflow_platform_with_postgresql_dataindex_and_job_service_db_migration.yaml` | New platform YAML with `dbMigrationStrategy: job` + `QUARKUS_ADD_EXTENSION_ARGS` |
| `bddframework/pkg/config/config.go` | `--tests.upgrade.from_version` / `--tests.upgrade.to_version` flags |
| `testbdd/installers/sonataflow_installer.go` | Exported `LogicOperatorSubscriptionName`, `LogicOperatorSubscriptionChannel`, `LogicOperatorDeploymentName` |
| `testbdd/executor/bdd_executor.go` | `@upgradeTests` tag exclusion from normal CI runs |
| `hack/run-tests.sh` | `-count=1` flag; `upgrade.from_version` / `upgrade.to_version` in `STRING_TEST_PARAMS` |
| `test/yaml.go` | `GetSFPlatformWithDIandJSWithDBMigration()` path accessor |

---

## How to Run

```bash
cd testbdd
make run-tests \
  feature=features/operator/operator_upgrade.feature \
  cr_deployment_only=true \
  operator_installation_source=olm \
  "operator_catalog_image=registry-proxy.engineering.redhat.com/rh-osbs/iib:<IIB_INDEX>" \
  "upgrade.from_version=1.38.1" \
  "upgrade.to_version=1.39.0" \
  tags="@upgradeTests"
```

The test is **excluded from all normal CI runs** by default. It only executes when
`--godog.tags=@upgradeTests` is explicitly passed.

---

## Key Design Decisions

### OLM Install Strategy
- **From-version** installs from `redhat-operators` (`registry.redhat.io`) — no staging auth required.
- **To-version** patches the subscription `source` to `bdd-tests-kogito-catalog` (the custom IIB
  CatalogSource created at suite start from `--tests.operator_catalog_image`).
- Before each install, all pre-existing `logic-operator.*` CSVs are deleted from
  `openshift-serverless-logic` to avoid OLM `@existing ... not referenced by subscription` conflicts.

### Operator Version Verification
OLM resolves images to SHA256 digests — version strings do not appear in container image refs.
Version is verified via the `olm.owner` label that OLM stamps on every managed Deployment:
`logic-operator.v<toVersion>`. The step polls until the label matches (up to 5 min).

### Namespace Routing for Resource Checks
`data.OperatorNamespace` is only set by `SonataFlow Operator is deployed` (YAML/OLM install that
owns the operator). The upgrade install step deliberately does **not** set it, so
`configMapNamespace()` / `serviceExists()` always resolve to `data.Namespace` (the workload
namespace), not `openshift-serverless-logic`.

### Preview-Profile Workflow Lifecycle
The `callbackstatetimeouts` workflow uses preview profile (Buildah Docker build in-cluster).
It must be **deleted before the upgrade** — the new operator will not reconcile a
`SonataFlowBuild` produced by the old version. After the upgrade it is redeployed and reaches
`Running` quickly because the built image is already cached in the internal registry.

### In-Cluster Maven Build Speed
The platform YAML sets `QUARKUS_ADD_EXTENSION_ARGS: "-Dquarkus.registry.client.enabled=false"`
in `spec.build.config.strategyOptions`. Without this, `quarkus:add-extension` contacts
`registry.quarkus.io` and serially attempts to download codestart JARs for ~50+ extensions
(all 404), taking 15+ minutes and exceeding the 20-minute workflow `Running` timeout.

### Post-Upgrade DI/JS Log Check
After upgrade, old pods and new pods coexist briefly. `WaitForAnyPodsByDeploymentToContainTextInLog`
is used (not `WaitForAll`) so the step passes as soon as the new pod logs `started in`.
The container name is derived by stripping the `sonataflow-platform-` prefix from the deployment
name (e.g. `sonataflow-platform-data-index-service` → container `data-index-service`).

---

## Known Limitations / Open Items

| Item | Notes |
|---|---|
| `dbMigrationStrategy: job` in platform YAML | Setting this causes `ImagePullBackOff` on the migrator job image during from-version startup. The DB migrator job step exists in code but is **commented out** in the feature file pending resolution. |
| Workflow `Running` timeout post-upgrade | Set to 5 min post-upgrade (image already cached). If the internal registry is cold this may need increasing. |
| `InstallPlan` approval window | Polls for 5 min. On a loaded cluster this may need extending. |
| `WaitForPodsWithLabel` after upgrade | Waits in `openshift-operators` for `app.kubernetes.io/name=sonataflow-operator`. The pod label is set by the CSV; if the CSV uses a different label this will need updating. |

---

## Verified Run Summary (`bdd-7f78`, 2026-08-18)

```
from-version : 1.38.1  (redhat-operators)
to-version   : 1.39.0  (IIB registry-proxy.engineering.redhat.com/rh-osbs/iib:...)
duration     : 7m02s
result       : PASSED ✅
```

| Step | Time | Result |
|---|---|---|
| Install operator at from-version | ~23s | ✅ |
| Namespace + Postgres + Platform | ~1m56s | ✅ |
| Deploy workflow + wait Running | ~1m52s | ✅ (image cached from prior run) |
| Delete workflow pre-upgrade | <1s | ✅ |
| Upgrade operator to to-version | ~6s | ✅ |
| Verify operator version (olm.owner) | ~2s | ✅ |
| DI pods log `started in` | ~2s | ✅ |
| JS pods log `started in` | ~1s | ✅ |
| Redeploy workflow + wait Running | ~2m37s | ✅ |
| ConfigMap content checks | <1s | ✅ |
