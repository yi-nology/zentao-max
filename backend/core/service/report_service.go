package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	zentaosdk "github.com/yi-nology/zentao-max/backend/core/zentao/sdk"
	"go.uber.org/zap"

	"github.com/yi-nology/zentao-max/backend/core/logger"
	"github.com/yi-nology/zentao-max/backend/core/models"
	myzentao "github.com/yi-nology/zentao-max/backend/core/zentao"
)

type ReportService struct {
	client *myzentao.Client
}

func NewReportService(client *myzentao.Client) *ReportService {
	return &ReportService{client: client}
}

func severityInt(v interface{}) int {
	switch s := v.(type) {
	case float64:
		return int(s)
	case int:
		return s
	case string:
		var n int
		fmt.Sscanf(s, "%d", &n)
		return n
	default:
		return 0
	}
}

func (s *ReportService) GenerateBugReport(productID int, projectID int, projectName string, statusFilter string, keyword string, externalInfo string, messageHeader string, priorityAssignees []string) (*models.BugReport, error) {
	bugs, err := s.client.GetAllBugsByProjectWithProduct(productID, projectID)
	if err != nil {
		return nil, fmt.Errorf("获取Bug列表失败: %w", err)
	}

	logger.Info("获取Bug列表",
		zap.Int("productID", productID),
		zap.Int("projectID", projectID),
		zap.Int("bugCount", len(bugs)))

	if statusFilter == "" {
		statusFilter = "active"
	}

	filtered := make([]zentaosdk.Bug, 0)
	statusBreakdown := map[string]int{"active": 0, "resolved": 0, "closed": 0}
	for _, b := range bugs {
		statusBreakdown[b.Status]++
		if statusFilter == "all" || b.Status == statusFilter {
			filtered = append(filtered, b)
		}
	}

	logger.Info("Bug过滤结果",
		zap.String("statusFilter", statusFilter),
		zap.Int("filteredCount", len(filtered)),
		zap.Any("statusBreakdown", statusBreakdown))

	assigneeMap := make(map[string]*models.AssigneeBugStats)
	for _, b := range filtered {
		name := b.AssignedTo.Realname
		if name == "" {
			name = b.AssignedTo.Account
		}
		if name == "" {
			name = "未指派"
		}
		stat, ok := assigneeMap[name]
		if !ok {
			stat = &models.AssigneeBugStats{
				Assignee: name,
				Account:  b.AssignedTo.Account,
			}
			assigneeMap[name] = stat
		}
		stat.Total++
		sev := severityInt(b.Severity)
		switch sev {
		case 1:
			stat.Fatal++
			stat.HighSeverity++
		case 2:
			stat.Serious++
			stat.HighSeverity++
		case 3:
			stat.Moderate++
			stat.HighSeverity++
		case 4:
			stat.Minor++
		}
	}

	details := make([]models.AssigneeBugStats, 0, len(assigneeMap))
	for _, stat := range assigneeMap {
		details = append(details, *stat)
	}

	// 构建优先人员集合
	prioritySet := make(map[string]bool)
	for _, name := range priorityAssignees {
		prioritySet[name] = true
	}

	// 排序：优先人员排前面，然后按 Bug 数量降序
	sort.Slice(details, func(i, j int) bool {
		iPriority := prioritySet[details[i].Assignee] || prioritySet[details[i].Account]
		jPriority := prioritySet[details[j].Assignee] || prioritySet[details[j].Account]
		if iPriority != jPriority {
			return iPriority
		}
		return details[i].Total > details[j].Total
	})

	totalHigh := 0
	for _, d := range details {
		totalHigh += d.HighSeverity
	}

	now := time.Now()
	title := fmt.Sprintf("Bug 分布报告 - %s", projectName)
	message := buildMessage(title, now, len(filtered), totalHigh, details, statusBreakdown, keyword, externalInfo, messageHeader)

	return &models.BugReport{
		Title:           title,
		Timestamp:       now.Format(time.RFC3339),
		ProjectName:     projectName,
		Total:           len(filtered),
		HighSeverity:    totalHigh,
		StatusBreakdown: statusBreakdown,
		Details:         details,
		Message:         message,
	}, nil
}

