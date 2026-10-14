package platform

import "testing"

func TestResourceQuotaPatchInheritanceAndValidation(t *testing.T) {
	defaults := DefaultRuntimePolicy().Resource
	upload := int64(10)
	next, effective, err := PatchResourceQuota(defaults, ResourceQuotaOverrides{"storedFileGB": 5}, ResourceQuotaPatch{"resourceUploadMB": &upload})
	if err != nil || effective.ResourceUploadMB != 10 || effective.StoredFileGB != 5 || effective.TaskCount != defaults.TaskCount {
		t.Fatalf("merge: %#v %v", effective, err)
	}
	defaults.ResourceUploadMB = 25
	_, inherited, err := PatchResourceQuota(defaults, next, ResourceQuotaPatch{"resourceUploadMB": nil})
	if err != nil || inherited.ResourceUploadMB != 25 || inherited.StoredFileGB != 5 {
		t.Fatalf("restore: %#v %v", inherited, err)
	}
	for _, key := range []string{"unknown", "workerConcurrency"} {
		if _, _, err := PatchResourceQuota(defaults, next, ResourceQuotaPatch{key: nil}); err == nil {
			t.Fatal("accepted unknown null field", key)
		}
	}
	zero := int64(0)
	if _, _, err := PatchResourceQuota(defaults, next, ResourceQuotaPatch{"storedFileGB": &zero}); err == nil {
		t.Fatal("accepted zero capacity")
	}
	if _, value, err := PatchResourceQuota(defaults, next, ResourceQuotaPatch{"recycleBinRetentionDays": &zero}); err != nil || value.RecycleBinRetentionDays != 0 {
		t.Fatal("zero retention", err)
	}
}
