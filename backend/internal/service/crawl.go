package service

import (
	"context"
	"errors"
	"time"

	"github.com/oekit/concurrency/fanout"

	myzentao "github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"
	"github.com/yi-nology/zentao-max/backend/pkg/logger"
)

// crawlWorkers 跨执行爬取的并发度。上游实际并发由客户端读池/网关限流，
// 这里给软上限；fanout.Map 的排队项在 ctx 截止时按 context.Canceled 记入
// 失败清单并立即返回，不存在手搓 sem/wg 的「超时后排干整条队列」类 bug。
const crawlWorkers = 8

// crawlAcrossExecutions 跨执行并行爬取的统一入口（2026-11-04 破坏性重构：
// tasks/stories 各写一遍 sem+wg+select+deadline，select 在 sem/ctx.Done
// 双就绪时随机选择曾把 60s 预算拖成 137s——换 oekit fanout.Map）。
//
// 流程：共享全执行列表（executions:all SWR，空列表防毒化）→ fanout.Map
// 有界并发爬取（budget 从列表就绪后起算，ctx 打进每个 HTTP 请求，截止
// 确定性收口）→ 按 ID 去重（dedupID 非 nil）→ 失败项记日志不致命。
func crawlAcrossExecutions[T any](
	cache *CacheService,
	client *myzentao.Client,
	budget time.Duration,
	dedupID func(T) int,
	fetchOne func(ctx context.Context, execID int) ([]T, error),
) ([]T, error) {
	t0 := time.Now()
	allExecs, err := allExecutionsViaSWR(cache, client)
	if err != nil {
		return nil, err
	}
	logger.Info("跨执行爬取：执行列表就绪",
		"elapsed", time.Since(t0).Round(time.Millisecond),
		"executions", len(allExecs),
	)
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()

	execIDs := make([]int, len(allExecs))
	for i, e := range allExecs {
		execIDs[i] = e.ID
	}
	results, err := fanout.Map(ctx, execIDs, crawlWorkers, func(ctx context.Context, execID int) ([]T, error) {
		return fetchOne(ctx, execID)
	})

	var out []T
	seen := map[int]struct{}{}
	for _, rs := range results {
		for _, item := range rs {
			if dedupID != nil {
				id := dedupID(item)
				if _, dup := seen[id]; dup {
					continue
				}
				seen[id] = struct{}{}
			}
			out = append(out, item)
		}
	}

	if err != nil {
		var fe *fanout.Error
		if errors.As(err, &fe) {
			logger.Warn("跨执行爬取部分失败（预算收口/单执行错误，返回已收集部分）",
				"failed", len(fe.Failed),
				"executions", len(execIDs),
				"collected", len(out),
				"elapsed_total", time.Since(t0).Round(time.Millisecond),
			)
		}
	}
	logger.Info("跨执行爬取完成",
		"collected", len(out),
		"executions", len(execIDs),
		"elapsed_total", time.Since(t0).Round(time.Millisecond),
	)
	return out, nil
}

// fetchTasksAcrossExecutions tasks 版爬取（fetch 走 GetTasksContext，
// ctx 能掐断在途 HTTP）。
func (s *TaskService) fetchTasksAcrossExecutions(budget time.Duration) ([]zentaosdk.Task, error) {
	return crawlAcrossExecutions(s.cache, s.client, budget, nil,
		func(ctx context.Context, execID int) ([]zentaosdk.Task, error) {
			return s.client.GetTasksContext(ctx, execID, 1, 10000)
		})
}

// fetchStoriesAcrossExecutions stories 版爬取（跨执行去重：同一需求挂多
// 执行只算一次；fetch 走 GetStoriesByExecutionContext）。
func (s *StoryService) fetchStoriesAcrossExecutions(budget time.Duration) ([]zentaosdk.Story, error) {
	return crawlAcrossExecutions(s.cache, s.client, budget, func(st zentaosdk.Story) int { return st.ID },
		func(ctx context.Context, execID int) ([]zentaosdk.Story, error) {
			return s.client.GetStoriesByExecutionContext(ctx, execID, 1, 10000)
		})
}
