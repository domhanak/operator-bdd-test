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

package installers

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubesmarts/operator-bdd-test/bddframework/pkg/config"
	"github.com/kubesmarts/operator-bdd-test/bddframework/pkg/framework"
	"github.com/kubesmarts/operator-bdd-test/bddframework/pkg/installers"
	srvframework "github.com/kubesmarts/operator-bdd-test/testbdd/framework"
)

const defaultOperatorImage = "registry-proxy.engineering.redhat.com/rh-osbs/openshift-serverless-1-logic-rhel9-operator:latest"

var (
	// sonataFlowYamlClusterInstaller installs SonataFlow operator cluster wide using YAMLs
	sonataFlowYamlClusterInstaller = installers.YamlClusterWideServiceInstaller{
		InstallClusterYaml:               installSonataFlowUsingYaml,
		InstallationNamespace:            LogicOperatorNamespace,
		WaitForClusterYamlServiceRunning: waitForSonataFlowOperatorUsingYamlRunning,
		GetAllClusterYamlCrsInNamespace:  getSonataFlowCrsInNamespace,
		UninstallClusterYaml:             uninstallSonataFlowUsingYaml,
		ClusterYamlServiceName:           sonataFlowServiceName,
		CleanupClusterYamlCrsInNamespace: cleanupSonataFlowCrsInNamespace,
	}

	// sonataFlowCustomOlmClusterWideInstaller installs SonataFlow cluster wide using OLM with custom catalog
	sonataFlowCustomOlmClusterWideInstaller = installers.OlmClusterWideServiceInstaller{
		SubscriptionName:                    logicOperatorSubscriptionName,
		Channel:                             sonataFlowOperatorSubscriptionChannel,
		Catalog:                             framework.GetCustomKogitoOperatorCatalog,
		InstallationTimeoutInMinutes:        5,
		GetAllClusterWideOlmCrsInNamespace:  getSonataFlowCrsInNamespace,
		CleanupClusterWideOlmCrsInNamespace: cleanupSonataFlowCrsInNamespace,
	}

	// sonataFlowOlmClusterWideInstaller installs SonataFlow cluster wide using OLM with community catalog
	sonataFlowOlmClusterWideInstaller = installers.OlmClusterWideServiceInstaller{
		SubscriptionName:                    sonataFlowOperatorSubscriptionName,
		Channel:                             sonataFlowOperatorSubscriptionChannel,
		Catalog:                             framework.GetCommunityCatalog,
		InstallationTimeoutInMinutes:        5,
		GetAllClusterWideOlmCrsInNamespace:  getSonataFlowCrsInNamespace,
		CleanupClusterWideOlmCrsInNamespace: cleanupSonataFlowCrsInNamespace,
	}

	// SonataFlowNamespace is the SonataFlow namespace for yaml cluster-wide deployment
	SonataFlowNamespace   = "sonataflow-operator-system"
	sonataFlowServiceName = "SonataFlow operator"

	sonataFlowOperatorSubscriptionName    = "sonataflow-operator"
	sonataFlowOperatorSubscriptionChannel = "stable"

	sonataFlowOperatorControllerConfigName                = sonataFlowOperatorSubscriptionName + "-controllers-config"
	sonataFlowOperatorBuilderConfigName                   = sonataFlowOperatorSubscriptionName + "-builder-config"
	sonataFlowOperatorControllerManagerServiceAccountName = sonataFlowOperatorSubscriptionName + "-controller-manager"
	sonataFlowOperatorMetricsReaderName                   = sonataFlowOperatorSubscriptionName + "-metrics-reader"
	sonataFlowOperatorLeaderElectionRoleName              = sonataFlowOperatorSubscriptionName + "-leader-election-role"
	sonataFlowOperatorBuilderManagerRoleName              = sonataFlowOperatorSubscriptionName + "-builder-manager-role"

	// Openshift Serverless Logic naming constants
	LogicOperatorNamespace           = "openshift-serverless-logic"
	LogicOperatorSubscriptionName    = "logic-operator"
	LogicOperatorSubscriptionChannel = "stable"
	LogicOperatorDeploymentName      = LogicOperatorSubscriptionName + "-controller-manager"

	logicOperatorSubscriptionName = LogicOperatorSubscriptionName

	logicOperatorControllerConfigName                = logicOperatorSubscriptionName + "-controllers-config"
	logicOperatorBuilderConfigName                   = logicOperatorSubscriptionName + "-builder-config"
	logicOperatorControllerManagerServiceAccountName = logicOperatorSubscriptionName + "-controller-manager"
	logicOperatorMetricsReaderName                   = logicOperatorSubscriptionName + "-metrics-reader"
	logicOperatorLeaderElectionRoleName              = logicOperatorSubscriptionName + "-leader-election-role"
	logicOperatorBuilderManagerRoleName              = logicOperatorSubscriptionName + "-builder-manager-role"
)

