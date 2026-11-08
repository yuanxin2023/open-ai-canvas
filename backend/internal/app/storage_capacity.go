// 账号文件容量统一使用全局默认与用户逐项覆盖。
package app

type AccountFileStorageUsage struct {
	UsedBytes  int64 `json:"usedBytes"`
	TotalBytes int64 `json:"totalBytes"`
	OverLimit  bool  `json:"overLimit"`
}

func (s *Service) storedFileLimit(userID string, policy RuntimePolicySetting) (int64, error) {
	resourcePolicy, err := userResourcePolicyWithRepository(s.repo, policy.Resource, userID)
	if err != nil {
		return 0, err
	}
	return gigabytes(resourcePolicy.StoredFileGB), nil
}

func (s *Service) AccountFileStorageUsage(userID string) (*AccountFileStorageUsage, error) {
	policy, err := s.UserRuntimePolicy(userID)
	if err != nil {
		return nil, err
	}
	used, err := s.repo.UserStoredFileBytes(userID)
	if err != nil {
		return nil, err
	}
	total := gigabytes(policy.Resource.StoredFileGB)
	return &AccountFileStorageUsage{UsedBytes: used, TotalBytes: total, OverLimit: used > total}, nil
}
