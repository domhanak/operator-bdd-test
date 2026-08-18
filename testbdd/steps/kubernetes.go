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
	"github.com/cucumber/godog"

	"github.com/kubesmarts/operator-bdd-test/bddframework/pkg/framework"
)

func registerKubernetesSteps(ctx *godog.ScenarioContext, data *Data) {
	// Added step to the one from the Kogito framework to be able to use quotes inside the text
	ctx.Step(`^Deployment "([^"]*)" pods log contains text '([^']*)' within (\d+) minutes$`, data.deploymentPodsLogContainsTextWithinMinutes)
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
