package docusign

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const templateFieldsPath = "/restapi/v2.1/accounts/%s/envelopes/%s/docGenFormFields/"

type DocGenService struct {
	apiClient   *APIClient
	authUpdater AuthInfoUpdater
}

type FormFieldsDTO struct {
	Fields []FormFields `json:"docGenFormFields"`
}

type FormFields struct {
	DocumentID string      `json:"documentId"`
	FieldList  []FormField `json:"docGenFormFieldList"`
}

type FormField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func NewDocGenService(apiClient *APIClient, authUpdater AuthInfoUpdater) *DocGenService {
	return &DocGenService{
		apiClient:   apiClient,
		authUpdater: authUpdater,
	}
}

type EnvelopeTemplateFieldGenerator interface {
	GenerateTemplateFields() FormFieldsDTO
	SetDocumentID(documentID string)
}

type NdaTemplateFields struct {
	VendorName  string
	SignerName  string
	VendorTaxID string
	DocumentID  string
}

func (f *NdaTemplateFields) GenerateTemplateFields() FormFieldsDTO {
	return FormFieldsDTO{
		Fields: []FormFields{
			{
				DocumentID: f.DocumentID,
				FieldList: []FormField{
					{
						Name:  "vendorName",
						Value: f.VendorName,
					},
					{
						Name:  "signerName",
						Value: f.SignerName,
					},
					{
						Name:  "vendorTaxId",
						Value: f.VendorTaxID,
					},
				},
			},
		},
	}
}

func (f *NdaTemplateFields) SetDocumentID(documentID string) {
	f.DocumentID = documentID
}

func (s *DocGenService) FillTemplateFields(envelopeSummary EnvelopeSummary, generator EnvelopeTemplateFieldGenerator, user DocusignUser) error {
	authInfo, err := s.authUpdater.UpdateAuthInfo(user)
	if err != nil {
		return err
	}

	currentFields, err := s.getTemplateFields(envelopeSummary, user)
	if err != nil {
		return err
	}

	generator.SetDocumentID(currentFields.Fields[0].DocumentID)

	formFieldsRequest := generator.GenerateTemplateFields()
	requiredFields := make(map[string]bool)
	for _, field := range currentFields.Fields[0].FieldList {
		requiredFields[field.Name] = true
	}

	filledFields := make(map[string]bool)
	for _, field := range formFieldsRequest.Fields[0].FieldList {
		filledFields[field.Name] = true
	}

	missingFields := make([]string, 0)
	for fieldName := range requiredFields {
		if !filledFields[fieldName] {
			missingFields = append(missingFields, fieldName)
		}
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("missing required fields: %v", missingFields)
	}

	formFieldsRequestJSON, err := json.Marshal(formFieldsRequest)
	if err != nil {
		return err
	}

	fmt.Printf("Form fields request: %v \n", string(formFieldsRequestJSON))

	req, err := http.NewRequest(
		"PUT",
		fmt.Sprintf(s.apiClient.BaseURL+templateFieldsPath, authInfo.AccountId, envelopeSummary.EnvelopeID),
		bytes.NewBuffer(formFieldsRequestJSON),
	)
	if err != nil {
		return err
	}

	req.Header = s.apiClient.AuthHeader.Clone()
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.apiClient.Client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read error response: %v", err)
		}
		return fmt.Errorf("%s: failed to fill template fields: %s", resp.Status, string(bodyBytes))
	}

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Failed to read response body: %v", err)
	}

	return nil
}

func (s *DocGenService) getTemplateFields(envelopeSummary EnvelopeSummary, user DocusignUser) (FormFieldsDTO, error) {
	authInfo, err := s.authUpdater.UpdateAuthInfo(user)
	if err != nil {
		return FormFieldsDTO{}, err
	}

	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf(s.apiClient.BaseURL+templateFieldsPath, authInfo.AccountId, envelopeSummary.EnvelopeID),
		nil,
	)
	if err != nil {
		return FormFieldsDTO{}, err
	}

	req.Header = s.apiClient.AuthHeader.Clone()
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.apiClient.Client.Do(req)
	if err != nil {
		return FormFieldsDTO{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return FormFieldsDTO{}, fmt.Errorf("%s: failed to get template fields: %s", resp.Status, resp.Body)
	}

	var formFields FormFieldsDTO
	err = json.NewDecoder(resp.Body).Decode(&formFields)
	if err != nil {
		return FormFieldsDTO{}, err
	}

	return formFields, nil
}
