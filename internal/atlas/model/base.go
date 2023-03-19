package model

import "time"

type Base struct {
	InsertTime time.Time `json:"insert_time"`
	UpdateTime time.Time `json:"update_time"`
	InsertBy  string    `json:"insert_by"`
	UpdateBy  string    `json:"update_by"`
	IsActive bool      `json:"is_active"`
}