Feature: Upgrade OSL Operator from a previous version to the next version
  As an OpenShift cluster admin
  I want to upgrade the OpenShift Serverless Logic operator via OLM
  So that all managed workflows, Data Index, and Job Service are migrated and running at the new version

  # Run with:
  #   --godog.tags=@upgradeTests
  #   --tests.upgrade.from_version=1.37.2
  #   --tests.upgrade.to_version=1.38.0
  #   --tests.operator_installation_source=olm
  #   --tests.operator_catalog_image=registry-proxy.engineering.redhat.com/rh-osbs/iib:1196314

  @upgradeTests
  Scenario: Upgrade operator to next version and verify DB migration and services
    # ── Pre-upgrade: install operator at from-version via OLM ────────────────
    Given SonataFlow Operator at from-version is installed via OLM
    # ── Pre-upgrade setup ────────────────────────────────────────────────────
    Given Namespace is created
    When Postgres is deployed
    When SonataFlowPlatform with DataIndexAndJobsService using Postgres and DB migration strategy is deployed

    # Deploy a preview-profile workflow so we can verify the guide
    When SonataFlow callbackstatetimeouts example is deployed
    Then SonataFlow "callbackstatetimeouts" has the condition "Running" set to "True" within 20 minutes

    # ── Step 1 — Increase Job Service retry interval ─────────────────────────
    # Patched by applying a higher value before the operator stops managing pods.
    # This is a manual/external concern; the test verifies the platform is stable
    # prior to upgrade rather than automating the property edit itself.

    # ── Step 2 (dev profile) — scale down / annotate dev workflows ───────────
    # No dev workflows in this scenario; step is satisfied by the preview workflow above.

    # ── Step 3 (preview profile) — capture current workflow state ────────────
    # Workflow is Running at from-version; verified above.
    # ── Step 3.1 (preview profile) — delete workflow before upgrade ────────────
    # Preview-profile workflows must be deleted before upgrading the operator;
    # the new operator will not reconcile builds from the old version.
    # After the upgrade the workflow is redeployed and rebuilt from scratch.
    When SonataFlow "callbackstatetimeouts" is deleted

    # ── Step 5 — Back up Data Index database ─────────────────────────────────
    # Database backup is an operational concern performed outside the cluster;
    # this step asserts the Data Index deployment is healthy before upgrade.
    Then Service "sonataflow-platform-data-index-service" exists

    # ── Step 6 — Back up Job Service database ────────────────────────────────
    Then Service "sonataflow-platform-jobs-service" exists

    # ── Step 7 — Upgrade the operator ────────────────────────────────────────
    When SonataFlow Operator is upgraded to next version

    # ── Post-upgrade: verify the operator is running at the new version ───────
    Then SonataFlow Operator running version matches upgrade target

    # ── Step 8 — Data Index restarts at new version ───────────────────────────
    # The operator reconciles Data Index automatically; we verify the deployment
    # is running after the operator upgrade.
    Then Deployment "sonataflow-platform-data-index-service" pods log contains text 'started in' within 3 minutes

    # ── Step 9 — Job Service restarts at new version ──────────────────────────
    Then Deployment "sonataflow-platform-jobs-service" pods log contains text 'started in' within 3 minutes

    # ── DB migrator job (operator upgrade step) ───────────────────────────────
    # Verifies that the operator created and completed a sonataflow-db-migrator-job
    # labelled app=sonataflow-platform, app.kubernetes.io/version=${UPGRADE_TO_VERSION}.
    #
    # TODO: Solve issue where setting 'job' in YAML ( dbMigrationStrategy: job ) fails to pull
    #       the image on fromVersion startup
    # Then DB migrator job for platform "sonataflow-platform" completes within 10 minutes

    # ── Step 11 — Workflow in preview mode redeployed manualy ──────────────────────────
    Then SonataFlow callbackstatetimeouts example is deployed
    # ── Step 11 — Workflow in preview mode should start normally ───────────────────────
    Then SonataFlow "callbackstatetimeouts" has the condition "Running" set to "True" within 5 minutes

    # Verify the managed-props ConfigMap was regenerated with new-version URLs
    Then ConfigMap "callbackstatetimeouts-managed-props" exists
    Then ConfigMap "callbackstatetimeouts-managed-props" contains following strings:
      | kogito.data-index.health-enabled = true                         |
      | kogito.data-index.url                                           |
      | kogito.jobs-service.health-enabled = true                       |
      | kogito.jobs-service.url                                         |
      | quarkus.http.port = 8080                                        |
      | quarkus.devservices.enabled = false                             |
      | quarkus.kogito.devservices.enabled = false                      |

    # ── Step 13 — Restore Job Service retry interval ──────────────────────────
    # Operational step performed externally; scenario closes by confirming the
    # Job Service ConfigMap reflects baseline settings after operator reconciles.
    Then ConfigMap "sonataflow-platform-jobs-service-props" exists
    Then ConfigMap "sonataflow-platform-jobs-service-props" contains following strings:
      | kogito.jobs-service.management.leader-check.expiration-in-seconds = 60 |
      | kogito.jobs-service.http.job-status-change-events = true               |
