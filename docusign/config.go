package docusign

import (
	"log"
	"os"
)

type DocusignConfig struct {
	ClientID    string
	BaseAuthURL string
	PrivateKey  []byte
}

func LoadConfig() *DocusignConfig {
	privateKey, err := os.ReadFile("../private.pem")
	if err != nil {
		log.Fatalf("Error reading private key: %v", err)
	}

	baseAuthURL := "https://account.docusign.com"

	if os.Getenv("APP_ENV") == "dev" {
		baseAuthURL = "https://account-d.docusign.com"
	}

	return &DocusignConfig{
		ClientID:    os.Getenv("DOCUSIGN_CLIENT_ID"),
		BaseAuthURL: baseAuthURL,
		PrivateKey:  privateKey,
	}
}
