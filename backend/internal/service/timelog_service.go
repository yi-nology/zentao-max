package service

import (
	"github.com/yi-nology/zentao-max/backend/internal/dto"
	myzentao "github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
)

// TimelogService 工时统计业务逻辑服务
// 负责处理工时统计相关的业务逻辑
type TimelogService struct {
	client *myzentao.Client
}

// NewTimelogService 创建工时统计服务
func NewTimelogService(client *myzentao.Client) *TimelogService {
	return &TimelogService{client: client}
}

// GetTimelogAnalysis 获取工时统计分析
// 业务逻辑：调用禅道客户端获取工时数据
func (s *TimelogService) GetTimelogAnalysis(query *dto.TimelogQueryDTO) (map[string]interface{}, error) {
	analysis, err := s.client.GetTimelogAnalysis(
		query.ProductID,
		query.ProjectID,
		query.ExecutionID,
		query.AssignedTo,
		query.DateFrom,
		query.DateTo,
	)
	if err != nil {
		return nil, err
	}

	return analysis, nil
}

// GetTimelogDashboard 获取工时统计看板数据
// 业务逻辑：提取看板相关的统计数据
func (s *TimelogService) GetTimelogDashboard(query *dto.TimelogQueryDTO) (map[string]interface{}, error) {
	analysis, err := s.GetTimelogAnalysis(query)
	if err != nil {
		return nil, err
	}

	// 提取看板数据
	dashboardData := map[string]interface{}{
		"totalHours":  analysis["totalHours"],
		"effortCount": analysis["effortCount"],
		"taskCount":   analysis["taskCount"],
		"byProject":   analysis["byProject"],
		"byType":      analysis["byType"],
		"byDate":      analysis["byDate"],
	}

	return dashboardData, nil
}

// GetTimelogEfforts 获取工时流水明细
// 业务逻辑：提取工时明细数据
func (s *TimelogService) GetTimelogEfforts(query *dto.TimelogQueryDTO) (interface{}, error) {
	analysis, err := s.GetTimelogAnalysis(query)
	if err != nil {
		return nil, err
	}

	// 提取明细数据
	effortsData := analysis["efforts"]

	return effortsData, nil
}
