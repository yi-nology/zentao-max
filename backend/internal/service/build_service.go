package service

import (
	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"

	"github.com/yi-nology/zentao-max/backend/internal/vo"
	myzentao "github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
)

type BuildService struct {
	client *myzentao.Client
}

func NewBuildService(client *myzentao.Client) *BuildService {
	return &BuildService{client: client}
}

func (s *BuildService) GetBuildsByProject(projectID int) ([]vo.BuildVO, error) {
	builds, err := s.client.GetBuildsByProject(projectID, 1, 100)
	if err != nil {
		return nil, err
	}
	return s.convertToVO(builds), nil
}

func (s *BuildService) GetBuildsByExecution(executionID int) ([]vo.BuildVO, error) {
	builds, err := s.client.GetBuildsByExecution(executionID, 1, 100)
	if err != nil {
		return nil, err
	}
	return s.convertToVO(builds), nil
}

func (s *BuildService) convertToVO(builds []zentaosdk.Build) []vo.BuildVO {
	if len(builds) == 0 {
		return []vo.BuildVO{}
	}

	result := make([]vo.BuildVO, 0, len(builds))
	for _, build := range builds {
		result = append(result, vo.BuildVO{
			ID:      build.ID,
			Product: build.Product,
			Project: build.Project,
			Name:    build.Name,
			Date:    build.Date,
		})
	}
	return result
}
