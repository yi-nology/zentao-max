package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"github.com/yi-nology/zentao-max/backend/internal/event"
	"github.com/yi-nology/zentao-max/backend/internal/initialization"
	"github.com/yi-nology/zentao-max/backend/pkg/logger"
	"github.com/yi-nology/zentao-max/backend/internal/models"

	)

type SchedulerService struct {
	store    *initialization.ConfigStore
	cron     *cron.Cron
	report   *ReportService
	webhook  *WebhookService
	entryMap map[string]cron.EntryID
	mu       sync.Mutex
}

func NewSchedulerService(
	store *initialization.ConfigStore,
	reportService *ReportService,
	webhookService *WebhookService,
) *SchedulerService {
	return &SchedulerService{
		store:    store,
		cron:     cron.New(cron.WithSeconds()),
		report:   reportService,
		webhook:  webhookService,
		entryMap: make(map[string]cron.EntryID),
	}
}

func (s *SchedulerService) Start() error {
	tasks, err := s.store.ListTasks()
	if err != nil {
		return fmt.Errorf("加载定时任务失败: %w", err)
	}

	enabledCount := 0
	for i := range tasks {
		if tasks[i].Enabled {
			enabledCount++
			if err := s.registerTask(&tasks[i]); err != nil {
				logger.Error("注册定时任务失败",
					"taskID", tasks[i].ID,
					"taskName", tasks[i].Name,
					"cronExpr", tasks[i].CronExpr,
					"error", err)
				s.saveRegisterErrorLog(&tasks[i], err)
			}
		}
	}
	s.cron.Start()
	logger.Info("定时任务调度器已启动",
		"totalTasks", len(tasks),
		"enabledTasks", enabledCount)
	return nil
}

func (s *SchedulerService) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	logger.Info("定时任务调度器已停止")
}

func (s *SchedulerService) registerTask(task *models.SchedulerTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if oldID, ok := s.entryMap[task.ID]; ok {
		s.cron.Remove(oldID)
		delete(s.entryMap, task.ID)
	}

	cronExpr := task.CronExpr
	if len(strings.Fields(cronExpr)) == 5 {
		cronExpr = "0 " + cronExpr
	}

	taskCopy := *task
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		s.executeTask(&taskCopy, false)
	})
	if err != nil {
		return fmt.Errorf("解析cron表达式失败 '%s': %w", task.CronExpr, err)
	}

	s.entryMap[task.ID] = entryID
	logger.Info("定时任务已注册",
		"taskID", task.ID,
		"taskName", task.Name,
		"cronExpr", task.CronExpr)
	return nil
}

func (s *SchedulerService) unregisterTask(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entryID, ok := s.entryMap[taskID]; ok {
		s.cron.Remove(entryID)
		delete(s.entryMap, taskID)
	}
}

func (s *SchedulerService) saveRegisterErrorLog(task *models.SchedulerTask, err error) {
	logEntry := &models.TaskExecutionLog{
		ID:             uuid.New().String(),
		TaskID:         task.ID,
		TaskName:       task.Name,
		StartedAt:      time.Now(),
		Status:         "failed",
		Error:          fmt.Sprintf("任务注册失败: %v", err),
		WebhookResults: []models.WebhookResult{},
	}
	now := time.Now()
	logEntry.FinishedAt = &now
	if saveErr := s.store.SaveLog(logEntry); saveErr != nil {
		logger.Error("保存注册失败日志出错",
			"taskID", task.ID,
			"error", saveErr)
	}
}

func (s *SchedulerService) ListTasks() ([]models.SchedulerTask, error) {
	return s.store.ListTasks()
}

func (s *SchedulerService) GetTask(id string) (*models.SchedulerTask, error) {
	return s.store.GetTask(id)
}

