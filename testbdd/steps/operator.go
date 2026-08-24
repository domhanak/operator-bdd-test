/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package steps

import (
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubesmarts/operator-bdd-test/bddframework/pkg/config"
	"github.com/kubesmarts/operator-bdd-test/bddframework/pkg/framework"
	kogitoInstallers "github.com/kubesmarts/operator-bdd-test/bddframework/pkg/installers"
	"github.com/kubesmarts/operator-bdd-test/testbdd/installers"
)

func registerOperatorSteps(ctx *godog.ScenarioContext, data *Data) {
	ctx.Step(`^SonataFlow Operator is deployed$`, data.sonataFlowOperatorIsDeployed)
	ctx.Step(`^SonataFlow Operator has (\d+) (?:pod|pods) running$`, data.sonataFlowOperatorHasPodsRunning)
	ctx.Step(`^Service "([^"]*)" exists$`, data.serviceExists)
	ctx.Step(`^ConfigMap "([^"]*)" exists$`, data.configMapExists)
	ctx.Step(`^ConfigMap "([^"]*)" is empty$`, data.configMapIsEmpty)
	ctx.Step(`^ConfigMap "([^"]*)" contains following strings:$`, data.configMapContainsStrings)
	ctx.Step(`^SonataFlow Operator is installed via OLM$`, data.sonataFlowOperatorIsInstalledViaOLM)
	ctx.Step(`^SonataFlow Operator at from-version is installed via OLM$`, data.sonataFlowOperatorAtFromVersionIsInstalledViaOLM)
	ctx.Step(`^SonataFlow Operator is upgraded to next version$`, data.sonataFlowOperatorIsUpgradedToNextVersion)
	ctx.Step(`^SonataFlow Operator running version matches upgrade target$`, data.sonataFlowOperatorRunningVersionMatchesUpgradeTarget)
	ctx.Step(`^DB migrator job for platform "([^"]*)" completes within (\d+) minutes?$`, data.dbMigratorJobForPlatformCompletesWithinMinutes)
	// Not migrated yet
	//ctx.Step(`^Kogito operator should be installed$`, data.kogitoOperatorShouldBeInstalled)
	//ctx.Step(`^CLI install Kogito operator$`, data.cliInstallKogitoOperator)
}

func (data *Data) sonataFlowOperatorIsDeployed() (err error) {
	var installer kogitoInstallers.ServiceInstaller
	// Always use OSL namespace
	data.OperatorNamespace = installers.LogicOperatorNamespace
	if config.UseProductOperator() {
		installer, err = &kogitoInstallers.YamlClusterWideServiceInstaller{}, fmt.Errorf("OLM is not supported by the steps yet")
	} else {
		installer, err = installers.GetSonataFlowInstaller()
	}
	if err != nil {
		return err
	}
	return installer.Install(data.OperatorNamespace)
}

func (data *Data) sonataFlowOperatorHasPodsRunning(numberOfPods int) error {
	return framework.WaitForPodsWithLabel(data.OperatorNamespace, "app.kubernetes.io/name", "sonataflow-operator", numberOfPods, 1)
}

func (data *Data) serviceExists(serviceName string) error {
	ns := data.configMapNamespace()
	framework.GetLogger(ns).Info("Checking if Service exists", "service", serviceName)

	_, err := framework.GetService(ns, serviceName)
	if err != nil {
		return fmt.Errorf("Service %s does not exist in namespace %s: %v", serviceName, ns, err)
	}
	return nil
}

// configMapNamespace returns the namespace for ConfigMap and Service lookups.
//
// Rules:
//   - data.OperatorNamespace is set only when a scenario explicitly installs the
//     operator via "SonataFlow Operator is deployed". In that case, the step also
//     checks operator-level resources (builder-config, metrics-service) which live
//     in openshift-serverless-logic, so OperatorNamespace takes precedence.
//   - In upgrade scenarios "SonataFlow Operator at from-version is installed via
//     OLM" deliberately does NOT set data.OperatorNamespace, so all resource
//     checks resolve to data.Namespace (the randomly generated scenario namespace,
//     e.g. bdd-4a36, where platform and workflow resources are deployed).
func (data *Data) configMapNamespace() string {
	if data.OperatorNamespace != "" {
		return data.OperatorNamespace
	}
	return data.Namespace
}

