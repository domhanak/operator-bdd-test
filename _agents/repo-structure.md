# Repository Structure

## Top-level

```
_agents/                        agent context files (this folder)
bddframework/                   reusable framework Go module
  pkg/
    config/config.go            all CLI flags (--tests.*); getters for every param
    framework/
      operator.go               OLM helpers: CSV, Subscription, CatalogSource, WaitForOnOpenshift
      kubernetes.go             pod/deployment helpers, WaitFor*, log scanning, IsConfigMapExist
      openshift.go              OpenShift-specific resources
    installers/                 OlmClusterWideServiceInstaller, YamlClusterWideServiceInstaller
    steps/                      base Gherkin steps (namespace, postgres, kogito framework)
hack/
  run-tests.sh                  entry-point: maps Make vars → --tests.* flags; run with go test
tasks/
  upgrade-tests-implementation.md              original implementation doc (Tasks 0, base scenario)
  upgrade-tests-improvement-plan-tasks-1-8.md  improvement plan Tasks 1–8 (implemented)
  upgrade-tests-improvement-plan-task-9.md     Task 9 original planning doc
  upgrade-tests-improvement-plan-task-9-implementation.md  Task 9 ready-to-implement spec ← authoritative
  upgrade-tests-improvement-plan-task-10.md    Task 10 plan (implemented)
test/
  yaml.go                       path accessors for all test-data YAMLs (GetSonataFlow*())
  testdata/                     all YAML fixtures
    callbackstate-timeouts/
      01_sonataflow.org_v1alpha08_sonataflow_callbackstatetimeouts.yaml   preview profile
      gitops/
        callbackstatetimeouts_gitops.yaml   gitops profile, ${UPGRADE_GITOPS_IMAGE_VERSION}
    greeting/
      01_sonataflow.org_v1alpha08_sonataflow_greeting.yaml                dev profile
      gitops/
        01_sonataflow.org_v1alpha08_sonataflow_greeting.yaml             gitops profile (personal image)
    persistence/postgres/       Postgres deployment YAML
    sonataflow/platform/sonataflow.org_v1alpha08/
      sonataflow_platform_with_postgresql_dataindex_and_job_service.yaml          (labels: osl.logic.com/securityZone: api)
      sonataflow_platform_with_postgresql_dataindex_and_job_service_db_migration.yaml
testbdd/
  executor/bdd_executor.go      godog suite wiring; scenarioManagesOperatorInstall() bypass; tag exclusions
  features/                     .feature files (Gherkin scenarios)
    operator/
      operator_upgrade.feature                             @upgradeTests — THE upgrade scenario
    installation/
      install_logic_operator_yaml.feature                  YAML install path
    sonataflow/platform/
      platform_with_data_index_and_job_service.feature     DI+JS platform smoke
      platform_configmap_properties.feature                ConfigMap props checks  @platform
      platform_service_custom_labels.feature               Label propagation test  @platform
      plaform_default.feature                              default platform
    sonataflow/clusterplatform/
      deploy_greeting_example.feature
    sonataflow/performance/
      deploy_100_workflows_with_di_js_postgresql.feature   @performance
    data-index/
      deploy_data_index_service.feature
    job-service/
      deploy_job_service.feature
  installers/sonataflow_installer.go       OLM + YAML installers; all exported name constants
  steps/
    data.go                     Data struct (embeds framework Data + OperatorNamespace)
    operator.go                 upgrade/OLM steps, ConfigMap steps, resolveUpgradePlaceholders
                                  also: configMapExists, configMapContainsStrings (workload NS)
                                  Task 9 will add: configMapExistsInClusterOperatorNamespace,
                                                   configMapInClusterOperatorNamespaceContainsStrings
    kubernetes.go               Deployment log-check step (resolves placeholders before scan)
                                  deploymentHasLabelWithValue, deploymentPodsHaveLabelWithValue
    sonataflow.go               SonataFlow CRUD steps; applyCallbackstateTimeoutsGitops
    sonataflowplatform.go       SonataFlowPlatform deploy steps
    postgres.go                 Postgres deploy step
```

## Module boundaries

Two Go modules, managed by a single `go.work`:

| Module | Path | Imports |
|--------|------|---------|
| `bddframework` | `bddframework/` | external only |
| `testbdd` | `testbdd/` + `test/` | imports `bddframework` |

Vet each module independently:
```bash
cd bddframework && go vet ./...
cd testbdd      && go vet ./...
```
