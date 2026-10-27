package model

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestWorkbenchSubmissionIdentityIsUniqueWithinOwner(t *testing.T) {
	parsed, err := schema.Parse(&WorkbenchSubmission{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.PrimaryFields) != 2 || parsed.PrimaryFields[0].DBName != "user_id" || parsed.PrimaryFields[1].DBName != "request_key" {
		t.Fatalf("primary identity = %+v", parsed.PrimaryFields)
	}
	task, err := schema.Parse(&Task{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	if field := task.LookUpField("WorkbenchSubmission"); field == nil || field.DBName != "" {
		t.Fatal("public receipt must not add a mutable task database column")
	}
}
