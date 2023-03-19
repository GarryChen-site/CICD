package model

type Zone struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	EnvID int64 `json:"env_id"`
	EnvName string `json:"env_name"`
	K8s string `json:"k8s"`
	K8sVersion string `json:"k8s_version"`
	Description string `json:"description"`
	Base
}