package zentao

import (
	"context"
	"sync"

	"github.com/oekit/concurrency/async"
	"github.com/yi-nology/zentao-max/backend/pkg/logger"
)

// fetchPagesConcurrent 并发拉取第 2..pageTotal 页（信号量限流 + GoSafe 收编 panic），
// 结果按各自下标写入（无竞争），返回每页结果与缺页数——失败页为 nil，
// 语义与 products/bugs/timelog 三处手搓 sem/wg 版本完全一致（issue #3 归一）。
//
// 缺页属于部分数据语义：记日志返回已收集部分，缓存过期重拉可自愈。
// fetchPage 拿到页码自行携带业务上下文（productID 等），单页失败仅记日志。
func fetchPagesConcurrent[T any](ctx context.Context, concurrency, pageTotal int, fetchPage func(ctx context.Context, page int) ([]T, error)) ([][]T, int) {
	if pageTotal < 2 {
		return nil, 0
	}
	results := make([][]T, pageTotal-1)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for p := 2; p <= pageTotal; p++ {
		wg.Add(1)
		page := p
		async.GoSafe(func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			items, err := fetchPage(ctx, page)
			if err != nil {
				logger.Warn("分页并发拉取失败", "page", page, "error", err)
				return
			}
			results[page-2] = items
		})
	}
	wg.Wait()
	missing := 0
	for _, r := range results {
		if r == nil {
			missing++
		}
	}
	return results, missing
}
