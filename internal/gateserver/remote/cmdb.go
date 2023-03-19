package remote

import "cicd_go/internal/atlas/model"

type Cmdb interface {
	FetchAllApps() ([]*App, error)

	FetchAppsByUserName(userName string) ([]*App, error)

	FetchInstancesSpeces() ([]*InstanceSpec, error)

	FetchAppQuotasByAppAndEnv(appID string, env string) ([]model.AppQuota, error)

	FetchEnvironments() ([]*ENV, error)

	FetchOrganizations() ([]*model.Organization, error)

	FetchAppByAppID(appID string) (*App, error)

	SearchUsersByUserName(userName string) ([]*model.User, error)

	UpdateAppMember(appID string, developers string, testers string) (bool,error)

	FetchZonesByEnv(env string) ([]*model.Zone, error)

	FetchAllZones() ([]*model.Zone, error)

}