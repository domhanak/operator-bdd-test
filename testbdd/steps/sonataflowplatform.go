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
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"

	framework "github.com/kubesmarts/operator-bdd-test/bddframework/pkg/framework"
	"github.com/kubesmarts/operator-bdd-test/test"
	"github.com/kubesmarts/operator-bdd-test/test/utils"

	//"github.com/kubesmarts/operator-bdd-test/test"
	"os"
)

const (
	minikubePlatform  = "minikube"
	openshiftPlatform = "openshift"
)

func registerPlatformSteps(ctx *godog.ScenarioContext, data *Data) {
	ctx.Step(`^SonataFlowPlatform is deployed$`, data.sonataFlowPlatformIsDeployed)
	ctx.Step(`^SonataFlowPlatform with postgres config is deployed$`, data.sonataFlowPlatformWithDataIndexIsDeployed)
	ctx.Step(`^SonataFlowPlatform with DataIndexAndJobsService using Postgres is deployed$`, data.sonataFlowPlatformWithDataIndexAndJobsServiceUsingPostgresIsDeployed)
	ctx.Step(`^SonataFlowPlatform with DataIndexAndJobsService using Postgres and DB migration strategy is deployed$`, data.sonataFlowPlatformWithDataIndexAndJobsServiceWithDBMigrationIsDeployed)
}

func (data *Data) sonataFlowPlatformIsDeployed() error {
	projectDir, _ := utils.GetProjectDir()
	projectDir = strings.Replace(projectDir, "/testbdd", "", -1)

	// TODO or kubectl
	out, err := framework.CreateCommand("oc", "apply", "-f", filepath.Join(projectDir, getSonataFlowPlatformFilename()), "-n", data.Namespace).Execute()

	if err != nil {
		framework.GetLogger(data.Namespace).Error(err, fmt.Sprintf("Applying SonataFlowPlatform failed, output: %s", out))
	}

	return err
}

func (data *Data) sonataFlowPlatformWithDataIndexIsDeployed() error {
	projectDir, _ := utils.GetProjectDir()
	projectDir = strings.Replace(projectDir, "/testbdd", "", -1)

	// TODO or kubectl
	out, err := framework.CreateCommand("oc", "apply", "-f",
		filepath.Join(projectDir, getSonataFlowPlatformFilename()),
		"-n",
		data.Namespace).Execute()

	if err != nil {
		framework.GetLogger(data.Namespace).Error(err, fmt.Sprintf("Applying SonataFlowPlatform failed, output: %s", out))
	}

	return err
}

func (data *Data) sonataFlowPlatformWithDataIndexAndJobsServiceWithDBMigrationIsDeployed() error {
	projectDir, _ := utils.GetProjectDir()
	projectDir = strings.Replace(projectDir, "/testbdd", "", -1)

	out, err := framework.CreateCommand("oc", "apply", "-f",
		filepath.Join(projectDir, test.GetSFPlatformWithDIandJSWithDBMigration()),
		"-n",
		data.Namespace).Execute()
	if err != nil {
		return fmt.Errorf("applying SonataFlowPlatform WithDataIndexAndJobsServiceWithDBMigration failed, output: %s: %w", out, err)
	}

	if jobServiceDepErr := framework.WaitForDeploymentRunning(data.Namespace, "sonataflow-platform-jobs-service", 1, 5); jobServiceDepErr != nil {
		return fmt.Errorf("jobs-service deployment did not become ready: %w", jobServiceDepErr)
	}

	if dataIndexDepErr := framework.WaitForDeploymentRunning(data.Namespace, "sonataflow-platform-data-index-service", 1, 8); dataIndexDepErr != nil {
		return fmt.Errorf("data-index-service deployment did not become ready: %w", dataIndexDepErr)
	}

	return nil
}

func (data *Data) sonataFlowPlatformWithDataIndexAndJobsServiceUsingPostgresIsDeployed() error {
	projectDir, _ := utils.GetProjectDir()
	projectDir = strings.Replace(projectDir, "/testbdd", "", -1)

	// TODO or kubectl
	out, err := framework.CreateCommand("oc", "apply", "-f",
		filepath.Join(projectDir, test.GetSFPlatformWithDIandJSUsingPostgres()),
		"-n",
		data.Namespace).Execute()

	if jobServiceDepErr := framework.WaitForDeploymentRunning(data.Namespace, "sonataflow-platform-jobs-service", 1, 2); jobServiceDepErr != nil {
		framework.GetLogger(data.Namespace).Error(jobServiceDepErr, fmt.Sprintf("jobs-service deployment did not become ready, output: %s", out))
	}

	if dataIndexDepErr := framework.WaitForDeploymentRunning(data.Namespace, "sonataflow-platform-data-index-service", 1, 2); dataIndexDepErr != nil {
		framework.GetLogger(data.Namespace).Error(dataIndexDepErr, fmt.Sprintf("data-index-service deployment did not become ready, output: %s", out))
	}

	if err != nil {
		framework.GetLogger(data.Namespace).Error(err, fmt.Sprintf("Applying SonataFlowPlatform WithDataIndexAndJobsServiceUsingPostgres failed, output: %s", out))
	}

	return err
}

func getSonataFlowPlatformFilename() string {
	if getClusterPlatform() == openshiftPlatform {
		return test.GetPlatformOpenshiftE2eTest()
	}
	return test.GetPlatformMinikubeE2eTest()
}

func getClusterPlatform() string {
	if v, ok := os.LookupEnv("CLUSTER_PLATFORM"); ok {
		return v
	}
	return minikubePlatform
}
