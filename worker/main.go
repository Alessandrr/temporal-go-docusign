package main

import (
	"log"
	"net/http"
	"time"

	"github.com/alessandrr/temporal-go-docusign/docusign"
	"github.com/joho/godotenv"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	cache := docusign.NewKeysCache(1 * time.Minute)
	defer cache.Stop()

	apiClient := docusign.NewAPIClient(http.Header{}, &http.Client{})
	authService := docusign.NewAuthService(cache, apiClient, docusign.LoadConfig())
	activities := docusign.NewActivities(apiClient, authService)

	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatalln("Unable to create Temporal client", err)
	}

	defer c.Close()

	w := worker.New(c, "nda", worker.Options{})

	w.RegisterActivity(activities)
	w.RegisterWorkflow(docusign.SendNdaWorkflow)
	w.RegisterWorkflow(docusign.WaitForSigningWorkflow)

	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start Temporal worker", err)
	}
}
