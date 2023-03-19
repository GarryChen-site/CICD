package model

type AppQuota struct {
	ID int64 `json:"id"`
	AppID string `json:"app_id"`
	AppName string `json:"app_name"`
	EnvID int64 `json:"env_id"`
	OrgID int64 `json:"org_id"`
	OrgName string `json:"org_name"`
	EnvName string `json:"env_name"`
	SpecTypeID int64 `json:"spec_type_id"`
	SpecTypeName string `json:"spec_type_name"`
	Number int64 `json:"number"`
	Base
}