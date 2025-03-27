package docusign

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

func SendNdaWorkflow(ctx workflow.Context) (string, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var DocusignActivities *DocusignActivities

	var envelopeSummary EnvelopeSummary
	err := workflow.ExecuteActivity(ctx, DocusignActivities.CreateNdaEnvelope, DocusignTestUser).Get(ctx, &envelopeSummary)
	if err != nil {
		return "", err
	}

	return envelopeSummary.EnvelopeID, nil
}
