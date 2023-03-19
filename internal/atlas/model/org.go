package model

type Organization struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	OrgCode string `json:"org_code"`
	ParentOrgID int64 `json:"parent_org_id"`
	UserWorkerNumber string `json:"user_worker_number"`
	Description string `json:"description"`
}