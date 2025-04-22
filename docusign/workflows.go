package docusign

import (
	"encoding/json"
	"time"

	"go.temporal.io/sdk/workflow"
)

func SendNdaWorkflow(ctx workflow.Context) (string, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var activities *Activities

	var envelopeSummary EnvelopeSummary
	err := workflow.ExecuteActivity(ctx, activities.CreateNdaEnvelope, DocusignTestUser).Get(ctx, &envelopeSummary)
	if err != nil {
		return "", err
	}

	ndaFields := NdaTemplateFields{
		VendorName:  "Hello",
		VendorTaxID: "1234567890",
		SignerName:  "World",
	}
	data, err := json.Marshal(ndaFields)
	if err != nil {
		return "", err
	}

	ndaTemplateFields := &TemplateFieldWrapper{
		Type: "nda",
		Data: data,
	}

	err = workflow.ExecuteActivity(ctx, activities.FillTemplateFields, envelopeSummary, ndaTemplateFields, DocusignTestUser).Get(ctx, &envelopeSummary)
	if err != nil {
		return "", err
	}

	err = workflow.ExecuteActivity(ctx, activities.SendDraftEnvelope, envelopeSummary, DocusignTestUser).Get(ctx, &envelopeSummary)
	if err != nil {
		return "", err
	}

	var status string

	err = workflow.ExecuteChildWorkflow(ctx, WaitForSigningWorkflow, envelopeSummary.EnvelopeID).Get(ctx, &status)
	if err != nil {
		return "", err
	}

	return status, nil
}

func WaitForSigningWorkflow(ctx workflow.Context, envelopeID string) (string, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var activities *Activities

	var envelopeStatus EnvelopeStatus

	for {
		err := workflow.ExecuteActivity(ctx, activities.GetEnvelopeStatus, envelopeID, DocusignTestUser).Get(ctx, &envelopeStatus)
		if err != nil {
			return "", err
		}

		if envelopeStatus.Status == "completed" || envelopeStatus.Status == "voided" || envelopeStatus.Status == "declined" {
			break
		}

		workflow.Sleep(ctx, 1*time.Minute)
	}

	return envelopeStatus.Status, nil
}
