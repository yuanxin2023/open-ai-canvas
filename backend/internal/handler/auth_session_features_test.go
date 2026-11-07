package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestAnonymousSessionProjectsInspirationFeatures(t *testing.T) {
	router, db, _ := profileHandlerFixture(t)
	for _, enabled := range []bool{true, false, true} {
		encoded, err := json.Marshal(map[string]bool{
			"inspirationHallEnabled": enabled, "inspirationSourcesVisible": !enabled,
			"creditsEnabled": true, "frontendModelsEnabled": true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Save(&model.SystemSetting{Key: "feature_availability", ValueJSON: string(encoded), UpdatedBy: "private-admin"}).Error; err != nil {
			t.Fatal(err)
		}
		for _, cookie := range []string{"", "invalid-session"} {
			response := profileRequest(router, http.MethodGet, "/api/auth/session", "", cookie)
			if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("unexpected session response: %d %s", response.Code, response.Body.String())
			}
			var body struct {
				Code int                        `json:"code"`
				Data map[string]json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Code != 0 || len(body.Data) != 2 || string(body.Data["user"]) != "null" {
				t.Fatalf("unexpected anonymous payload: %s", response.Body.String())
			}
			var features map[string]bool
			if err := json.Unmarshal(body.Data["features"], &features); err != nil {
				t.Fatal(err)
			}
			if len(features) != 2 || features["inspirationHallEnabled"] != enabled || features["inspirationSourcesVisible"] != !enabled {
				t.Fatalf("unexpected guest features: %s", response.Body.String())
			}
		}
	}
	if response := profileRequest(router, http.MethodGet, "/api/auth/profile", "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("guest gained profile access: %d", response.Code)
	}
}

func TestAnonymousSessionFeatureReadFailure(t *testing.T) {
	router, db, _ := profileHandlerFixture(t)
	if err := db.Save(&model.SystemSetting{Key: "feature_availability", ValueJSON: "{broken"}).Error; err != nil {
		t.Fatal(err)
	}
	response := profileRequest(router, http.MethodGet, "/api/auth/session", "", "")
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("configuration failure returned success: %d %s", response.Code, response.Body.String())
	}
}

func TestAuthenticatedSessionRetainsFullFeatures(t *testing.T) {
	router, _, cookie := profileHandlerFixture(t)
	response := profileRequest(router, http.MethodGet, "/api/auth/session", "", cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("authenticated session: %d %s", response.Code, response.Body.String())
	}
	var body struct {
		Data struct {
			User     *model.User    `json:"user"`
			Features map[string]any `json:"features"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.User == nil || body.Data.Features["creditsEnabled"] != true || len(body.Data.Features) <= 2 {
		t.Fatalf("authenticated features were reduced: %s", response.Body.String())
	}
}