func buildMessage(title string, t time.Time, total, highSeverity int, details []models.AssigneeBugStats, statusBreakdown map[string]int, keyword string, externalInfo string, messageHeader string) string {
	var sb strings.Builder
	kw := ""
	if keyword != "" {
		kw = fmt.Sprintf("【%s】", keyword)
	}
	sb.WriteString(fmt.Sprintf("%s🔴 %s\n", kw, title))
	sb.WriteString(fmt.Sprintf("📅 %s\n", t.Format("2006-01-02 15:04:05")))
	if messageHeader != "" {
		sb.WriteString(fmt.Sprintf("📌 %s\n", messageHeader))
	}
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(fmt.Sprintf("📊 剩余 Bug：%d个（高级别 %d个）\n\n", total, highSeverity))

	for _, d := range details {
		highStr := ""
		if d.HighSeverity > 0 {
			highStr = fmt.Sprintf(" 其中%d个高级别", d.HighSeverity)
		}
		sb.WriteString(fmt.Sprintf("👤 %s  %d个%s\n", d.Assignee, d.Total, highStr))
		sb.WriteString(fmt.Sprintf("   └ 致命:%d 严重:%d 一般:%d 轻微:%d\n", d.Fatal, d.Serious, d.Moderate, d.Minor))
	}

	sb.WriteString("\n━━━━━━━━━━━━━━━━━━━━\n")
	if externalInfo != "" {
		sb.WriteString(fmt.Sprintf("📌 外部信息：\n%s\n━━━━━━━━━━━━━━━━━━━━\n", externalInfo))
	}
	sb.WriteString(fmt.Sprintf("⚠️ 高级别 Bug 共 %d个，需重点关注！\n", highSeverity))
	sb.WriteString(fmt.Sprintf("📈 Bug 状态分布：活跃 %d | 已解决 %d | 已关闭 %d",
		statusBreakdown["active"], statusBreakdown["resolved"], statusBreakdown["closed"]))
	return sb.String()
}

func (s *ReportService) GenerateRequirementReport(productID int, projectID int, projectName string, productName string, keyword string, externalInfo string, messageHeader string, priorityAssignees []string) (*models.RequirementReport, error) {
	var stories []zentaosdk.Story
	var err error

	if projectID > 0 {
		stories, err = s.client.GetAllStoriesByProject(projectID)
	} else if productID > 0 {
		stories, err = s.client.GetAllStories(productID)
	} else {
		return nil, fmt.Errorf("请提供产品ID或项目ID")
	}
	if err != nil {
		return nil, fmt.Errorf("获取需求列表失败: %w", err)
	}

	statusBreakdown := map[string]int{}
	for _, st := range stories {
		statusBreakdown[st.Status]++
	}

	assigneeMap := make(map[string]*models.AssigneeStoryStats)
	for _, st := range stories {
		name := ""
		account := ""
		if ref, ok := st.AssignedTo.(zentaosdk.UserRef); ok {
			name = ref.Realname
			account = ref.Account
		} else if ref, ok := st.AssignedTo.(map[string]interface{}); ok {
			if v, ok := ref["realname"].(string); ok {
				name = v
			}
			if v, ok := ref["account"].(string); ok {
				account = v
			}
		}
		if name == "" {
			name = "未指派"
		}
		stat, ok := assigneeMap[name]
		if !ok {
			stat = &models.AssigneeStoryStats{
				Assignee: name,
				Account:  account,
			}
			assigneeMap[name] = stat
		}
		stat.Total++
		switch st.Status {
		case "active":
			stat.Active++
		case "changed":
			stat.Changed++
		case "closed":
			stat.Closed++
		case "resolved":
			stat.Resolved++
		case "accepted":
			stat.Accepted++
		}
	}

	details := make([]models.AssigneeStoryStats, 0, len(assigneeMap))
	for _, stat := range assigneeMap {
		details = append(details, *stat)
	}

	// 构建优先人员集合
	prioritySet := make(map[string]bool)
	for _, name := range priorityAssignees {
		prioritySet[name] = true
	}

	// 排序：优先人员排前面，然后按需求数量降序
	sort.Slice(details, func(i, j int) bool {
		iPriority := prioritySet[details[i].Assignee] || prioritySet[details[i].Account]
		jPriority := prioritySet[details[j].Assignee] || prioritySet[details[j].Account]
		if iPriority != jPriority {
			return iPriority
		}
		return details[i].Total > details[j].Total
	})

	now := time.Now()
	title := fmt.Sprintf("需求进度报告 - %s", projectName)
	message := buildRequirementMessage(title, now, len(stories), details, statusBreakdown, keyword, externalInfo, messageHeader)

	return &models.RequirementReport{
		Title:           title,
		Timestamp:       now.Format(time.RFC3339),
		ProjectName:     projectName,
		ProductName:     productName,
		Total:           len(stories),
		StatusBreakdown: statusBreakdown,
		Details:         details,
		Message:         message,
	}, nil
}

