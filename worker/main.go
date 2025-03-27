package main

import (
	"log"
	"net/http"
	"time"

	"github.com/alessandrr/temporal-go-docusign/docusign"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	cache := docusign.NewDocusignKeysCache(1 * time.Minute)

	apiClient := docusign.NewDocusignAPIClient(http.Header{}, &http.Client{})
	authService := docusign.NewDocusignAuthService(cache, apiClient)
	activities := docusign.NewDocusignActivities(apiClient, authService)

	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatalln("Unable to create Temporal client", err)
	}

	defer c.Close()

	w := worker.New(c, "nda", worker.Options{})

	w.RegisterActivity(activities)
	w.RegisterWorkflow(docusign.SendNdaWorkflow)

	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start Temporal worker", err)
	}
}
