# Upgrade Tests Improvement Plan — Task 10: GitOps Workflow Migration

**Branch:** `SRVLOGIC-1138-migration-tests`  
**Status:** 🟡 Planning — awaiting approval before implementation  
**Depends on:** `tasks/upgrade-tests-implementation.md` (base scenario passing)

---

## Goal

Extend the upgrade scenario to cover the **GitOps-profile workflow migration
path** as described in the official OSL upgrade guide (§11.6.3 / §11.6.3.3).

**Pre-upgrade (§11.6.3):**
1. Deploy the gitops workflow with the from-version pre-built image, verify `Running`.
2. Delete the gitops workflow CR before the operator upgrade.

**Post-upgrade (§11.6.3.3):**
1. Redeploy the gitops workflow CR referencing the **to-version pre-built image**
   (new tag → cluster pulls the rebuilt image, not the cached one).
2. Verify it reaches `Running` and that the operator reconciled the
   `callbackstatetimeouts-managed-props` ConfigMap.

---

## Design: single YAML, one placeholder

**File:** `test/testdata/callbackstate-timeouts/gitops/callbackstatetimeouts_gitops.yaml`
(already created)

```yaml
metadata:
  name: callbackstatetimeouts
  annotations:
    sonataflow.org/profile: gitops
spec:
  podTemplate:
    container:
      image: quay.io/dhanak/callbackstatetimeouts:${UPGRADE_GITOPS_IMAGE_VERSION}
```

The BDD step substitutes `${UPGRADE_GITOPS_IMAGE_VERSION}` in memory before
`oc apply`:
- Pre-upgrade deploy   → `config.GetUpgradeFromVersion()` (e.g. `1.38.1`)
- Post-upgrade redeploy → `config.GetUpgradeToVersion()` (e.g. `1.39.0`)

Different tags guarantee a fresh image pull on redeploy.

### CR name — why the delete before gitops redeploy is necessary

Both the preview YAML and the gitops YAML use `name: callbackstatetimeouts`.

The existing `When SonataFlow "callbackstatetimeouts" is deleted` step (pre-upgrade)
removes whichever CR exists at that point. After the upgrade, the operator has
no `callbackstatetimeouts` CR. The gitops redeploy step creates a fresh one.

However, in the **post-upgrade** phase the scenario later redeployes the
preview variant of the same CR. If the gitops CR is still alive when the
preview YAML is applied, `oc apply` would patch the profile back from `gitops`
to `preview` — the operator would then attempt to start a new Buildah build
from scratch which takes 20 minutes and is undesirable.

Therefore the scenario must **explicitly delete the gitops CR** after verifying
it, before redeploying the preview variant. This delete step is added in Task 10d.

---

## Tasks

### Task 10a — GitOps workflow YAML ✅ already created

`test/testdata/callbackstate-timeouts/gitops/callbackstatetimeouts_gitops.yaml`

No further changes needed.

---

### Task 10b — Add path accessor in `test/yaml.go`

**File:** `test/yaml.go`

```go
const sonataFlowCallbackstateTimeoutsGitopsWorkflow = "callbackstate-timeouts/gitops/callbackstatetimeouts_gitops.yaml"

func GetSonataFlowCallbackstateTimeoutsGitops() string {
    return e2eSamples + sonataFlowCallbackstateTimeoutsGitopsWorkflow
}
```

---

### Task 10c — Two new Gherkin steps in `testbdd/steps/sonataflow.go`

**File:** `testbdd/steps/sonataflow.go`

Register:

```go
ctx.Step(`^SonataFlow callbackstatetimeouts gitops at from-version is deployed$`,
    data.sonataFlowCallbackstateTimeoutsGitopsAtFromVersionIsDeployed)
ctx.Step(`^SonataFlow callbackstatetimeouts gitops at to-version is redeployed$`,
    data.sonataFlowCallbackstateTimeoutsGitopsAtToVersionIsRedeployed)
```

Both implementations:
1. Read the YAML template bytes from `test.GetSonataFlowCallbackstateTimeoutsGitops()`.
2. Replace `${UPGRADE_GITOPS_IMAGE_VERSION}` with `config.GetUpgradeFromVersion()`
   or `config.GetUpgradeToVersion()` respectively.
3. Write the resolved content to a `os.CreateTemp` file.
4. Run `oc apply -f <tempfile> -n data.Namespace`.
5. Remove the temp file.

---

### Task 10d — Extend the upgrade feature scenario

**File:** `testbdd/features/operator/operator_upgrade.feature`

The full annotated diff of what changes:

