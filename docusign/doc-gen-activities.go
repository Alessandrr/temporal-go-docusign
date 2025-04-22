package docusign

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const templateFieldsPath = "/restapi/v2.1/accounts/%s/envelopes/%s/docGenFormFields/"

type TemplateFieldWrapper struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type FormFieldsDTO struct {
	Documents []DocumentFields `json:"docGenFormFields"`
}

type DocumentFields struct {
	DocumentID string      `json:"documentId"`
	Fields     []FormField `json:"docGenFormFieldList"`
}

type FormFields struct {
	DocumentID string      `json:"documentId"`
	FieldList  []FormField `json:"docGenFormFieldList"`
}

type FormField struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Required string `json:"required"`
}

type EnvelopeTemplateFieldGenerator interface {
	GenerateTemplateFields() FormFieldsDTO
	SetDocumentID(documentID string)
}

type NdaTemplateFields struct {
	VendorName  string `json:"vendorName"`
	SignerName  string `json:"signerName"`
	VendorTaxID string `json:"vendorTaxId"`
	DocumentID  string `json:"documentId"`
}

func (f *NdaTemplateFields) GenerateTemplateFields() FormFieldsDTO {
	return FormFieldsDTO{
		Documents: []DocumentFields{
			{
				DocumentID: f.DocumentID,
				Fields: []FormField{
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

func (s *Activities) FillTemplateFields(envelopeSummary EnvelopeSummary, wrapper *TemplateFieldWrapper, user DocusignUser) error {
	var generator EnvelopeTemplateFieldGenerator

	switch wrapper.Type {
	case "nda":
		var nda NdaTemplateFields
		if err := json.Unmarshal(wrapper.Data, &nda); err != nil {
			return fmt.Errorf("failed to unmarshal NDA template fields: %v", err)
		}
		generator = &nda
	default:
		return fmt.Errorf("unknown template type: %s", wrapper.Type)
	}

	authInfo, err := s.authUpdater.UpdateAuthInfo(user)
	if err != nil {
		return err
	}

	envelopeFields, err := s.getTemplateFields(envelopeSummary, user)
	if err != nil {
		return err
	}

	if len(envelopeFields.Documents) == 0 {
		return fmt.Errorf("no documents found for envelope %s", envelopeSummary.EnvelopeID)
	} else if len(envelopeFields.Documents) > 1 {
		fmt.Printf("multiple documents found for envelope %s which is unexpected", envelopeSummary.EnvelopeID)
	}

	remoteDocument := envelopeFields.Documents[0]
	generator.SetDocumentID(remoteDocument.DocumentID)
	formFieldsRequest := generator.GenerateTemplateFields()

	if len(formFieldsRequest.Documents) == 0 {
		return fmt.Errorf("no documents generated for envelope %s", envelopeSummary.EnvelopeID)
	} else if len(formFieldsRequest.Documents) > 1 {
		fmt.Printf("multiple documents generated for envelope %s which is unexpected", envelopeSummary.EnvelopeID)
	}

	generatedFields := formFieldsRequest.Documents[0].Fields

	if err := s.validateFields(remoteDocument.Fields, generatedFields); err != nil {
		return err
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

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to fill template fields: status=%s body=%s", resp.Status, string(bodyBytes))
	}

	return nil
}

func (s *Activities) validateFields(requiredFields []FormField, filledFields []FormField) error {
	requiredMap := make(map[string]bool)
	for _, field := range requiredFields {
		requiredMap[field.Name] = field.Required == "true"
	}

	filledMap := make(map[string]bool)
	for _, field := range filledFields {
		filledMap[field.Name] = field.Value != ""
	}

	missingFields := make([]string, 0)
	for fieldName, required := range requiredMap {
		if required && !filledMap[fieldName] {
			missingFields = append(missingFields, fieldName)
		}
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("missing required fields: %v", missingFields)
	}

	return nil
}

func (s *Activities) getTemplateFields(envelopeSummary EnvelopeSummary, user DocusignUser) (FormFieldsDTO, error) {
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

	if resp.StatusCode != http.StatusOK {
		return FormFieldsDTO{}, fmt.Errorf("%s: failed to get template fields: %s", resp.Status, resp.Body)
	}

	var formFields FormFieldsDTO
	err = json.NewDecoder(resp.Body).Decode(&formFields)
	if err != nil {
		return FormFieldsDTO{}, err
	}

	return formFields, nil
}