func buildRequirementMessage(title string, t time.Time, total int, details []models.AssigneeStoryStats, statusBreakdown map[string]int, keyword string, externalInfo string, messageHeader string) string {
	var sb strings.Builder
	kw := ""
	if keyword != "" {
		kw = fmt.Sprintf("【%s】", keyword)
	}
	sb.WriteString(fmt.Sprintf("%s📋 %s\n", kw, title))
	sb.WriteString(fmt.Sprintf("📅 %s\n", t.Format("2006-01-02 15:04:05")))
	if messageHeader != "" {
		sb.WriteString(fmt.Sprintf("📌 %s\n", messageHeader))
	}
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(fmt.Sprintf("📊 需求总数：%d个\n\n", total))

	for _, d := range details {
		sb.WriteString(fmt.Sprintf("👤 %s  共%d个需求\n", d.Assignee, d.Total))
		sb.WriteString(fmt.Sprintf("   └ 活跃:%d 变更:%d 已解决:%d 已关闭:%d 已验收:%d\n",
			d.Active, d.Changed, d.Resolved, d.Closed, d.Accepted))
	}

	sb.WriteString("\n━━━━━━━━━━━━━━━━━━━━\n")
	if externalInfo != "" {
		sb.WriteString(fmt.Sprintf("📌 外部信息：\n%s\n━━━━━━━━━━━━━━━━━━━━\n", externalInfo))
	}
	sb.WriteString(fmt.Sprintf("📈 需求状态分布："))
	for status, count := range statusBreakdown {
		sb.WriteString(fmt.Sprintf("%s %d | ", status, count))
	}
	return sb.String()
}

func (s *ReportService) GenerateTaskReport(productID int, projectID int, projectName string, productName string, keyword string, externalInfo string, messageHeader string, priorityAssignees []string) (*models.TaskProgressReport, error) {
	var tasks []zentaosdk.Task
	var err error

	if projectID > 0 {
		tasks, err = s.client.GetAllTasksByProject(projectID)
	} else if productID > 0 {
		tasks, err = s.client.GetAllTasksByProduct(productID)
	} else {
		return nil, fmt.Errorf("请提供产品ID或项目ID")
	}
	if err != nil {
		return nil, fmt.Errorf("获取任务列表失败: %w", err)
	}

	statusBreakdown := map[string]int{}
	var totalEstimate, totalConsumed float64

	for _, t := range tasks {
		statusBreakdown[t.Status]++
		totalEstimate += toFloat64(t.Estimate)
		totalConsumed += toFloat64(t.Consumed)
	}

	var overallProgress float64
	if totalEstimate > 0 {
		overallProgress = (totalConsumed / totalEstimate) * 100
		if overallProgress > 100 {
			overallProgress = 100
		}
	}

	assigneeMap := make(map[string]*models.TaskProgressStats)
	for _, t := range tasks {
		name := ""
		account := ""
		if ref, ok := t.AssignedTo.(zentaosdk.UserRef); ok {
			name = ref.Realname
			account = ref.Account
		} else if ref, ok := t.AssignedTo.(map[string]interface{}); ok {
			if v, ok := ref["realname"].(string); ok {
				name = v
			}
			if v, ok := ref["account"].(string); ok {
				account = v
			}
		}
		if name == "" {
			name = "未指派"
		}
		stat, ok := assigneeMap[name]
		if !ok {
			stat = &models.TaskProgressStats{
				Assignee: name,
				Account:  account,
			}
			assigneeMap[name] = stat
		}
		stat.Total++
		stat.Estimate += toFloat64(t.Estimate)
		stat.Consumed += toFloat64(t.Consumed)
		switch t.Status {
		case "wait":
			stat.Wait++
		case "doing":
			stat.Doing++
		case "done":
			stat.Done++
		case "pause":
			stat.Paused++
		case "cancel":
			stat.Cancelled++
		}
	}

	for _, stat := range assigneeMap {
		if stat.Estimate > 0 {
			stat.Progress = (stat.Consumed / stat.Estimate) * 100
			if stat.Progress > 100 {
				stat.Progress = 100
			}
		}
	}

	details := make([]models.TaskProgressStats, 0, len(assigneeMap))
	for _, stat := range assigneeMap {
		details = append(details, *stat)
	}

	// 构建优先人员集合
	prioritySet := make(map[string]bool)
	for _, name := range priorityAssignees {
		prioritySet[name] = true
	}

	// 排序：优先人员排前面，然后按任务数量降序
	sort.Slice(details, func(i, j int) bool {
		iPriority := prioritySet[details[i].Assignee] || prioritySet[details[i].Account]
		jPriority := prioritySet[details[j].Assignee] || prioritySet[details[j].Account]
		if iPriority != jPriority {
			return iPriority
		}
		return details[i].Total > details[j].Total
	})

	now := time.Now()
	title := fmt.Sprintf("任务进度报告 - %s", projectName)
	message := buildTaskMessage(title, now, len(tasks), totalEstimate, totalConsumed, overallProgress, details, statusBreakdown, keyword, externalInfo, messageHeader)

	return &models.TaskProgressReport{
		Title:           title,
		Timestamp:       now.Format(time.RFC3339),
		ProjectName:     projectName,
		ProductName:     productName,
		Total:           len(tasks),
		StatusBreakdown: statusBreakdown,
		TotalEstimate:   totalEstimate,
		TotalConsumed:   totalConsumed,
		OverallProgress: overallProgress,
		Details:         details,
		Message:         message,
	}, nil
}