func (s *SchedulerService) CreateTask(task *models.SchedulerTask) error {
	now := time.Now()
	task.ID = uuid.New().String()
	task.CreatedAt = now
	task.UpdatedAt = now
	if task.StatusFilter == "" {
		task.StatusFilter = "active"
	}
	if task.ReportType == "" {
		task.ReportType = "bug"
	}
	if task.ReportType == "bug-aging" && task.AgingDays <= 0 {
		task.AgingDays = 7
	}
	for i := range task.Webhooks {
		if task.Webhooks[i].ID == "" {
			task.Webhooks[i].ID = uuid.New().String()
		}
		if task.Webhooks[i].Name == "" {
			task.Webhooks[i].Name = task.Webhooks[i].Platform
		}
	}
	if err := s.store.CreateTask(task); err != nil {
		logger.Error("创建定时任务失败",
			"taskName", task.Name,
			"error", err)
		return err
	}
	logger.Info("定时任务已创建",
		"taskID", task.ID,
		"taskName", task.Name,
		"enabled", task.Enabled,
		"cronExpr", task.CronExpr)
	if task.Enabled {
		return s.registerTask(task)
	}
	return nil
}

func (s *SchedulerService) UpdateTask(task *models.SchedulerTask) error {
	// 保留已存在的CreatedAt，避免请求体未携带时被重置为零值
	if existing, err := s.store.GetTask(task.ID); err != nil {
		return fmt.Errorf("查询任务失败: %w", err)
	} else if existing != nil {
		task.CreatedAt = existing.CreatedAt
	}
	task.UpdatedAt = time.Now()
	if task.ReportType == "" {
		task.ReportType = "bug"
	}
	if task.ReportType == "bug-aging" && task.AgingDays <= 0 {
		task.AgingDays = 7
	}
	for i := range task.Webhooks {
		if task.Webhooks[i].ID == "" {
			task.Webhooks[i].ID = uuid.New().String()
		}
		if task.Webhooks[i].Name == "" {
			task.Webhooks[i].Name = task.Webhooks[i].Platform
		}
	}
	if err := s.store.UpdateTask(task); err != nil {
		logger.Error("更新定时任务失败",
			"taskID", task.ID,
			"taskName", task.Name,
			"error", err)
		return err
	}
	logger.Info("定时任务已更新",
		"taskID", task.ID,
		"taskName", task.Name,
		"enabled", task.Enabled,
		"cronExpr", task.CronExpr)
	s.unregisterTask(task.ID)
	if task.Enabled {
		return s.registerTask(task)
	}
	return nil
}

func (s *SchedulerService) DeleteTask(id string) error {
	s.unregisterTask(id)
	logger.Info("定时任务已删除", "taskID", id)
	return s.store.DeleteTask(id)
}

func (s *SchedulerService) ToggleTask(id string) (*models.SchedulerTask, error) {
	task, err := s.store.GetTask(id)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("任务不存在")
	}
	task.Enabled = !task.Enabled
	task.UpdatedAt = time.Now()
	if err := s.store.UpdateTask(task); err != nil {
		return nil, err
	}
	s.unregisterTask(id)
	if task.Enabled {
		if err := s.registerTask(task); err != nil {
			return nil, err
		}
	}
	logger.Info("定时任务状态已切换",
		"taskID", task.ID,
		"taskName", task.Name,
		"enabled", task.Enabled)
	return task, nil
}

func (s *SchedulerService) RunTaskNow(id string) (*models.TaskExecutionLog, error) {
	task, err := s.store.GetTask(id)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("任务不存在")
	}
	logger.Info("手动触发定时任务",
		"taskID", task.ID,
		"taskName", task.Name)
	return s.executeTask(task, true), nil
}