func (data *Data) configMapExists(cmName string) error {
	ns := data.configMapNamespace()
	framework.GetLogger(ns).Info("Checking if ConfigMap exists", "configMap", cmName)

	exists, err := framework.IsConfigMapExist(types.NamespacedName{Name: cmName, Namespace: ns})
	if err != nil {
		return fmt.Errorf("error while checking if ConfigMap %s exists: %v", cmName, err)
	}
	if !exists {
		return fmt.Errorf("ConfigMap %s does not exist in namespace %s", cmName, ns)
	}
	return nil
}

func (data *Data) configMapIsEmpty(cmName string) error {
	ns := data.configMapNamespace()
	framework.GetLogger(ns).Info("Checking that ConfigMap has no data", "configMap", cmName)

	cm := &corev1.ConfigMap{}
	exists, err := framework.GetObjectWithKey(types.NamespacedName{Name: cmName, Namespace: ns}, cm)
	if err != nil {
		return fmt.Errorf("error fetching ConfigMap %s: %v", cmName, err)
	}
	if !exists {
		return fmt.Errorf("ConfigMap %s does not exist in namespace %s", cmName, ns)
	}

	for key, value := range cm.Data {
		if strings.TrimSpace(value) != "" {
			return fmt.Errorf("ConfigMap '%s' is not empty: key '%s' has value '%s'", cmName, key, value)
		}
	}
	return nil
}

func (data *Data) configMapContainsStrings(cmName string, table *godog.Table) error {
	ns := data.configMapNamespace()
	framework.GetLogger(ns).Info("Validating ConfigMap contains strings", "configMap", cmName)

	cm := &corev1.ConfigMap{}
	exists, err := framework.GetObjectWithKey(types.NamespacedName{Name: cmName, Namespace: ns}, cm)
	if err != nil {
		return fmt.Errorf("error fetching ConfigMap %s: %v", cmName, err)
	}
	if !exists {
		return fmt.Errorf("ConfigMap %s does not exist in namespace %s", cmName, ns)
	}

	// Concatenate all ConfigMap values into one large string for easy searching
	var allDataValues strings.Builder
	for _, value := range cm.Data {
		allDataValues.WriteString(value)
	}

	// Iterate over the Gherkin data table
	for _, row := range table.Rows {
		if len(row.Cells) == 0 {
			continue
		}

		expectedString := row.Cells[0].Value

		// Placeholder Resolution Logic - allows to check for string influenced by the stream
		expectedString = resolveUpgradePlaceholders(expectedString)

		if strings.Contains(expectedString, "${RELATED_IMAGE_BASE_BUILDER}") {
			builderImage := config.GetRelatedImage("RELATED_IMAGE_BASE_BUILDER")
			framework.GetLogger(data.Namespace).Info("Builder Image is:", "Image:", builderImage)
			if builderImage == "" {
				// Fallback to the default if the property wasn't provided during the test run
				builderImage = "registry.redhat.io/openshift-serverless-1/logic-swf-builder-rhel9:1.38.0"
			}
			expectedString = strings.ReplaceAll(expectedString, "${RELATED_IMAGE_BASE_BUILDER}", builderImage)
		}

		if !strings.Contains(allDataValues.String(), expectedString) {
			return fmt.Errorf("ConfigMap '%s' does not contain the expected string:\n'%s'", cmName, expectedString)
		}
	}

	return nil
}

// sonataFlowOperatorIsInstalledViaOLM installs the OSL operator at the version configured
// via --tests.operator.version using OLM and the custom catalog image. This is the generic
// install step for non-upgrade scenarios that still need a specific operator version pinned.
func (data *Data) sonataFlowOperatorIsInstalledViaOLM() error {
	version := config.GetOperatorVersion()
	if version == "" {
		return fmt.Errorf("operator version is not configured: set --tests.operator.version")
	}
	return data.installOperatorViaOLM(version)
}

// sonataFlowOperatorAtFromVersionIsInstalledViaOLM installs the OSL operator at the
// configured from-version using OLM and the custom catalog image. It creates an
// OlmClusterWideServiceInstaller on the fly with:
//   - SubscriptionName: "logic-operator"
//   - Channel:          the from-version string (e.g. "1.37.2")
//   - StartingCSV:      "logic-operator.v<fromVersion>"
//
// This pins OLM to the exact from-version CSV so the subsequent upgrade step
// has a clean baseline to upgrade from.
func (data *Data) sonataFlowOperatorAtFromVersionIsInstalledViaOLM() error {
	fromVersion := config.GetUpgradeFromVersion()
	if fromVersion == "" {
		return fmt.Errorf("upgrade from-version is not configured: set --tests.upgrade.from_version")
	}
	return data.installOperatorViaOLM(fromVersion)
}

