# Key Constants and Configuration

## Operator namespaces and names

| Constant / Value | Location | Meaning |
|------------------|----------|---------|
| `installers.LogicOperatorNamespace` = `"openshift-serverless-logic"` | `sonataflow_installer.go:85` | Namespace where OSL operator pod runs |
| `installers.LogicOperatorSubscriptionName` = `"logic-operator"` | `sonataflow_installer.go:86` | OLM Subscription / CSV package name |
| `installers.LogicOperatorSubscriptionChannel` = `"stable"` | `sonataflow_installer.go:87` | OLM channel |
| `installers.LogicOperatorDeploymentName` = `"logic-operator-controller-manager"` | `sonataflow_installer.go:88` | Operator Deployment name |
| `framework.GetClusterOperatorNamespace()` → `"openshift-operators"` | `operator.go:511` | Subscription, InstallPlan, operator pod (OLM cluster-wide) |
| `framework.GetCustomKogitoOperatorCatalog().Source()` → `"bdd-tests-kogito-catalog"` | `operator.go` | Custom IIB CatalogSource name |

## ConfigMap names (workload namespace — `data.Namespace`)

| ConfigMap | Namespace | Who creates it |
|-----------|-----------|----------------|
| `callbackstatetimeouts-managed-props` | `data.Namespace` | Operator (after workflow reaches Running) |
| `sonataflow-platform-jobs-service-props` | `data.Namespace` | Operator (platform reconciliation) |
| `sonataflow-platform-data-index-service-props` | `data.Namespace` | Operator (platform reconciliation) |

## ConfigMap names (cluster operator namespace — `openshift-operators`)

| ConfigMap | Namespace | Who creates it |
|-----------|-----------|----------------|
| `logic-operator-builder-config` | `openshift-operators` | OLM (from CSV) — contains builder Dockerfile template |
| `logic-operator-controllers-config` | `openshift-operators` | OLM (from CSV) |

Steps targeting these must use `framework.GetClusterOperatorNamespace()` directly —
**not** `configMapNamespace()` or `data.OperatorNamespace`. See Task 9 steps
`ConfigMap "X" exists in cluster operator namespace` /
`ConfigMap "X" in cluster operator namespace contains following strings:`.

## CLI flags — upgrade testing

All flags prefixed `--tests.upgrade.*`:

| Flag | Default | Getter |
|------|---------|--------|
| `upgrade.from_version` | `""` | `config.GetUpgradeFromVersion()` |
| `upgrade.to_version` | `""` | `config.GetUpgradeToVersion()` |
| `upgrade.to_kogito_runtime_version` | `"9.106.0.redhat-00002"` | `config.GetUpgradeToKogitoRuntimeVersion()` |
| `upgrade.to_quarkus_core_version` | `"3.33.2.redhat-00008"` | `config.GetUpgradeToQuarkusCoreVersion()` |

## CLI flags — non-upgrade OLM install

| Flag | Default | Getter |
|------|---------|--------|
| `operator.version` | `""` | `config.GetOperatorVersion()` |
| `operator_installation_source` | `""` | `config.GetOperatorInstallationSource()` |
| `operator_catalog_image` | `""` | `config.GetOperatorCatalogImage()` |

`operator.version` is used by `SonataFlow Operator is installed via OLM` (platform tests).
`upgrade.from_version` / `upgrade.to_version` are used by the upgrade scenario.
When either `upgrade.from_version` or `operator.version` is set, `BeforeSuite` skips
the global operator install.

All flags are also in `STRING_TEST_PARAMS` in `hack/run-tests.sh`.

## CSV naming pattern

```
logic-operator.v<version>
e.g. logic-operator.v1.39.0
```

Built as: `fmt.Sprintf("%s.v%s", installers.LogicOperatorSubscriptionName, version)`

OLM stamps `olm.owner: logic-operator.v<version>` label on every managed
Deployment — used by `sonataFlowOperatorRunningVersionMatchesUpgradeTarget`.

## DI / JS deployment names (workload namespace)

| Service | Deployment name | Container name |
|---------|----------------|----------------|
| Data Index | `sonataflow-platform-data-index-service` | `data-index-service` |
| Job Service | `sonataflow-platform-jobs-service` | `jobs-service` |

Container name derived by `deploymentContainerName()`: strip `sonataflow-platform-` prefix.

## Workflow profile annotations

| Profile | Annotation value | Build managed by operator? | Delete before upgrade? |
|---------|-----------------|---------------------------|----------------------|
| dev | `sonataflow.org/profile: dev` (default) | Yes (live reload) | No |
| preview | `sonataflow.org/profile: preview` | Yes (Buildah in-cluster) | **Yes** |
| gitops | `sonataflow.org/profile: gitops` | **No** — image in `spec.podTemplate.container.image` | **Yes** (rebuild + retag first) |

## Test data YAML paths (relative to `test/testdata/`)

| Accessor in `test/yaml.go` | Path |
|---------------------------|------|
| `GetSonataFlowE2eCallbackstateTimeoutsFolder()` | `callbackstate-timeouts/` |
| `GetSonataFlowCallbackstateTimeoutsGitops()` | `callbackstate-timeouts/gitops/callbackstatetimeouts_gitops.yaml` |
| `GetSFPlatformWithDIandJSWithDBMigration()` | `sonataflow/platform/.../sonataflow_platform_with_postgresql_dataindex_and_job_service_db_migration.yaml` |
| `GetSFPlatformWithDIandJSUsingPostgres()` | `sonataflow/platform/.../sonataflow_platform_with_postgresql_dataindex_and_job_service.yaml` |
| `GetSonataFlowE2eGreetingFolder()` | `greeting/` |
| `GetPostgresFolder()` | `persistence/postgres/` |

## Godog tags

| Tag | Meaning |
|-----|---------|
| `@upgradeTests` | Upgrade scenario — excluded from all normal runs; must be passed explicitly |
| `@platform` | Platform-level tests (custom labels, ConfigMap props, etc.) |
| `@Smoke` / `@smoke` | Smoke subset |
| `@devMode` / `@previewMode` / `@gitOpsMode` | Profile-specific scenarios |
| `@disabled` | Always skipped |
| `@performance` | Performance tests — excluded unless explicitly requested |
