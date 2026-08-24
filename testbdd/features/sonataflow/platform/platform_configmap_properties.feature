Feature: Validate ConfigMap property content for Workflow, Data Index and Job Service

  Background:
    Given Namespace is created
    When Postgres is deployed
    When SonataFlowPlatform with DataIndexAndJobsService using Postgres is deployed

  @previewMode
  Scenario: Verify workflow ConfigMap properties after deploying callbackstatetimeouts
    When SonataFlow callbackstatetimeouts example is deployed
    Then SonataFlow "callbackstatetimeouts" has the condition "Running" set to "True" within 5 minutes
    Then ConfigMap "callbackstatetimeouts-props" exists
    Then ConfigMap "callbackstatetimeouts-props" is empty
    Then ConfigMap "callbackstatetimeouts-managed-props" exists
    Then ConfigMap "callbackstatetimeouts-managed-props" contains following strings:
      | kogito.data-index.health-enabled = true                                                          |
      | kogito.data-index.url                                                                            |
      | kogito.events.grouping = true                                                                    |
      | kogito.events.grouping.binary = true                                                             |
      | kogito.events.processdefinitions.enabled = true                                                  |
      | kogito.events.processdefinitions.errors.propagate = true                                         |
      | kogito.events.processinstances.enabled = true                                                    |
      | kogito.events.usertasks.enabled = false                                                          |
      | kogito.jobs-service.health-enabled = true                                                        |
      | kogito.jobs-service.url                                                                          |
      | mp.messaging.outgoing.kogito-job-service-job-request-events.connector = quarkus-http            |
      | mp.messaging.outgoing.kogito-job-service-job-request-events.url                                 |
      | mp.messaging.outgoing.kogito-processdefinitions-events.url                                      |
      | mp.messaging.outgoing.kogito-processinstances-events.url                                        |
      | org.kie.kogito.addons.knative.eventing.health-enabled = false                                   |
      | kogito.service.url                                                                               |
      | quarkus.http.port = 8080                                                                         |
      | quarkus.http.host = 0.0.0.0                                                                      |
      | quarkus.devservices.enabled = false                                                              |
      | quarkus.kogito.devservices.enabled = false                                                       |

  @previewMode
  Scenario: Verify Data Index Service ConfigMap properties
    Then ConfigMap "sonataflow-platform-data-index-service-props" exists
    Then ConfigMap "sonataflow-platform-data-index-service-props" contains following strings:
      | kogito.service.url                                                                              |
      | quarkus.devservices.enabled = false                                                             |
      | quarkus.http.host = 0.0.0.0                                                                     |
      | quarkus.http.port = 8080                                                                        |
      | quarkus.kogito.devservices.enabled = false                                                      |
      | quarkus.smallrye-health.check."io.quarkus.kafka.client.health.KafkaHealthCheck".enabled = false |

  @previewMode
  Scenario: Verify Job Service ConfigMap properties
    Then ConfigMap "sonataflow-platform-jobs-service-props" exists
    Then ConfigMap "sonataflow-platform-jobs-service-props" contains following strings:
      | kogito.jobs-service.http.job-status-change-events = true                                                                                    |
      | kogito.jobs-service.management.leader-check.expiration-in-seconds = 60                                                                     |
      | kogito.service.url                                                                                                                          |
      | mp.messaging.outgoing.kogito-job-service-job-status-events-http.url                                                                        |
      | sonataflow-platform-data-index-service                                                                                                      |
      | quarkus.devservices.enabled = false                                                                                                         |
      | quarkus.http.host = 0.0.0.0                                                                                                                 |
      | quarkus.http.port = 8080                                                                                                                    |
      | quarkus.kogito.devservices.enabled = false                                                                                                  |
      | quarkus.smallrye-health.check."io.quarkus.kafka.client.health.KafkaHealthCheck".enabled = false                                            |
      | quarkus.smallrye-health.check."org.kie.kogito.jobs.service.management.JobServiceLeaderLivenessHealthCheck".enabled = true                  |
      | quarkus.smallrye-health.check."org.kie.kogito.jobs.service.messaging.http.health.knative.KSinkInjectionHealthCheck".enabled = false        |
