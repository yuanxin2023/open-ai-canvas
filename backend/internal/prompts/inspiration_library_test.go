package prompts

import (
	"testing"

	"infinite-canvas/backend/internal/model"
)

func libraryCommand(revision int64, operation, id string) InspirationLibraryCommand {
	return InspirationLibraryCommand{ExpectedUserID: "owner", ExpectedRevision: &revision, Operation: operation, ID: id}
}

func adoptedInspirationID() string {
	for id := range inspirationCatalogIDs {
		return id
	}
	panic("empty catalog")
}

func TestInspirationLibraryReusesPersonalPromptsAndProtectsRevisions(t *testing.T) {
	svc, db := newUserPromptTestServiceWithDB(t)
	old, err := svc.CreateUserPrompt("owner", UserPromptRequest{Title: "Existing", Mode: model.InspirationModeImage, Prompt: "Keep original"})
	if err != nil {
		t.Fatal(err)
	}
	view, err := svc.InspirationLibrary("owner")
	if err != nil || len(view.Prompts) != 1 || view.Prompts[0].ID != old.ID {
		t.Fatalf("personal identity lost: %#v / %v", view, err)
	}
	translation := libraryCommand(view.Revision, "save", customInspirationPrefix+old.ID)
	translation.Draft = &InspirationDraft{Title: old.Title, Mode: old.Mode, Prompt: old.Prompt, PromptZh: "保留原中文版本"}
	view, err = svc.ApplyInspirationLibrary("owner", translation)
	if err != nil {
		t.Fatal(err)
	}
	stale := view.Revision
	if _, err := svc.UpdateUserPrompt("owner", old.ID, UserPromptRequest{Title: "Changed elsewhere", Mode: model.InspirationModeImage, Prompt: "New original"}); err != nil {
		t.Fatal(err)
	}
	command := libraryCommand(stale, "save", customInspirationPrefix+old.ID)
	command.Draft = &InspirationDraft{Title: "Stale overwrite", Mode: model.InspirationModeImage, Prompt: "Old draft"}
	if _, err := svc.ApplyInspirationLibrary("owner", command); appErrorStatus(err) != 409 {
		t.Fatalf("stale save = %v", err)
	}
	view, err = svc.InspirationLibrary("owner")
	if err != nil || view.Prompts[0].Prompt != "New original" {
		t.Fatalf("newer edit was overwritten: %v", err)
	}
	if metadata := view.State.Metadata[old.ID]; metadata.PromptZh != "保留原中文版本" || !metadata.PromptZhNeedsReview {
		t.Fatalf("secondary draft was lost or silently reused: %#v", metadata)
	}
	foreign, err := svc.InspirationLibrary("other")
	if err != nil || len(foreign.Prompts) != 0 {
		t.Fatalf("foreign read: %#v / %v", foreign, err)
	}
	var roots int64
	if err := db.Model(&model.UserInspirationLibrary{}).Count(&roots).Error; err != nil || roots != 1 {
		t.Fatalf("GET created a state row: %d / %v", roots, err)
	}
}

func TestInspirationHideAndFavoriteAreIdempotentAndPreserveCopies(t *testing.T) {
	svc := newUserPromptTestService(t)
	id := adoptedInspirationID()
	command := libraryCommand(0, "save", id)
	command.Draft = &InspirationDraft{Title: "Personal edit", Image: "/inspiration-thumbs/review.webp", Mode: model.InspirationModeImage, Prompt: "Private body"}
	view, err := svc.ApplyInspirationLibrary("owner", command)
	if err != nil {
		t.Fatal(err)
	}
	promptID := view.State.Overrides[id]
	favorite := libraryCommand(view.Revision, "favorite", id)
	enabled := true
	favorite.Favorited = &enabled
	view, err = svc.ApplyInspirationLibrary("owner", favorite)
	if err != nil {
		t.Fatal(err)
	}
	revision := view.Revision
	view, err = svc.ApplyInspirationLibrary("owner", favorite)
	if err != nil || view.Revision != revision || len(view.State.Favorites) != 1 {
		t.Fatalf("favorite replay changed state: %#v / %v", view, err)
	}
	view, err = svc.ApplyInspirationLibrary("owner", libraryCommand(view.Revision, "hide", id))
	if err != nil || view.State.Overrides[id] != promptID || len(view.State.Favorites) != 1 || len(view.Prompts) != 1 || view.Prompts[0].Prompt != "Private body" {
		t.Fatalf("hide lost content: %#v / %v", view, err)
	}
	view, err = svc.ApplyInspirationLibrary("owner", libraryCommand(view.Revision, "restore", id))
	if err != nil || len(view.State.Hidden) != 0 || view.State.Overrides[id] != promptID {
		t.Fatalf("restore lost edit: %#v / %v", view, err)
	}
	if err := svc.DeleteUserPrompt("owner", promptID); err != nil {
		t.Fatal(err)
	}
	view, err = svc.InspirationLibrary("owner")
	if err != nil || len(view.Prompts) != 0 || len(view.State.Overrides) != 0 || len(view.State.Favorites) != 1 {
		t.Fatalf("explicit personal deletion cleanup: %#v / %v", view, err)
	}
}

