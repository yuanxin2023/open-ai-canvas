package platform

import (
	"encoding/json"
	"fmt"
	"infinite-canvas/backend/internal/kernel"
)

type ResourceQuotaOverrides map[string]int64
type ResourceQuotaPatch map[string]*int64

// The map contains only explicit overrides; null is a patch command, never a stored value.
func MergeResourceQuota(defaults RuntimeResourcePolicy, overrides ResourceQuotaOverrides) (RuntimeResourcePolicy, error) {
	encoded, err := json.Marshal(defaults)
	if err != nil {
		return RuntimeResourcePolicy{}, err
	}
	var fields map[string]int64
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return RuntimeResourcePolicy{}, err
	}
	for key, value := range overrides {
		if _, ok := fields[key]; !ok {
			return RuntimeResourcePolicy{}, kernel.BadAuthRequest(fmt.Sprintf("未知资源配额项目：%s", key))
		}
		fields[key] = value
	}
	encoded, err = json.Marshal(fields)
	if err != nil {
		return RuntimeResourcePolicy{}, err
	}
	var result RuntimeResourcePolicy
	if err := json.Unmarshal(encoded, &result); err != nil {
		return RuntimeResourcePolicy{}, err
	}
	policy := DefaultRuntimePolicy()
	policy.Resource = result
	if err := validateRuntimePolicy(policy); err != nil {
		return RuntimeResourcePolicy{}, err
	}
	return result, nil
}

func PatchResourceQuota(defaults RuntimeResourcePolicy, current ResourceQuotaOverrides, patch ResourceQuotaPatch) (ResourceQuotaOverrides, RuntimeResourcePolicy, error) {
	next := make(ResourceQuotaOverrides, len(current)+len(patch))
	for key, value := range current {
		next[key] = value
	}
	encoded, err := json.Marshal(defaults)
	if err != nil {
		return nil, RuntimeResourcePolicy{}, err
	}
	var fields map[string]int64
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, RuntimeResourcePolicy{}, err
	}
	// Validate the key even when null requests inheritance.
	for key, value := range patch {
		if _, ok := fields[key]; !ok {
			return nil, RuntimeResourcePolicy{}, kernel.BadAuthRequest(fmt.Sprintf("未知资源配额项目：%s", key))
		}
		if value == nil {
			delete(next, key)
		} else {
			next[key] = *value
		}
	}
	effective, err := MergeResourceQuota(defaults, next)
	return next, effective, err
}