func buildTaskMessage(title string, t time.Time, total int, totalEstimate, totalConsumed, overallProgress float64, details []models.TaskProgressStats, statusBreakdown map[string]int, keyword string, externalInfo string, messageHeader string) string {
	var sb strings.Builder
	kw := ""
	if keyword != "" {
		kw = fmt.Sprintf("【%s】", keyword)
	}
	sb.WriteString(fmt.Sprintf("%s✅ %s\n", kw, title))
	sb.WriteString(fmt.Sprintf("📅 %s\n", t.Format("2006-01-02 15:04:05")))
	if messageHeader != "" {
		sb.WriteString(fmt.Sprintf("📌 %s\n", messageHeader))
	}
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(fmt.Sprintf("📊 任务总数：%d个 | 整体进度：%.0f%%\n", total, overallProgress))
	sb.WriteString(fmt.Sprintf("⏱ 预估工时：%.1fh | 已消耗：%.1fh\n\n", totalEstimate, totalConsumed))

	for _, d := range details {
		progressStr := fmt.Sprintf("%.0f%%", d.Progress)
		sb.WriteString(fmt.Sprintf("👤 %s  共%d个任务  进度%s\n", d.Assignee, d.Total, progressStr))
		sb.WriteString(fmt.Sprintf("   └ 待开始:%d 进行中:%d 已完成:%d 已暂停:%d\n",
			d.Wait, d.Doing, d.Done, d.Paused))
	}

	sb.WriteString("\n━━━━━━━━━━━━━━━━━━━━\n")
	if externalInfo != "" {
		sb.WriteString(fmt.Sprintf("📌 外部信息：\n%s\n━━━━━━━━━━━━━━━━━━━━\n", externalInfo))
	}
	sb.WriteString(fmt.Sprintf("📈 任务状态分布：待开始 %d | 进行中 %d | 已完成 %d | 已暂停 %d",
		statusBreakdown["wait"], statusBreakdown["doing"], statusBreakdown["done"], statusBreakdown["pause"]))
	return sb.String()
}

