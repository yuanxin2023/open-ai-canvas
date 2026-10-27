package canvas

import (
	"encoding/json"
	"strings"

	"infinite-canvas/backend/internal/canvas/previslegacy"
)

// This projection is deliberately applied after receipt lookup/digest checks.
// Valid queued requests keep their original mutation identity and wire payload.
func migratePrevisOperations(req CanvasOperationsRequest) (CanvasOperationsRequest, bool, error) {
	converted := req
	converted.Operations = append([]CanvasOperation(nil), req.Operations...)
	changed := false
	for index, original := range req.Operations {
		op := original
		if original.Type == "create_canvas" {
			op.Patch = make(map[string]json.RawMessage, len(original.Patch))
			for key, value := range original.Patch {
				op.Patch[key] = value
			}
			if old, exists := original.Patch["directorScenes"]; exists {
				value, err := previslegacy.Scenes(old)
				if err != nil {
					return req, false, previsMigrationError(err)
				}
				if next, exists := original.Patch["previsScenes"]; exists && !jsonEqual(rawJSON(next), rawJSON(old)) && !jsonEqual(rawJSON(next), rawJSON(value)) {
					return req, false, previsMigrationError(&previslegacy.Error{Path: "create_canvas.previsScenes"})
				}
				op.Patch["previsScenes"] = value
				delete(op.Patch, "directorScenes")
				changed = true
			}
		} else if original.Type == "set_field" && original.Field == "directorScenes" {
			op.Field = "previsScenes"
			for _, field := range []*json.RawMessage{&op.Value, &op.BeforeValue} {
				if len(*field) == 0 || string(*field) == "null" {
					continue
				}
				value, err := previslegacy.Scenes(*field)
				if err != nil {
					return req, false, previsMigrationError(err)
				}
				*field = value
			}
			changed = true
		}
		for _, field := range []*json.RawMessage{&op.Node, &op.Expected} {
			if len(*field) == 0 {
				continue
			}
			value, updated, err := previslegacy.Node(*field)
			if err != nil {
				return req, false, previsMigrationError(err)
			}
			if updated {
				*field, changed = value, true
			}
		}
		if original.Type == "patch_node" {
			for _, field := range []*map[string]json.RawMessage{&op.Patch, &op.Before} {
				values := make(map[string]json.RawMessage, len(*field))
				for path, value := range *field {
					nextPath := path
					if strings.HasPrefix(path, "metadata.") {
						nextPath = "metadata." + previslegacy.CanonicalMetadataField(strings.TrimPrefix(path, "metadata."))
					}
					if path == "metadata" {
						converted, updated, err := previslegacy.Metadata(value)
						if err != nil {
							return req, false, previsMigrationError(err)
						}
						value, changed = converted, changed || updated
					}
					if previous, exists := values[nextPath]; exists && !jsonEqual(rawJSON(previous), rawJSON(value)) {
						return req, false, previsMigrationError(&previslegacy.Error{Path: "patch_node." + nextPath})
					}
					values[nextPath] = value
					changed = changed || nextPath != path
				}
				*field = values
			}
		}
		converted.Operations[index] = op
	}
	return converted, changed, nil
}
