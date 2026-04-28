Feature: Deploy 100 workflows with one DataIndex using Postgres
  As a developer
  I want to deploy 100 workflows with DataIndex using Postgresq
  So that I can verify the deployment behaves expectedly

  @workflows100
  Scenario: Deploy the 100 workflows and verify 100 pods are running

  Given Namespace is created
  When Postgres is deployed
  When SonataFlowPlatform with DataIndexAndJobsService using Postgres is deployed
  When 100 greeting workflows are deployed  
  Then Wait for 100 pods to be running in the namespace
  