// installOperatorViaOLM is the shared implementation for OLM-based operator installs.
// It deletes any pre-existing subscription and CSVs so OLM starts clean, then installs
// the operator at the given version using the redhat-operators product catalog.
//
// Do NOT set data.OperatorNamespace here. Callers that need namespace-scoped resource
// resolution (serviceExists / configMapNamespace) should resolve to data.Namespace, which
// is set by the subsequent "Namespace is created" step.
func (data *Data) installOperatorViaOLM(version string) error {
	operatorNS := installers.LogicOperatorNamespace
	subNS := framework.GetClusterOperatorNamespace()
	csv := fmt.Sprintf("logic-operator.v%s", version)

	framework.GetLogger(operatorNS).Info("Installing SonataFlow operator via OLM",
		"version", version, "csv", csv,
		"catalog", "redhat-operators", "subscriptionNamespace", subNS)

	cli := "kubectl"
	if framework.IsOpenshift() {
		cli = "oc"
	}
	// Delete any pre-existing subscription so OLM starts clean at the target CSV.
	// CreateIfNotExists would silently keep a stale subscription locked to a
	// different startingCSV, causing unsatisfiable constraint errors.
	if _, err := framework.CreateCommand(cli, "delete", "subscription",
		installers.LogicOperatorSubscriptionName, "-n", subNS,
		"--ignore-not-found=true").Execute(); err != nil {
		return fmt.Errorf("error deleting existing subscription: %w", err)
	}
	// Delete ALL logic-operator CSVs from the operator namespace.
	// A leftover CSV from a previous test run has no subscription reference and
	// causes OLM's constraint solver to emit "@existing ... is not referenced by
	// a subscription" conflicts.
	if err := deleteAllLogicOperatorCSVs(cli, installers.LogicOperatorNamespace); err != nil {
		return fmt.Errorf("error deleting existing logic-operator CSVs: %w", err)
	}

	// Use the custom IIB catalog when one is configured (operator_catalog_image is set),
	// otherwise fall back to the product catalog (redhat-operators).
	catalogFn := framework.GetProductCatalog
	if config.GetOperatorCatalogImage() != "" {
		catalogFn = framework.GetCustomKogitoOperatorCatalog
	}

	installer := &kogitoInstallers.OlmClusterWideServiceInstaller{
		SubscriptionName:                    installers.LogicOperatorSubscriptionName,
		Channel:                             installers.LogicOperatorSubscriptionChannel,
		StartingCSV:                         csv,
		Catalog:                             catalogFn,
		InstallationTimeoutInMinutes:        10,
		GetAllClusterWideOlmCrsInNamespace:  func(_ string) ([]client.Object, error) { return nil, nil },
		CleanupClusterWideOlmCrsInNamespace: func(_ string) bool { return true },
	}

	return installer.Install(operatorNS)
}

// resolveUpgradePlaceholders replaces ${UPGRADE_FROM_VERSION} and ${UPGRADE_TO_VERSION}
// with values from the test configuration flags --tests.upgrade.from_version and
// --tests.upgrade.to_version respectively.
func resolveUpgradePlaceholders(s string) string {
	if strings.Contains(s, "${UPGRADE_FROM_VERSION}") {
		s = strings.ReplaceAll(s, "${UPGRADE_FROM_VERSION}", config.GetUpgradeFromVersion())
	}
	if strings.Contains(s, "${UPGRADE_TO_VERSION}") {
		s = strings.ReplaceAll(s, "${UPGRADE_TO_VERSION}", config.GetUpgradeToVersion())
	}
	return s
}

