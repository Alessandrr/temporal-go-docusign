package main

import (
	"context"
	"log"

	"github.com/alessandrr/temporal-go-docusign/docusign"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

func main() {
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: "nda",
		ID:        "nda-workflow-" + uuid.NewString(),
	}

	we, err := c.ExecuteWorkflow(context.Background(), workflowOptions, docusign.SendNdaWorkflow)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	log.Println("Started workflow")

	var result string
	err = we.Get(context.Background(), &result)
	if err != nil {
		log.Fatalln("Unable to get wf result", err)
	}

	log.Println("Workflow result:", result)
}