func TestInspirationImportRollsBackAndRetainsStablePersonalIdentity(t *testing.T) {
	svc, db := newUserPromptTestServiceWithDB(t)
	command := libraryCommand(0, "import", "")
	entry := InspirationImportEntry{ID: "custom:from-file", InspirationDraft: InspirationDraft{Title: "Import", Mode: model.InspirationModeText, Prompt: "Preserve imported text"}}
	command.Import = &InspirationImport{Version: 1, Custom: []InspirationImportEntry{entry, {ID: "custom:invalid", InspirationDraft: InspirationDraft{Title: "Invalid", Mode: model.InspirationModeText}}}}
	if _, err := svc.ApplyInspirationLibrary("owner", command); err == nil {
		t.Fatal("invalid import succeeded")
	}
	var count int64
	if err := db.Model(&model.UserPrompt{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("partial import committed: %d / %v", count, err)
	}
	command.Import.Custom = command.Import.Custom[:1]
	command.Import.Favorites = []string{entry.ID}
	view, err := svc.ApplyInspirationLibrary("owner", command)
	if err != nil {
		t.Fatal(err)
	}
	promptID := view.State.ImportedCustom[entry.ID]
	if len(view.State.Favorites) != 1 || view.State.Favorites[0] != customInspirationPrefix+promptID {
		t.Fatalf("import did not canonicalize favorites: %#v", view.State)
	}
	command.ExpectedRevision = &view.Revision
	view, err = svc.ApplyInspirationLibrary("owner", command)
	if err != nil || len(view.Prompts) != 1 || view.Prompts[0].ID != promptID {
		t.Fatalf("repeat import duplicated or changed identity: %#v / %v", view, err)
	}
	remove := libraryCommand(view.Revision, "favorite", customInspirationPrefix+promptID)
	disabled := false
	remove.Favorited = &disabled
	view, err = svc.ApplyInspirationLibrary("owner", remove)
	if err != nil || len(view.State.Favorites) != 0 {
		t.Fatalf("canonical unfavorite failed: %#v / %v", view, err)
	}
}

func TestInspirationLibraryKeepsQuotaOwnershipAndClosedContracts(t *testing.T) {
	svc, db := newUserPromptTestServiceWithDB(t)
	rows := make([]model.UserPrompt, maxUserPrompts)
	for index := range rows {
		rows[index] = model.UserPrompt{ID: "existing-" + string(rune(0x1000+index)), UserID: "owner", Title: "Existing", Mode: model.InspirationModeText, Prompt: "Body", TagsJSON: "[]"}
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	command := libraryCommand(0, "save", "custom:new")
	command.Draft = &InspirationDraft{Title: "Overflow", Mode: model.InspirationModeText, Prompt: "Body"}
	if _, err := svc.ApplyInspirationLibrary("owner", command); appErrorStatus(err) != 400 {
		t.Fatalf("500 prompt limit bypassed: %v", err)
	}
	command.ExpectedUserID = "other"
	if _, err := svc.ApplyInspirationLibrary("owner", command); appErrorStatus(err) != 401 {
		t.Fatalf("account mismatch accepted: %v", err)
	}
	command = libraryCommand(0, "favorite", "seedance:unadopted-id")
	enabled := true
	command.Favorited = &enabled
	if _, err := svc.ApplyInspirationLibrary("owner", command); appErrorStatus(err) != 400 {
		t.Fatalf("arbitrary catalog key accepted: %v", err)
	}
	if len(inspirationCatalogIDs) != 1341 {
		t.Fatalf("catalog ID count = %d", len(inspirationCatalogIDs))
	}
	for _, value := range []string{"/inspiration-thumbs/../secret.png", "//example.com/image.png", "/api/admin/private.png", "/short-drama-styles/%2e%2e/secret.png"} {
		if validBuiltinInspirationCover(value) {
			t.Fatalf("invalid local cover accepted: %s", value)
		}
	}
}
