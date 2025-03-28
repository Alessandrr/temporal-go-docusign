package docusign

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type ndaTemplateId string

const (
	US_NDA_TEMPLATE_ID ndaTemplateId = "ad54810f-f5b2-435a-9f9b-3741cbb3a3a3"
)

type Activities struct {
	apiClient   *APIClient
	authUpdater AuthInfoUpdater
}

func NewActivities(apiClient *APIClient, authUpdater AuthInfoUpdater) *Activities {
	return &Activities{
		apiClient:   apiClient,
		authUpdater: authUpdater,
	}
}

const envelopesPath = "/restapi/v2.1/accounts/%s/envelopes/"

type EnvelopeStatus struct {
	Status string `json:"status"`
}

type EnvelopeTemplateDefinition struct {
	TemplateID    string          `json:"templateId"`
	Status        string          `json:"status"`
	TemplateRoles []TemplateRoles `json:"templateRoles"`
}

type TemplateRoles struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	RoleName string `json:"roleName"`
}

type EnvelopeDefinition struct {
	Status       string             `json:"status"`
	EmailSubject string             `json:"emailSubject"`
	Documents    []DocusignDocument `json:"documents"`
	Recipients   DocusignRecipients `json:"recipients"`
	EnvelopeID   string             `json:"envelopeId"`
}

type DocusignDocument struct {
	DocumentBase64 string `json:"documentBase64"`
	DocumentID     string `json:"documentId"`
	FileExtension  string `json:"fileExtension"`
	Name           string `json:"name"`
}

type DocusignRecipients struct {
	CarbonCopies []DocusignRecipient `json:"carbonCopies"`
	Signers      []DocusignRecipient `json:"signers"`
}

type DocusignRecipient struct {
	Email       string `json:"email"`
	Name        string `json:"name"`
	RecipientID string `json:"recipientId"`
}

type EnvelopeSummary struct {
	Status     string `json:"status"`
	EnvelopeID string `json:"envelopeId"`
}

func (s *Activities) CreateNdaEnvelope(ctx context.Context, user DocusignUser) (EnvelopeSummary, error) {
	authInfo, err := s.authUpdater.UpdateAuthInfo(user)
	if err != nil {
		return EnvelopeSummary{}, err
	}

	templateDefinition := EnvelopeTemplateDefinition{
		TemplateID: string(US_NDA_TEMPLATE_ID),
		Status:     "created",
		TemplateRoles: []TemplateRoles{
			{
				Email:    "alessandrr@my.com",
				Name:     "Alessandro",
				RoleName: "Vendor",
			},
		},
	}

	templateDefinitionJSON, err := json.Marshal(templateDefinition)
	if err != nil {
		return EnvelopeSummary{}, err
	}

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf(s.apiClient.BaseURL+envelopesPath, authInfo.AccountId),
		bytes.NewBuffer(templateDefinitionJSON),
	)
	if err != nil {
		return EnvelopeSummary{}, err
	}

	req.Header = s.apiClient.AuthHeader.Clone()
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.apiClient.Client.Do(req)
	if err != nil {
		return EnvelopeSummary{}, err
	}

	defer resp.Body.Close()

	var envelopeSummary EnvelopeSummary
	err = json.NewDecoder(resp.Body).Decode(&envelopeSummary)
	if err != nil {
		return EnvelopeSummary{}, err
	}

	return envelopeSummary, nil
}

func (s *Activities) SendDraftEnvelope(ctx context.Context, envelopeSummary EnvelopeSummary, user DocusignUser) (EnvelopeSummary, error) {
	authInfo, err := s.authUpdater.UpdateAuthInfo(user)
	if err != nil {
		return EnvelopeSummary{}, err
	}

	updateBody := EnvelopeStatus{
		Status: "sent",
	}

	updateBodyJSON, err := json.Marshal(updateBody)
	if err != nil {
		return EnvelopeSummary{}, err
	}

	req, err := http.NewRequest(
		"PUT",
		fmt.Sprintf(s.apiClient.BaseURL+envelopesPath+envelopeSummary.EnvelopeID, authInfo.AccountId),
		bytes.NewBuffer(updateBodyJSON),
	)
	if err != nil {
		return EnvelopeSummary{}, err
	}

	req.Header = s.apiClient.AuthHeader.Clone()
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.apiClient.Client.Do(req)
	if err != nil {
		return EnvelopeSummary{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return EnvelopeSummary{}, fmt.Errorf("%s: failed to send draft envelope: %s", resp.Status, resp.Body)
	}

	return envelopeSummary, nil
}
