package remote

import "time"

type App struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Description string `json:"description"`
	ServiceType string `json:"service_type"`
	AppType string `json:"app_type"`
	Owner string `json:"owner"`
	Developer string `json:"developer"`
	DeveloperNames string `json:"developer_names"`
	Tester string `json:"tester"`
	TesterNames string `json:"tester_names"`
	Department string `json:"department"`
	DepartmentCode string `json:"department_code"`
	CmdbAppID string `json:"cmdb_app_id"`
	EnableHa bool `json:"enable_ha"`
	EnvUrlMap map[string]string `json:"env_url_map"`
	InserTime time.Time `json:"insert_time"`
	UpdateTime time.Time `json:"update_time"`
}
