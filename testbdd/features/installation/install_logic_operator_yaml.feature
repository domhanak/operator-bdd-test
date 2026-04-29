Feature: Deploy Logic Operator with using YAML
  As a developer
  I want to install the Logic Operator using YAML
  So that I can verify the installation behaves expectedly

  @installation
  Scenario: Deploy the operator and verify it is running
    Given Namespace is created
    When SonataFlow Operator is deployed
    Then SonataFlow Operator has 1 pod running
    And Service "logic-operator-controller-manager-metrics-service" exists
    And ConfigMap "logic-operator-builder-config" exists
    And ConfigMap "logic-operator-controllers-config" exists
    And ConfigMap "logic-operator-builder-config" contains following strings:
      | FROM ${RELATED_IMAGE_BASE_BUILDER} AS builder                |
      | FROM registry.access.redhat.com/ubi9/openjdk-17-runtime:1.23 |
    When Postgres is deployed
    When SonataFlowPlatform with DataIndexAndJobsService using Postgres is deployed
    When SonataFlow callbackstatetimeouts example is deployed
    Then SonataFlow "callbackstatetimeouts" has the condition "Running" set to "True" within 5 minutes
    Then HTTP POST request on non-dev SonataFlow "callbackstatetimeouts" is successful within 1 minute with path "callbackstatetimeouts", expectedResponseContains '"workflowdata":{"message":"Hello"}"' and body:
    """json
    {"message": "Hello"
    }
    """