package remote

import "time"

type ENV struct {
	ID   int64  `json:"id"`
	CmdbEnvID int64 `json:"cmdb_env_id"`
	Name string `json:"name"`
	Nginx string `json:"nginx"`
	DNS string `json:"dns"`
	DockerYard string `json:"docker_yard"`
	IsInUse bool `json:"is_in_use"`
	EnableHa bool `json:"enable_ha"`
	Description string `json:"description"`
	InsertTime time.Time `json:"insert_time"`

}