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
	framework.GetLogger(data.OperatorNamespace).Info("Checking if Service exists", "service", serviceName)

	_, err := framework.GetService(data.OperatorNamespace, serviceName)
	if err != nil {
		return fmt.Errorf("Service %s does not exist in namespace %s: %v", serviceName, data.OperatorNamespace, err)
	}
	return nil
}

// configMapNamespace returns OperatorNamespace when the operator has been deployed in this
// scenario, and falls back to the scenario's own Namespace for ConfigMaps that live alongside
// platform/workflow resources (e.g. *-props, *-managed-props).
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

	data.OperatorNamespace = installers.LogicOperatorNamespace
	// Subscription lives in the cluster-operator namespace (openshift-operators).
	subNS := framework.GetClusterOperatorNamespace()
	fromCSV := fmt.Sprintf("logic-operator.v%s", fromVersion)

	framework.GetLogger(data.OperatorNamespace).Info("Installing SonataFlow operator at from-version via OLM",
		"version", fromVersion, "csv", fromCSV,
		"catalog", "redhat-operators", "subscriptionNamespace", subNS)

	// Delete any pre-existing subscription so OLM starts clean at fromCSV.
	// CreateIfNotExists would silently keep a stale subscription locked to a
	// different startingCSV, causing unsatisfiable constraint errors.
	cli := "kubectl"
	if framework.IsOpenshift() {
		cli = "oc"
	}
	if _, err := framework.CreateCommand(cli, "delete", "subscription",
		installers.LogicOperatorSubscriptionName, "-n", subNS,
		"--ignore-not-found=true").Execute(); err != nil {
		return fmt.Errorf("error deleting existing subscription: %w", err)
	}
	// Also delete the CSV from the target namespace so OLM re-installs cleanly.
	if _, err := framework.CreateCommand(cli, "delete", "csv", fromCSV,
		"-n", installers.LogicOperatorNamespace,
		"--ignore-not-found=true").Execute(); err != nil {
		return fmt.Errorf("error deleting existing CSV %s: %w", fromCSV, err)
	}

	fromInstaller := &kogitoInstallers.OlmClusterWideServiceInstaller{
		SubscriptionName: installers.LogicOperatorSubscriptionName,
		Channel:          installers.LogicOperatorSubscriptionChannel,
		StartingCSV:      fromCSV,
		// Use redhat-operators (registry.redhat.io) — no staging auth required.
		Catalog:                             framework.GetProductCatalog,
		InstallationTimeoutInMinutes:        10,
		GetAllClusterWideOlmCrsInNamespace:  func(_ string) ([]client.Object, error) { return nil, nil },
		CleanupClusterWideOlmCrsInNamespace: func(_ string) bool { return true },
	}

	return fromInstaller.Install(data.OperatorNamespace)
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

	ns := installers.LogicOperatorNamespace
	subscriptionName := installers.LogicOperatorSubscriptionName

	framework.GetLogger(ns).Info("Upgrading SonataFlow operator via OLM",
		"subscription", subscriptionName, "targetCSV", fmt.Sprintf("logic-operator.v%s", toVersion))

	cli := "kubectl"
	if framework.IsOpenshift() {
		cli = "oc"
	}

	// Pin the subscription to the target CSV so OLM queues an InstallPlan.
	// The channel stays "stable" — only the desired CSV changes.
	targetCSV := fmt.Sprintf("logic-operator.v%s", toVersion)
	_, err := framework.CreateCommand(cli, "patch", "subscription", subscriptionName,
		"-n", ns,
		"--type=merge",
		fmt.Sprintf(`--patch={"spec":{"channel":"%s","startingCSV":"%s"}}`,
			installers.LogicOperatorSubscriptionChannel, targetCSV),
	).Execute()
	if err != nil {
		return fmt.Errorf("error patching subscription to CSV %s: %v", targetCSV, err)
	}

	// Wait for OLM to surface an InstallPlan with the target CSV, then approve it.
	if approveErr := framework.WaitForOnOpenshift(ns, "InstallPlan for "+targetCSV+" approved", 5,
		func() (bool, error) {
			return approveInstallPlanForCSV(cli, ns, targetCSV)
		},
	); approveErr != nil {
		return fmt.Errorf("error approving InstallPlan for %s: %v", targetCSV, approveErr)
	}

	// Wait for the new operator pod to be running.
	return framework.WaitForPodsWithLabel(ns, "app.kubernetes.io/name", "logic-operator", 1, 5)
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

// sonataFlowOperatorRunningVersionMatchesUpgradeTarget verifies that the controller-manager
// Deployment in the operator namespace is running a pod whose image tag matches the
// configured upgrade to-version.
func (data *Data) sonataFlowOperatorRunningVersionMatchesUpgradeTarget() error {
	toVersion := config.GetUpgradeToVersion()
	if toVersion == "" {
		return fmt.Errorf("upgrade to-version is not configured: set --tests.upgrade.to_version")
	}

	ns := installers.LogicOperatorNamespace
	deployment, err := framework.GetDeployment(ns, installers.LogicOperatorDeploymentName)
	if err != nil {
		return fmt.Errorf("error fetching operator deployment: %v", err)
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
		if strings.Contains(container.Image, toVersion) {
			framework.GetLogger(ns).Info("Operator version verified", "image", container.Image, "version", toVersion)
			return nil
		}
	}
	return fmt.Errorf("operator deployment does not reference version %s in any container image", toVersion)
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
