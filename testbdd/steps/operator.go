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

	"github.com/kubesmarts/operator-bdd-test/testbdd/installers"

	"github.com/kubesmarts/operator-bdd-test/bddframework/pkg/config"
	"github.com/kubesmarts/operator-bdd-test/bddframework/pkg/framework"
	kogitoInstallers "github.com/kubesmarts/operator-bdd-test/bddframework/pkg/installers"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/types"
)

func registerOperatorSteps(ctx *godog.ScenarioContext, data *Data) {
	ctx.Step(`^SonataFlow Operator is deployed$`, data.sonataFlowOperatorIsDeployed)
	ctx.Step(`^SonataFlow Operator has (\d+) (?:pod|pods) running$`, data.sonataFlowOperatorHasPodsRunning)
	ctx.Step(`^Service "([^"]*)" exists$`, data.serviceExists)
	ctx.Step(`^ConfigMap "([^"]*)" exists$`, data.configMapExists)
	ctx.Step(`^ConfigMap "([^"]*)" is empty$`, data.configMapIsEmpty)
	ctx.Step(`^ConfigMap "([^"]*)" contains following strings:$`, data.configMapContainsStrings)
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
		if strings.Contains(expectedString, "${RELATED_IMAGE_BASE_BUILDER}") {
			builderImage := config.GetRelatedImage("RELATED_IMAGE_BASE_BUILDER")
			if builderImage == "" {
				// Fallback to the default if the property wasn't provided during the test run
				builderImage = "docker.io/apache/incubator-kie-sonataflow-builder:main"
			}
			expectedString = strings.ReplaceAll(expectedString, "${RELATED_IMAGE_BASE_BUILDER}", builderImage)
		}

		if !strings.Contains(allDataValues.String(), expectedString) {
			return fmt.Errorf("ConfigMap '%s' does not contain the expected string:\n'%s'", cmName, expectedString)
		}
	}

	return nil
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
