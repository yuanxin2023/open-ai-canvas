package submission

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"infinite-canvas/backend/internal/model"
)

// The version binds the source attempt, not a permission grant. Eligibility is
// checked again against current rows before a new task can commit.
func RetryVersion(task *model.Task) (string, error) {
	input := sha256.Sum256([]byte(task.InputJSON))
	prompt := sha256.Sum256([]byte(task.Prompt))
	return Fingerprint("workbench-retry", map[string]any{
		"id": task.ID, "userId": task.UserID, "type": task.Type, "kind": task.WorkbenchKind,
		"status": task.Status, "billingOrderId": task.BillingOrderID,
		"promptHash": hex.EncodeToString(prompt[:]), "model": task.Model, "logicalModelId": task.LogicalModelID, "projectId": task.ProjectID, "operation": task.Operation,
		"updatedAt": task.UpdatedAt.UTC().Format(time.RFC3339Nano), "inputHash": hex.EncodeToString(input[:]),
		"hasMediaRecovery": task.MediaRecoveryJSON != "", "providerCancelStatus": task.ProviderCancelStatus,
		"agentRunId": task.AgentRunID, "generationId": task.GenerationID, "creationSubmissionId": task.CreationSubmissionID,
	})
}
