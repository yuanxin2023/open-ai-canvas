package canvas

import (
	"encoding/json"
	"net/http"

	"infinite-canvas/backend/internal/canvas/previslegacy"
	"infinite-canvas/backend/internal/kernel"
)

func NormalizePrevisDocument(raw json.RawMessage) (json.RawMessage, error) {
	result, err := previslegacy.Normalize(raw)
	if err != nil {
		return nil, previsMigrationError(err)
	}
	return result, nil
}

func previsMigrationError(cause error) error {
	err := kernel.NewAppError(http.StatusConflict, cause.Error())
	err.Reason = "canvas_previs_migration_conflict"
	return err
}

func validateMigratedPrevisReferences(raw json.RawMessage) error {
	if err := previslegacy.ValidateReferences(raw); err != nil {
		return previsMigrationError(err)
	}
	return nil
}