func (s *ReportService) GenerateBugAgingReport(productID int, projectID int, projectName string, statusFilter string, agingDays int, keyword string, externalInfo string, priorityAssignees []string, messageHeader string) (*models.BugAgingReport, error) {
	bugs, err := s.client.GetAllBugsByProjectWithProduct(productID, projectID)
	if err != nil {
		return nil, fmt.Errorf("获取Bug列表失败: %w", err)
	}

	if agingDays <= 0 {
		agingDays = 7
	}

	logger.Info("获取Bug列表(超时分析)",
		zap.Int("productID", productID),
		zap.Int("projectID", projectID),
		zap.Int("bugCount", len(bugs)),
		zap.Int("agingDays", agingDays))

	if statusFilter == "" {
		statusFilter = "active"
	}

	now := time.Now()
	assigneeMap := make(map[string]*models.AssigneeBugAgingStats)

	for _, b := range bugs {
		if b.Status != "active" && b.Status != "resolved" {
			continue
		}
		if statusFilter != "all" && b.Status != statusFilter && statusFilter != "active-resolved" {
			continue
		}

		openedStr := parseDateField(b.OpenedDate)
		if openedStr == "" {
			continue
		}
		openedTime, err := time.ParseInLocation("2006-01-02T15:04:05Z", openedStr, time.UTC)
		if err != nil {
			openedTime, err = time.ParseInLocation("2006-01-02 15:04:05", openedStr, time.Local)
			if err != nil {
				openedTime, err = time.ParseInLocation("2006-01-02", openedStr, time.Local)
				if err != nil {
					continue
				}
			}
		}
		daysOpen := int(now.Sub(openedTime).Hours() / 24)
		if daysOpen < agingDays {
			continue
		}

		name := b.AssignedTo.Realname
		if name == "" {
			name = b.AssignedTo.Account
		}
		if name == "" {
			name = "未指派"
		}

		stat, ok := assigneeMap[name]
		if !ok {
			stat = &models.AssigneeBugAgingStats{
				Assignee: name,
				Account:  b.AssignedTo.Account,
				Bugs:     []models.BugAgingItem{},
			}
			assigneeMap[name] = stat
		}

		sev := severityInt(b.Severity)
		sevName := severityName(sev)

		stat.Total++
		stat.Bugs = append(stat.Bugs, models.BugAgingItem{
			ID:         b.ID,
			Title:      b.Title,
			Severity:   sevName,
			OpenedDate: openedStr,
			DaysOpen:   daysOpen,
		})
	}

	details := make([]models.AssigneeBugAgingStats, 0, len(assigneeMap))
	for _, stat := range assigneeMap {
		sort.Slice(stat.Bugs, func(i, j int) bool {
			return stat.Bugs[i].DaysOpen > stat.Bugs[j].DaysOpen
		})
		details = append(details, *stat)
	}

	// 构建优先人员集合
	prioritySet := make(map[string]bool)
	for _, name := range priorityAssignees {
		prioritySet[name] = true
	}

	// 排序：优先人员排前面，然后按 Bug 数量降序
	sort.Slice(details, func(i, j int) bool {
		iPriority := prioritySet[details[i].Assignee] || prioritySet[details[i].Account]
		jPriority := prioritySet[details[j].Assignee] || prioritySet[details[j].Account]
		if iPriority != jPriority {
			return iPriority
		}
		return details[i].Total > details[j].Total
	})

	total := 0
	for _, d := range details {
		total += d.Total
	}

	title := fmt.Sprintf("Bug 停留超时提醒 - %s", projectName)
	message := buildBugAgingMessage(title, now, total, agingDays, details, keyword, externalInfo, messageHeader)

	return &models.BugAgingReport{
		Title:       title,
		Timestamp:   now.Format(time.RFC3339),
		ProjectName: projectName,
		Total:       total,
		AgingDays:   agingDays,
		Details:     details,
		Message:     message,
	}, nil
}

func parseDateField(v interface{}) string {
	switch d := v.(type) {
	case string:
		return d
	case time.Time:
		return d.Format("2006-01-02 15:04:05")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func severityName(sev int) string {
	switch sev {
	case 1:
		return "致命"
	case 2:
		return "严重"
	case 3:
		return "一般"
	case 4:
		return "轻微"
	default:
		return "未知"
	}
}

func buildBugAgingMessage(title string, t time.Time, total, agingDays int, details []models.AssigneeBugAgingStats, keyword string, externalInfo string, messageHeader string) string {
	var sb strings.Builder
	kw := ""
	if keyword != "" {
		kw = fmt.Sprintf("【%s】", keyword)
	}
	sb.WriteString(fmt.Sprintf("%s⏰ %s\n", kw, title))
	sb.WriteString(fmt.Sprintf("📅 %s\n", t.Format("2006-01-02 15:04:05")))
	if messageHeader != "" {
		sb.WriteString(fmt.Sprintf("📌 %s\n", messageHeader))
	}
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(fmt.Sprintf("📊 超时 Bug：%d个（阈值：%d天）\n\n", total, agingDays))

	for _, d := range details {
		sb.WriteString(fmt.Sprintf("👤 %s  %d个超时Bug\n", d.Assignee, d.Total))
		for _, bug := range d.Bugs {
			titleStr := bug.Title
			if len([]rune(titleStr)) > 20 {
				titleStr = string([]rune(titleStr)[:20]) + "..."
			}
			sb.WriteString(fmt.Sprintf("   └ #%d [%s] %s 已停留 %d天\n", bug.ID, bug.Severity, titleStr, bug.DaysOpen))
		}
	}

	sb.WriteString("\n━━━━━━━━━━━━━━━━━━━━\n")
	if externalInfo != "" {
		sb.WriteString(fmt.Sprintf("📌 外部信息：\n%s\n━━━━━━━━━━━━━━━━━━━━\n", externalInfo))
	}
	sb.WriteString(fmt.Sprintf("⚠️ 以上 Bug 已超过 %d 天未解决，请尽快处理！", agingDays))
	return sb.String()
}