// GetSonataFlowInstaller returns SonataFlow installer
func GetSonataFlowInstaller() (installers.ServiceInstaller, error) {
	// If user doesn't pass SonataFlow operator image then use community OLM catalog to install operator
	if len(config.GetOperatorImageTag()) == 0 {
		framework.GetMainLogger().Info("Installing SonataFlow operator using community catalog.")
		return &sonataFlowOlmClusterWideInstaller, nil
	}

	if config.IsOperatorInstalledByYaml() || config.IsOperatorProfiling() {
		return &sonataFlowYamlClusterInstaller, nil
	}

	if config.IsOperatorInstalledByOlm() {
		return &sonataFlowCustomOlmClusterWideInstaller, nil
	}

	return nil, errors.New("no SonataFlow operator installer available for provided configuration")
}

func installSonataFlowUsingYaml() error {
	framework.GetMainLogger().Info("Installing SonataFlow operator")

	operatorImage := config.GetOperatorImageTag()
	manifestURI := config.GetOperatorYamlURI()

	// Read the content of operator.yaml from configured URL
	yamlContent, err := framework.ReadFromURI(manifestURI)
	if err != nil {
		framework.GetMainLogger().Error(err, "Error while reading the operator YAML file at %s", manifestURI)
		return err
	}

	// Patch the image reference of operator
	if len(operatorImage) > 0 {
		imageRegex := regexp.MustCompile(`image:\s*["']?.*?incubator-kie-sonataflow-operator[^"'\s]*["']?`)
		if !imageRegex.MatchString(yamlContent) {
			// Fallback: search for a generic placeholder if the specific one isn't found
			imageRegex = regexp.MustCompile(`image:\s*["']?placeholder["']?|image:\s*["']?main["']?`)
		}
		yamlContent = imageRegex.ReplaceAllString(yamlContent, fmt.Sprintf("image: %s", operatorImage))
	}

	// 2. Patch Related Images Environment Variables
	relatedImageVars := []string{
		"RELATED_IMAGE_JOBS_SERVICE_POSTGRESQL",
		"RELATED_IMAGE_JOBS_SERVICE_EPHEMERAL",
		"RELATED_IMAGE_DATA_INDEX_POSTGRESQL",
		"RELATED_IMAGE_DATA_INDEX_EPHEMERAL",
		"RELATED_IMAGE_DB_MIGRATOR_TOOL",
		"RELATED_IMAGE_BASE_BUILDER",
		"RELATED_IMAGE_DEVMODE",
	}

	for _, envVar := range relatedImageVars {
		overrideValue := config.GetRelatedImage(envVar)
		if len(overrideValue) > 0 {
			// (?s) allows the regex to read across newlines.
			// It finds "name: <VAR>" and the subsequent "value: " string, keeping them intact (${1})
			// and replaces the actual image tag.
			re := regexp.MustCompile(`(?s)(name:\s*` + envVar + `\s+value:\s*)([^\s"']+)`)
			yamlContent = re.ReplaceAllString(yamlContent, "${1}"+overrideValue)

			framework.GetMainLogger().Info(fmt.Sprintf("Patched %s with %s", envVar, overrideValue))
		}
	}

	// Replace builder image reference in sonataflow-operator-builder-config
	builderImageUrl := config.GetRelatedImage("RELATED_IMAGE_BASE_BUILDER")
	builderRegex := regexp.MustCompile(`docker\.io/apache/incubator-kie-sonataflow-builder[^\s"']*`)
	yamlContent = builderRegex.ReplaceAllString(yamlContent, builderImageUrl)

	// Replace sonataflow-operator-system with openshift-serverless-logic
	yamlContent = strings.ReplaceAll(yamlContent, SonataFlowNamespace, LogicOperatorNamespace)
	// Replace remaining community prefixes
	yamlContent = strings.ReplaceAll(yamlContent, "sonataflow-operator-", "logic-operator-")

	// Create also one file to be able to inspect the YAML if needed
	framework.CreateFile("./logs/", "operator.yaml", yamlContent)
	tempFilePath, err := framework.CreateTemporaryFile("logic-operator*.yaml", yamlContent)
	if err != nil {
		framework.GetMainLogger().Error(err, "Error while storing adjusted YAML content to temporary file")
		return err
	}

	// TODO: Make this differentiate between different CLIs
	_, err = framework.CreateCommand("oc", "create", "-f", tempFilePath).Execute()
	if err != nil {
		framework.GetMainLogger().Error(err, "Error while installing SonataFlow operator from YAML file")
		return err
	}

	return nil
}

