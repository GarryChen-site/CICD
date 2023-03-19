package model

import "time"

type User struct {
	ID int64 `json:"id"`
	WorkNumber string `json:"work_number"`
	RealName string `json:"real_name"`
	Username string `json:"username"`
	OrgID int64 `json:"org_id"`
	Org Organization `json:"org"`
	Roles []Role `json:"rules"`
	Email string `json:"email"`
	Source string `json:"source"`
	LDAPInsertTime time.Time `json:"ldap_insert_time"`
	LDAPUpdateTime time.Time `json:"ldap_update_time"`
	Extensions map[string]string `json:"extensions"`
	LastVisitAt time.Time `json:"last_visit_at"`
	Base

}