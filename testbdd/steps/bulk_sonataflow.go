package steps

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/cucumber/godog"
	"github.com/kubesmarts/operator-bdd-test/bddframework/pkg/framework"
)

func registerBulkWorkflowSteps(ctx *godog.ScenarioContext, data *Data) {
	ctx.Step(`^(\d+) greeting workflows are deployed$`, data.deployBulkWorkflows)
}

func (data *Data) deployBulkWorkflows(count int) error {
	framework.GetLogger(data.Namespace).Info("Generating bulk workflow deployment", "count", count)

	// 1. Define the template for a single SonataFlow CR in GitOps mode
	const workflowTemplate = `---
apiVersion: sonataflow.org/v1alpha08
kind: SonataFlow
metadata:
  name: greeting-{{.Index}}
  namespace: {{.Namespace}}
  annotations:
    sonataflow.org/profile: gitops
spec:
  podTemplate:
    container:
      image: quay.io/dhanak/greeting:1.38.0
  flow:
    start: ChooseOnLanguage
    functions:
      - name: greetFunction
        type: custom
        operation: sysout
    states:
      - name: ChooseOnLanguage
        type: switch
        dataConditions:
          - condition: "${$.[?(@.language  == 'English')]}"
            transition: GreetInEnglish
          - condition: "${$.[?(@.language  == 'Spanish')]}"
            transition: GreetInSpanish
        defaultCondition:
          transition: GreetInEnglish
      - name: GreetInEnglish
        type: inject
        data:
          greeting: 'Hello from YAML Workflow, '
        transition: GreetPerson
      - name: GreetInSpanish
        type: inject
        data:
          greeting: 'Saludos desde YAML Workflow, '
        transition: GreetPerson
      - name: GreetPerson
        type: operation
        actions:
          - name: greetAction
            functionRef:
              refName: greetFunction
              arguments:
                message: "$.greeting $.name"
        end:
          terminate: true
`
	tmpl, err := template.New("workflow").Parse(workflowTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %v", err)
	}

	// 2. Buffer to hold our massive multi-document YAML
	var yamlBuffer bytes.Buffer
	for i := 1; i <= count; i++ {
		err = tmpl.Execute(&yamlBuffer, map[string]interface{}{
			"Index":     i,
			"Namespace": data.Namespace, // Deploy into the current test scenario's namespace
		})
		if err != nil {
			return fmt.Errorf("failed to execute template for index %d: %v", i, err)
		}
	}

	// 3. Write it to a temporary file using your framework's utility
	tempFilePath, err := framework.CreateTemporaryFile("bulk-workflows-*.yaml", yamlBuffer.String())

	framework.CreateFile("./logs/", "bulk_workflows.yaml", yamlBuffer.String())
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %v", err)
	}

	// 4. Apply all 100 CRs in a single CLI command
	cli := "kubectl"
	if framework.IsOpenshift() {
		cli = "oc"
	}

	framework.GetLogger(data.Namespace).Info("Applying YAML to cluster...")
	output, err := framework.CreateCommand(cli, "apply", "-f", tempFilePath).Execute()
	if err != nil {
		return fmt.Errorf("failed to apply bulk workflows: %v, output: %s", err, output)
	}

	framework.GetLogger(data.Namespace).Info("Successfully applied workflows!")
	return nil
}