```gherkin
    # ── existing ──────────────────────────────────────────────────────────────
    When SonataFlow callbackstatetimeouts example is deployed
    Then SonataFlow "callbackstatetimeouts" has the condition "Running" set to "True" within 20 minutes

    # ── NEW: GitOps profile — deploy at from-version (guide §11.6.3) ─────────
    # oc apply patches the existing preview CR in-place: adds gitops profile
    # annotation and spec.podTemplate with the from-version pre-built image.
    # Verifying Running confirms the image is accessible and the from-version
    # operator reconciles gitops workflows correctly.
    When SonataFlow callbackstatetimeouts gitops at from-version is deployed
    Then SonataFlow "callbackstatetimeouts" has the condition "Running" set to "True" within 5 minutes

    # ── existing — now covers gitops CR (profile was just patched to gitops) ──
    # Guide §11.6.3 step 7: delete the gitops workflow before upgrading.
    When SonataFlow "callbackstatetimeouts" is deleted

    # ── existing ──────────────────────────────────────────────────────────────
    Then Service "sonataflow-platform-data-index-service" exists
    Then Service "sonataflow-platform-jobs-service" exists
    When SonataFlow Operator is upgraded to next version
    Then SonataFlow Operator running version matches upgrade target
    Then Deployment "sonataflow-platform-data-index-service" pods log contains text 'started in' within 3 minutes
    Then Deployment "sonataflow-platform-jobs-service" pods log contains text 'started in' within 3 minutes

    # ── NEW: GitOps profile — redeploy at to-version (guide §11.6.3.3) ───────
    # Redeploy with the image rebuilt using the to-version OSL builder.
    # The new image tag forces the cluster to pull the rebuilt image rather
    # than reusing the cached from-version image (imagePullPolicy: IfNotPresent).
    When SonataFlow callbackstatetimeouts gitops at to-version is redeployed
    Then SonataFlow "callbackstatetimeouts" has the condition "Running" set to "True" within 5 minutes
    Then ConfigMap "callbackstatetimeouts-managed-props" exists
    Then ConfigMap "callbackstatetimeouts-managed-props" contains following strings:
      | kogito.data-index.health-enabled = true    |
      | kogito.data-index.url                      |
      | kogito.jobs-service.health-enabled = true  |
      | kogito.jobs-service.url                    |
      | quarkus.http.port = 8080                   |
      | quarkus.devservices.enabled = false        |
      | quarkus.kogito.devservices.enabled = false |

    # ── NEW: delete gitops CR before redeploying preview variant ─────────────
    # The preview redeploy step below applies the preview-profile YAML to the
    # same CR name "callbackstatetimeouts". Without this delete the existing
    # gitops CR would be patched back to preview profile, triggering a new
    # Buildah build from scratch (~20 min) instead of reusing the cached image.
    When SonataFlow "callbackstatetimeouts" is deleted

    # ── existing — preview redeploy and its checks ────────────────────────────
    Then SonataFlow callbackstatetimeouts example is deployed
    Then SonataFlow "callbackstatetimeouts" has the condition "Running" set to "True" within 5 minutes
    Then ConfigMap "callbackstatetimeouts-managed-props" exists
    Then ConfigMap "callbackstatetimeouts-managed-props" contains following strings:
      | kogito.data-index.health-enabled = true    |
      ...
    Then ConfigMap "sonataflow-platform-jobs-service-props" exists
    Then ConfigMap "sonataflow-platform-jobs-service-props" contains following strings:
      ...
```

The managed-props ConfigMap is checked **twice**:
- After the gitops redeploy → verifies the to-version operator reconciles a
  gitops-profile workflow correctly.
- After the preview redeploy → verifies the to-version operator reconciles a
  preview-profile workflow correctly (different reconciliation code path).

---

## Resulting scenario shape

```
Given  SonataFlow Operator at from-version is installed via OLM
Given  Namespace is created
When   Postgres + SonataFlowPlatform deployed
When   callbackstatetimeouts (preview) → Running                        ← existing
When   callbackstatetimeouts gitops (from-version image) → Running      ← NEW
When   SonataFlow "callbackstatetimeouts" is deleted                    ← existing
Then   DI + JS services exist
When   Operator upgraded to next version
Then   Operator version matches upgrade target
Then   DI pods log new-version startup string
Then   JS pods log new-version startup string
When   callbackstatetimeouts gitops (to-version image) → Running        ← NEW
Then   ConfigMap "callbackstatetimeouts-managed-props" checks           ← NEW
When   SonataFlow "callbackstatetimeouts" is deleted                    ← NEW (explicit cleanup)
When   callbackstatetimeouts (preview) redeployed → Running             ← existing
Then   ConfigMap "callbackstatetimeouts-managed-props" checks           ← existing
Then   ConfigMap "sonataflow-platform-jobs-service-props" checks        ← existing
```

---

## Summary Table

| Sub-task | File | Change | Risk |
|----------|------|--------|------|
| 10a | `test/testdata/callbackstate-timeouts/gitops/callbackstatetimeouts_gitops.yaml` | Parameterised gitops YAML ✅ created | Low |
| 10b | `test/yaml.go` | Path constant + `GetSonataFlowCallbackstateTimeoutsGitops()` | Low |
| 10c | `testbdd/steps/sonataflow.go` | Two steps: read template → substitute version → `oc apply` | Low |
| 10d | `testbdd/features/operator/operator_upgrade.feature` | Pre-upgrade gitops deploy; post-upgrade gitops redeploy + checks + **explicit delete** before preview redeploy | Low |

All Go changes are one commit. No new config flags needed — both steps reuse
the existing `upgrade.from_version` / `upgrade.to_version` flags.