func waitForSonataFlowOperatorUsingYamlRunning() error {
	return srvframework.WaitForSonataFlowOperatorRunning(LogicOperatorNamespace)
}

func uninstallSonataFlowUsingYaml() error {
	framework.GetMainLogger().Info("Uninstalling SonataFlow operator")

	output, err := framework.CreateCommand("oc", "delete", "-f", "./operator.yaml", "--timeout=60s", "--ignore-not-found=true").Execute()
	if err != nil {
		framework.GetMainLogger().Error(err, fmt.Sprintf("Deleting SonataFlow operator failed, output:\n %s", output))
		return err
	}

	return nil
}

func getSonataFlowCrsInNamespace(namespace string) ([]client.Object, error) {
	var crs []client.Object

	//kogitoRuntimes := &v1beta1.KogitoRuntimeList{}
	//if err := framework.GetObjectsInNamespace(namespace, kogitoRuntimes); err != nil {
	//	return nil, err
	//}
	//for i := range kogitoRuntimes.Items {
	//	crs = append(crs, &kogitoRuntimes.Items[i])
	//}
	//
	//kogitoBuilds := &v1beta1.KogitoBuildList{}
	//if err := framework.GetObjectsInNamespace(namespace, kogitoBuilds); err != nil {
	//	return nil, err
	//}
	//for i := range kogitoBuilds.Items {
	//	crs = append(crs, &kogitoBuilds.Items[i])
	//}
	//
	//kogitoSupportingServices := &v1beta1.KogitoSupportingServiceList{}
	//if err := framework.GetObjectsInNamespace(namespace, kogitoSupportingServices); err != nil {
	//	return nil, err
	//}
	//for i := range kogitoSupportingServices.Items {
	//	crs = append(crs, &kogitoSupportingServices.Items[i])
	//}
	//
	//kogitoInfras := &v1beta1.KogitoInfraList{}
	//if err := framework.GetObjectsInNamespace(namespace, kogitoInfras); err != nil {
	//	return nil, err
	//}
	//for i := range kogitoInfras.Items {
	//	crs = append(crs, &kogitoInfras.Items[i])
	//}

	return crs, nil
}

func cleanupSonataFlowCrsInNamespace(namespace string) bool {
	crs, err := getSonataFlowCrsInNamespace(namespace)
	if err != nil {
		framework.GetLogger(namespace).Error(err, "Error getting SonataFlow CRs.")
		return false
	}

	for _, cr := range crs {
		if err := framework.DeleteObject(cr); err != nil {
			framework.GetLogger(namespace).Error(err, "Error deleting SonataFlow CR.", "CR name", cr.GetName())
			return false
		}
	}
	return true
}

/*
Commenting this to get rid of dependency on internal module - github.com/apache/incubator-kie-tools/packages/sonataflow-operator/internal/controller/workflowdef

	func getDefaultPostgresImageTag() string {
		return workflowdef.GetDefaultImageTag(defaultPostgresImage)
	}
*/