// sonataFlowOperatorIsUpgradedToNextVersion upgrades the OSL operator from the configured
// from-version to the to-version via OLM by patching the Subscription channel to the
// target version channel and approving any pending InstallPlan.
//
// Prerequisites: the operator must already be installed via OLM (the scenario's Given step
// deploys it at from-version). This step triggers the OLM upgrade and waits for the new
// operator pod to become ready.
func (data *Data) sonataFlowOperatorIsUpgradedToNextVersion() error {
	toVersion := config.GetUpgradeToVersion()
	if toVersion == "" {
		return fmt.Errorf("upgrade to-version is not configured: set --tests.upgrade.to_version")
	}

	// The operator itself runs in openshift-serverless-logic, but the OLM Subscription
	// and InstallPlans are cluster-wide and live in openshift-operators.
	operatorNS := installers.LogicOperatorNamespace
	subNS := framework.GetClusterOperatorNamespace()
	subscriptionName := installers.LogicOperatorSubscriptionName

	framework.GetLogger(operatorNS).Info("Upgrading SonataFlow operator via OLM",
		"subscription", subscriptionName, "subscriptionNamespace", subNS,
		"targetCSV", fmt.Sprintf("logic-operator.v%s", toVersion))

	cli := "kubectl"
	if framework.IsOpenshift() {
		cli = "oc"
	}

	// Switch the subscription source to the custom IIB catalog which carries the
	// to-version CSV, then pin it to the target CSV so OLM queues an InstallPlan.
	// The from-version was installed from redhat-operators; the to-version exists
	// only in the catalog image supplied via --tests.operator_catalog_image (the
	// custom "bdd-tests-kogito-catalog" CatalogSource registered at suite start).
	targetCSV := fmt.Sprintf("logic-operator.v%s", toVersion)
	customCatalog := framework.GetCustomKogitoOperatorCatalog()
	_, err := framework.CreateCommand(cli, "patch", "subscription", subscriptionName,
		"-n", subNS,
		"--type=merge",
		fmt.Sprintf(`--patch={"spec":{"channel":"%s","startingCSV":"%s","source":"%s","sourceNamespace":"%s"}}`,
			installers.LogicOperatorSubscriptionChannel, targetCSV,
			customCatalog.Source(), customCatalog.Namespace()),
	).Execute()
	if err != nil {
		return fmt.Errorf("error patching subscription to CSV %s: %v", targetCSV, err)
	}

	// Wait for OLM to surface an InstallPlan with the target CSV, then approve it.
	// InstallPlans are created in the same namespace as the Subscription (openshift-operators).
	if approveErr := framework.WaitForOnOpenshift(operatorNS, "InstallPlan for "+targetCSV+" approved", 5,
		func() (bool, error) {
			return approveInstallPlanForCSV(cli, subNS, targetCSV)
		},
	); approveErr != nil {
		return fmt.Errorf("error approving InstallPlan for %s: %v", targetCSV, approveErr)
	}

	// The to-version operator pod is deployed by OLM into the subscription namespace
	// (openshift-operators), not into openshift-serverless-logic. The pod label
	// app.kubernetes.io/name is set to "sonataflow-operator" by the CSV.
	return framework.WaitForPodsWithLabel(subNS, "app.kubernetes.io/name", "sonataflow-operator", 1, 5)
}

// approveInstallPlanForCSV finds an InstallPlan that contains the target CSV and approves it.
// Returns (true, nil) once approved, (false, nil) while still waiting.
func approveInstallPlanForCSV(cli, namespace, targetCSV string) (bool, error) {
	out, err := framework.CreateCommand(cli, "get", "installplan",
		"-n", namespace,
		"-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\t\"}{.spec.approved}{\"\\t\"}{.spec.clusterServiceVersionNames[*]}{\"\\n\"}{end}",
	).Execute()
	if err != nil {
		return false, nil // not ready yet
	}

	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		planName, approved, csvNames := parts[0], parts[1], parts[2]
		if strings.Contains(csvNames, targetCSV) && approved == "false" {
			if _, patchErr := framework.CreateCommand(cli, "patch", "installplan", planName,
				"-n", namespace,
				"--type=merge",
				`--patch={"spec":{"approved":true}}`,
			).Execute(); patchErr != nil {
				return false, patchErr
			}
			return true, nil
		}
		if strings.Contains(csvNames, targetCSV) && approved == "true" {
			return true, nil
		}
	}
	return false, nil
}