func (s *SchedulerService) executeTask(task *models.SchedulerTask, manual bool) *models.TaskExecutionLog {
	logEntry := &models.TaskExecutionLog{
		ID:             uuid.New().String(),
		TaskID:         task.ID,
		TaskName:       task.Name,
		StartedAt:      time.Now(),
		Status:         "running",
		WebhookResults: []models.WebhookResult{},
	}

	triggerType := "scheduled"
	if manual {
		triggerType = "manual"
	}

	logger.Info("开始执行定时任务",
		"taskID", task.ID,
		"taskName", task.Name,
		"triggerType", triggerType,
		"reportType", task.ReportType,
		"productID", task.ProductID,
		"projectID", task.ProjectID,
		"projectName", task.ProjectName,
		"statusFilter", task.StatusFilter)

	reportType := task.ReportType
	if reportType == "" {
		reportType = "bug"
	}

	var message string
	var reportErr error

	switch reportType {
	case "requirement":
		report, err := s.report.GenerateRequirementReport(task.ProductID, task.ProjectID, task.ProjectName, task.ProductName, task.Keyword, task.ExternalInfo, task.MessageHeader, task.PriorityAssignees)
		if err != nil {
			reportErr = err
		} else {
			message = report.Message
			logEntry.BugTotal = report.Total
		}
	case "task":
		report, err := s.report.GenerateTaskReport(task.ProductID, task.ProjectID, task.ProjectName, task.ProductName, task.Keyword, task.ExternalInfo, task.MessageHeader, task.PriorityAssignees)
		if err != nil {
			reportErr = err
		} else {
			message = report.Message
			logEntry.BugTotal = report.Total
			logEntry.HighSeverity = 0
			logEntry.AssigneeCount = len(report.Details)
		}
	case "bug-aging":
		agingDays := task.AgingDays
		if agingDays <= 0 {
			agingDays = 7
		}
		report, err := s.report.GenerateBugAgingReport(task.ProductID, task.ProjectID, task.ProjectName, task.StatusFilter, agingDays, task.Keyword, task.ExternalInfo, task.PriorityAssignees, task.MessageHeader)
		if err != nil {
			reportErr = err
		} else {
			message = report.Message
			logEntry.BugTotal = report.Total
			logEntry.AssigneeCount = len(report.Details)
		}
	default: // "bug"
		report, err := s.report.GenerateBugReport(task.ProductID, task.ProjectID, task.ProjectName, task.StatusFilter, task.Keyword, task.ExternalInfo, task.MessageHeader, task.PriorityAssignees)
		if err != nil {
			reportErr = err
		} else {
			message = report.Message
			logEntry.BugTotal = report.Total
			logEntry.HighSeverity = report.HighSeverity
			logEntry.AssigneeCount = len(report.Details)
		}
	}

	if reportErr != nil {
		logEntry.Status = "failed"
		logEntry.Error = reportErr.Error()
		now := time.Now()
		logEntry.FinishedAt = &now
		_ = s.store.SaveLog(logEntry)
		logger.Error("定时任务执行失败",
			"taskID", task.ID,
			"taskName", task.Name,
			"triggerType", triggerType,
			"error", reportErr)
		return logEntry
	}

	webhookResults := s.webhook.SendAllGeneric(task.Webhooks, message)
	logEntry.WebhookResults = webhookResults

	successCount := 0
	for _, r := range webhookResults {
		if r.Success {
			successCount++
		}
	}

	enabledCount := 0
	for _, wh := range task.Webhooks {
		if wh.Enabled {
			enabledCount++
		}
	}

	now := time.Now()
	logEntry.FinishedAt = &now

	if enabledCount == 0 {
		logEntry.Status = "success"
	} else if successCount == 0 {
		logEntry.Status = "failed"
	} else if successCount < enabledCount {
		logEntry.Status = "partial"
	} else {
		logEntry.Status = "success"
	}

	_ = s.store.SaveLog(logEntry)
	logger.Info("定时任务执行完成",
		"taskID", task.ID,
		"taskName", task.Name,
		"triggerType", triggerType,
		"reportType", reportType,
		"status", logEntry.Status,
		"bugTotal", logEntry.BugTotal,
		"webhookSuccess", successCount,
		"webhookEnabled", enabledCount)

	// 发布事件：桌面通知等订阅者会消费（仅在 Wails 模式生效）
	topic := event.TaskCompleted
	if logEntry.Status == "failed" {
		topic = event.TaskFailed
	}
	event.GetGlobalBus().Publish(topic, map[string]interface{}{
		"taskID":      task.ID,
		"taskName":    task.Name,
		"status":      logEntry.Status,
		"bugTotal":    logEntry.BugTotal,
		"triggerType": triggerType,
	})

	return logEntry
}

func (s *SchedulerService) ListLogs(taskID string, limit int) ([]models.TaskExecutionLog, error) {
	return s.store.ListLogs(taskID, limit)
}
