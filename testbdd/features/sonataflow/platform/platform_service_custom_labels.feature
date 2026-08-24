Feature: SonataFlowPlatform services propagate custom pod-template labels to Deployments
  As a platform engineer
  I want to set custom labels on the Data Index and Job Service pod templates in SonataFlowPlatform
  So that the operator propagates those labels to the managed Deployments
  and external tooling (e.g. network-policy controllers) can select the pods

  # Run with:
  #   --godog.tags=@platform
  #   --tests.cr_deployment_only=true
  #   --tests.operator_installation_source=olm
  #   --tests.operator_catalog_image=registry-proxy.engineering.redhat.com/rh-osbs/iib:1196314
  #   --tests.operator.version=1.39.0

  Background:
    Given SonataFlow Operator is installed via OLM
    Given Namespace is created
    When Postgres is deployed
    When SonataFlowPlatform with DataIndexAndJobsService using Postgres is deployed

  @platform
  Scenario: Custom labels defined in podTemplate.metadata are propagated to Data Index and Job Service Deployments
    # The platform YAML sets:
    #   spec.services.dataIndex.podTemplate.metadata.labels:
    #     osl.logic.com/securityZone: api
    #   spec.services.jobService.podTemplate.metadata.labels:
    #     osl.logic.com/securityZone: api
    #
    # The operator must copy these labels onto the pod template of each managed
    # Deployment so that external controllers (network-policy, admission webhooks,
    # monitoring selectors) can target the pods via label selectors.
    Then Deployment "sonataflow-platform-data-index-service" has label "osl.logic.com/securityZone" with value "api"
    Then Deployment "sonataflow-platform-data-index-service" pods have label "osl.logic.com/securityZone" with value "api"
    Then Deployment "sonataflow-platform-jobs-service" has label "osl.logic.com/securityZone" with value "api"
    Then Deployment "sonataflow-platform-jobs-service" pods have label "osl.logic.com/securityZone" with value "api"
