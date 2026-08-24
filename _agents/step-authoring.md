# Step Authoring Guide

## Adding a new step

### 1. Register in the correct file

| What the step does | File |
|--------------------|------|
| Operator install / upgrade / ConfigMap in operator NS | `testbdd/steps/operator.go` |
| SonataFlow CR deploy / delete / status | `testbdd/steps/sonataflow.go` |
| SonataFlowPlatform deploy | `testbdd/steps/sonataflowplatform.go` |
| Deployment pod log checks | `testbdd/steps/kubernetes.go` |
| Postgres | `testbdd/steps/postgres.go` |

All step files register via `registerXxxSteps(ctx, data)` called from
`data.RegisterAllSteps()` in `data.go`.

### 2. Step signature pattern

```go
// In register function:
ctx.Step(`^My step "([^"]*)" does something within (\d+) minutes?$`, data.myStepImpl)

// Implementation:
func (data *Data) myStepImpl(name string, timeoutInMin int) error {
    ns := data.Namespace  // workload namespace
    // ...
    return framework.WaitForOnOpenshift(ns, "description", timeoutInMin,
        func() (bool, error) { ... },
    )
}
```

### 3. Namespace rules

| What you're checking | Namespace to use |
|---------------------|-----------------|
| Workload resources (platform, workflows, ConfigMaps owned by the platform) | `data.Namespace` |
| Operator-level resources (builder-config, metrics-service, CSV) | `framework.GetClusterOperatorNamespace()` → `openshift-operators` |
| Operator install namespace | `installers.LogicOperatorNamespace` → `openshift-serverless-logic` |

**Never** use `data.OperatorNamespace` in upgrade steps — it is empty by design
(see `configMapNamespace()` comment in `operator.go`).

### 4. Placeholder resolution

If your step's string argument may contain `${UPGRADE_*}` placeholders, call
`resolveUpgradePlaceholders(s)` before using the string.  
`resolveUpgradePlaceholders` is package-private in `testbdd/steps` — call it
directly from any file in that package.

Supported placeholders → see `upgrade-tests.md`.

To add a new placeholder:
1. Add field + `set.StringVar` + getter to `bddframework/pkg/config/config.go`.
2. Add `"upgrade.new_flag"` to `STRING_TEST_PARAMS` in `hack/run-tests.sh`.
3. Add `if strings.Contains ... ReplaceAll` block to `resolveUpgradePlaceholders`
   in `testbdd/steps/operator.go`.

### 5. Targeting the cluster operator namespace (`openshift-operators`)

For steps that must check resources in `openshift-operators` (e.g. operator-level
ConfigMaps like `logic-operator-builder-config`), **do not** use `configMapNamespace()`
or `data.OperatorNamespace`. Instead, hardwire to `framework.GetClusterOperatorNamespace()`:

```go
func (data *Data) configMapExistsInClusterOperatorNamespace(cmName string) error {
    ns := framework.GetClusterOperatorNamespace()   // "openshift-operators" on OCP
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

Register a distinct Gherkin step name that makes the target explicit:
```
^ConfigMap "([^"]*)" exists in cluster operator namespace$
^ConfigMap "([^"]*)" in cluster operator namespace contains following strings:$
```

This avoids confusion with the existing `ConfigMap "X" exists` step which resolves
to the workload namespace (`data.Namespace`).

### 6. YAML template substitution in Go (gitops pattern)

When a YAML file contains a placeholder that must be resolved before `oc apply`
(e.g. `${UPGRADE_GITOPS_IMAGE_VERSION}`), substitute in the step itself:

```go
func (data *Data) myGitopsStep(version string) error {
    projectDir, _ := utils.GetProjectDir()
    projectDir = strings.Replace(projectDir, "/testbdd", "", -1)
    yamlBytes, err := os.ReadFile(filepath.Join(projectDir, test.GetMyYAML()))
    if err != nil {
        return err
    }
    resolved := strings.ReplaceAll(string(yamlBytes), "${MY_PLACEHOLDER}", version)
    tmp, err := os.CreateTemp("", "sonataflow-*.yaml")
    if err != nil {
        return err
    }
    defer os.Remove(tmp.Name())
    if _, err := tmp.WriteString(resolved); err != nil {
        return err
    }
    tmp.Close()
    out, err := framework.CreateCommand("oc", "apply", "-f", tmp.Name(), "-n", data.Namespace).Execute()
    if err != nil {
        return fmt.Errorf("oc apply failed: %w — output: %s", err, out)
    }
    return nil
}
```

### 7. Adding a YAML path accessor

In `test/yaml.go`:
```go
const myWorkflowYaml = "my-folder/my_workflow.yaml"  // relative to e2eSamples (test/testdata/)

func GetMyWorkflow() string {
    return e2eSamples + myWorkflowYaml
}
```

### 8. Waiting patterns

```go
// Poll until condition:
framework.WaitForOnOpenshift(ns, "display name", timeoutInMin, func() (bool, error) { ... })

// Wait for deployment ready:
framework.WaitForDeploymentRunning(ns, "deployment-name", podCount, timeoutInMin)

// Wait for pods by label:
framework.WaitForPodsWithLabel(ns, "label-key", "label-value", count, timeoutInMin)

// Scan pod logs (any pod matches):
framework.WaitForAnyPodsByDeploymentToContainTextInLog(ns, dName, containerName, text, timeout)

// Scan pod logs (all pods match):
framework.WaitForAllPodsByDeploymentToContainTextInLog(ns, dName, containerName, text, timeout)
```

Container name for platform services: strip `sonataflow-platform-` prefix
(`deploymentContainerName()` in `kubernetes.go`).

### 9. Running oc/kubectl commands

```go
cli := "kubectl"
if framework.IsOpenshift() {
    cli = "oc"
}
out, err := framework.CreateCommand(cli, "get", "csv", name, "-n", ns,
    "-o", "jsonpath={.status.phase}").Execute()
```

### 10. Deleting a SonataFlow CR (and waiting for it to vanish)

`sonataFlowIsDeleted` (in `sonataflow.go`) now waits up to 2 minutes for the CR
to fully disappear from the API server before returning. This is required when the
next step applies a YAML to the same CR name — `oc apply` against a still-terminating
resource races and fails with "not found" errors during patch.

Step: `When SonataFlow "callbackstatetimeouts" is deleted`

### 11. BeforeSuite operator install bypass

`scenarioManagesOperatorInstall()` in `bdd_executor.go` returns `true` when
`--tests.upgrade.from_version` OR `--tests.operator.version` is set. In that case
`BeforeSuite` skips `installGlobalSonataFlowOperator()` and only sets up the catalog
source. This prevents a community-catalog install racing against the OLM steps in
the scenario.
