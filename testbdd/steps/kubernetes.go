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

	"github.com/cucumber/godog"

	"github.com/kubesmarts/operator-bdd-test/bddframework/pkg/framework"
)

func registerKubernetesSteps(ctx *godog.ScenarioContext, data *Data) {
	// Added step to the one from the Kogito framework to be able to use quotes inside the text
	ctx.Step(`^Deployment "([^"]*)" pods log contains text '([^']*)' within (\d+) minutes$`, data.deploymentPodsLogContainsTextWithinMinutes)
	ctx.Step(`^Deployment "([^"]*)" has label "([^"]*)" with value "([^"]*)"$`, data.deploymentHasLabelWithValue)
	ctx.Step(`^Deployment "([^"]*)" pods have label "([^"]*)" with value "([^"]*)"$`, data.deploymentPodsHaveLabelWithValue)
}

func (data *Data) deploymentPodsLogContainsTextWithinMinutes(dName, logText string, timeoutInMin int) error {
	// The container name inside a pod is the last hyphen-separated segment of the
	// deployment name (e.g. "sonataflow-platform-data-index-service" → "data-index-service").
	// After an operator upgrade, the old pod and the new replacement pod coexist briefly;
	// we accept a match on any pod so the step passes as soon as the new pod has started.
	containerName := deploymentContainerName(dName)
	return framework.WaitForAnyPodsByDeploymentToContainTextInLog(data.Namespace, dName, containerName, logText, timeoutInMin)
}

// deploymentContainerName derives the container name from a deployment name by
// dropping the well-known "sonataflow-platform-" prefix when present, otherwise
// returning the full deployment name unchanged.
func deploymentContainerName(deploymentName string) string {
	const prefix = "sonataflow-platform-"
	if len(deploymentName) > len(prefix) && deploymentName[:len(prefix)] == prefix {
		return deploymentName[len(prefix):]
	}
	return deploymentName
}

// deploymentHasLabelWithValue asserts that the named Deployment carries a specific
// label key/value pair in its pod template metadata — which is what the operator
// propagates from spec.services.{dataIndex,jobService}.podTemplate.metadata.labels.
func (data *Data) deploymentHasLabelWithValue(deploymentName, labelKey, expectedValue string) error {
	deployment, err := framework.GetDeployment(data.Namespace, deploymentName)
	if err != nil {
		return fmt.Errorf("error fetching deployment %s: %w", deploymentName, err)
	}
	if deployment == nil {
		return fmt.Errorf("deployment %s not found in namespace %s", deploymentName, data.Namespace)
	}

	// The operator copies podTemplate.metadata.labels onto the pod template spec,
	// so we check deployment.Spec.Template.Labels (the pod template labels).
	podLabels := deployment.Spec.Template.Labels
	actualValue, exists := podLabels[labelKey]
	if !exists {
		return fmt.Errorf("deployment %s pod template does not have label %q (present labels: %v)",
			deploymentName, labelKey, podLabels)
	}
	if actualValue != expectedValue {
		return fmt.Errorf("deployment %s label %q: expected %q, got %q",
			deploymentName, labelKey, expectedValue, actualValue)
	}

	framework.GetLogger(data.Namespace).Info("Deployment label verified",
		"deployment", deploymentName, "label", labelKey, "value", actualValue)
	return nil
}

// deploymentPodsHaveLabelWithValue asserts that every running pod owned by the
// named Deployment carries the given label key/value pair. This verifies that
// the operator's pod-template label propagation reaches the actual pod objects,
// not just the Deployment spec.
func (data *Data) deploymentPodsHaveLabelWithValue(deploymentName, labelKey, expectedValue string) error {
	pods, err := framework.GetPodsByDeployment(data.Namespace, deploymentName)
	if err != nil {
		return fmt.Errorf("error fetching pods for deployment %s: %w", deploymentName, err)
	}
	if len(pods) == 0 {
		return fmt.Errorf("no pods found for deployment %s in namespace %s", deploymentName, data.Namespace)
	}

	for _, pod := range pods {
		actualValue, exists := pod.Labels[labelKey]
		if !exists {
			return fmt.Errorf("pod %s (deployment %s) does not have label %q (present labels: %v)",
				pod.Name, deploymentName, labelKey, pod.Labels)
		}
		if actualValue != expectedValue {
			return fmt.Errorf("pod %s (deployment %s) label %q: expected %q, got %q",
				pod.Name, deploymentName, labelKey, expectedValue, actualValue)
		}
	}

	framework.GetLogger(data.Namespace).Info("All pods label verified",
		"deployment", deploymentName, "label", labelKey, "value", expectedValue, "pods", len(pods))
	return nil
}