// deleteAllLogicOperatorCSVs lists all CSVs in namespace whose name begins with
// "logic-operator." and deletes each one. This cleans up any version left from a
// previous test run before re-installing from scratch via OLM.
func deleteAllLogicOperatorCSVs(cli, namespace string) error {
	out, err := framework.CreateCommand(cli, "get", "csv",
		"-n", namespace,
		"-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\n\"}{end}",
	).Execute()
	if err != nil {
		// If the CSV CRD doesn't exist yet, there's nothing to delete.
		return nil
	}
	prefix := installers.LogicOperatorSubscriptionName + "."
	for _, name := range strings.Split(strings.TrimSpace(out), "\n") {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		framework.GetLogger(namespace).Info("Deleting stale CSV", "csv", name)
		if _, delErr := framework.CreateCommand(cli, "delete", "csv", name,
			"-n", namespace,
			"--ignore-not-found=true").Execute(); delErr != nil {
			return fmt.Errorf("error deleting CSV %s: %w", name, delErr)
		}
	}
	return nil
}

// sonataFlowOperatorRunningVersionMatchesUpgradeTarget verifies that the OLM-managed
// controller-manager Deployment has been reconciled by the target CSV version.
//
// Image tags are not used for version detection because OLM uses SHA256 digests
// (e.g. registry.redhat.io/...@sha256:...) which do not embed a version string.
// Instead we check the olm.owner label that OLM stamps on every Deployment it
// manages — its value is always "<package>.v<version>" (e.g. logic-operator.v1.39.0).
func (data *Data) sonataFlowOperatorRunningVersionMatchesUpgradeTarget() error {
	toVersion := config.GetUpgradeToVersion()
	if toVersion == "" {
		return fmt.Errorf("upgrade to-version is not configured: set --tests.upgrade.to_version")
	}

	// The upgraded operator is deployed by OLM into the subscription namespace.
	ns := framework.GetClusterOperatorNamespace()
	expectedCSV := fmt.Sprintf("%s.v%s", installers.LogicOperatorSubscriptionName, toVersion)

	// OLM reconciles the Deployment asynchronously after approving the InstallPlan.
	// Poll until the olm.owner label on the Deployment matches the target CSV.
	return framework.WaitForOnOpenshift(ns, "operator deployment owned by "+expectedCSV, 5,
		func() (bool, error) {
			deployment, err := framework.GetDeployment(ns, installers.LogicOperatorDeploymentName)
			if err != nil {
				// Deployment may not exist yet during the rollout; keep waiting.
				return false, nil
			}
			actualCSV := deployment.Labels["olm.owner"]
			if actualCSV == expectedCSV {
				framework.GetLogger(ns).Info("Operator version verified via olm.owner label",
					"olm.owner", actualCSV)
				return true, nil
			}
			framework.GetLogger(ns).Info("Waiting for olm.owner label update",
				"current", actualCSV, "expected", expectedCSV)
			return false, nil
		},
	)
}

// dbMigratorJobForPlatformCompletesWithinMinutes waits for the sonataflow-db-migrator-job
// labelled with the target version to complete successfully. It matches the label selector:
//
//	app=<platformName>, app.kubernetes.io/version=<upgradeToVersion>
func (data *Data) dbMigratorJobForPlatformCompletesWithinMinutes(platformName string, timeoutInMin int) error {
	toVersion := config.GetUpgradeToVersion()
	if toVersion == "" {
		return fmt.Errorf("upgrade to-version is not configured: set --tests.upgrade.to_version")
	}

	ns := data.Namespace
	framework.GetLogger(ns).Info("Waiting for DB migrator job to complete",
		"platform", platformName, "version", toVersion, "namespace", ns)

	return framework.WaitForOnOpenshift(ns, "DB migrator job complete", timeoutInMin,
		func() (bool, error) {
			jobs := &batchv1.JobList{}
			if err := framework.GetObjectsInNamespace(ns, jobs); err != nil {
				return false, err
			}
			for i := range jobs.Items {
				job := &jobs.Items[i]
				if job.Labels["app"] == platformName &&
					job.Labels["app.kubernetes.io/version"] == toVersion &&
					job.Status.Succeeded >= 1 {
					return true, nil
				}
			}
			return false, nil
		},
	)
}

//
//func (data *Data) kogitoOperatorShouldBeInstalled() error {
//	return framework.WaitForKogitoOperatorRunning(data.Namespace)
//}
//
//func (data *Data) cliInstallKogitoOperator() error {
//	_, err := framework.ExecuteCliCommandInNamespace(data.Namespace, "install", "operator")
//	return err
//}
