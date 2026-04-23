#!/bin/bash

export DEBUG=true

rm -rf testbdd/logs/

go run testbdd/scripts/prune_namespaces.go

./hack/clean-cluster-operators.sh
./hack/clean-crds.sh

# Configuration variables (change these to match your actual test registry/tags)
REGISTRY="registry-proxy.engineering.redhat.com/rh-osbs"
OPERATOR_TAG="openshift-serverless-nightly-rhel-9-containers-candidate-69005-20260325162017"
JOBSSERVICE_POSTGESQL_TAG="openshift-serverless-nightly-rhel-9-containers-candidate-18542-20260325161846"
JOBSSERVICE_EPHEMERAL_TAG="openshift-serverless-nightly-rhel-9-containers-candidate-45012-20260325161758"
DATAINDEX_POSTGESQL_TAG="openshift-serverless-nightly-rhel-9-containers-candidate-71465-20260325161709"
DATAINDEX_EPHEMERAL_TAG="openshift-serverless-nightly-rhel-9-containers-candidate-58554-20260325161807"
DBMIGRATOR_TAG="openshift-serverless-nightly-rhel-9-containers-candidate-65986-20260325161934"
BUIDLER_TAG="openshift-serverless-nightly-rhel-9-containers-candidate-71914-20260325161933"
DEVMODE_TAG="openshift-serverless-nightly-rhel-9-containers-candidate-95367-20260325162027"

echo "🚀 Running Logic Operator BDD tests with custom related NIGHTLY images..."

# Execute the Godog test suite and pass the CLI arguments
go test ./testbdd/... -v --godog.tags="installation" --godog.format=junit --godog.format=pretty -args \
  --tests.operator_image_tag="${REGISTRY}/openshift-serverless-1-logic-rhel9-operator:${OPERATOR_TAG}" \
  --operator.related_image.jobs_service_postgresql="${REGISTRY}/openshift-serverless-1-logic-jobs-service-postgresql-rhel9:${JOBSSERVICE_POSTGESQL_TAG}" \
  --operator.related_image.jobs_service_ephemeral="${REGISTRY}/openshift-serverless-1-logic-jobs-service-ephemeral-rhel9:${JOBSSERVICE_EPHEMERAL_TAG}" \
  --operator.related_image.data_index_postgresql="${REGISTRY}/openshift-serverless-1-logic-data-index-postgresql-rhel9:${DATAINDEX_POSTGESQL_TAG}" \
  --operator.related_image.data_index_ephemeral="${REGISTRY}/openshift-serverless-1-logic-data-index-ephemeral-rhel9:${DATAINDEX_EPHEMERAL_TAG}" \
  --operator.related_image.db_migrator_tool="${REGISTRY}/openshift-serverless-1-logic-db-migrator-tool-rhel9:${DBMIGRATOR_TAG}" \
  --operator.related_image.base_builder="${REGISTRY}/openshift-serverless-1-logic-swf-builder-rhel9:${BUIDLER_TAG}" \
  --operator.related_image.devmode="${REGISTRY}/openshift-serverless-1-logic-swf-devmode-rhel9:${DEVMODE_TAG}" \
  --tests.cr_deployment_only


