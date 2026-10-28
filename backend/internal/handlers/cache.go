package handlers

import (
	"context"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/pkg/errors"
	"github.com/yi-nology/zentao-max/backend/internal/service"
)

// CacheHandler 离线缓存管理接口
type CacheHandler struct {
	cacheService *service.CacheService
}

func NewCacheHandler(cs *service.CacheService) *CacheHandler {
	return &CacheHandler{cacheService: cs}
}

// GetStatus GET /api/cache/status - 返回缓存统计
func (h *CacheHandler) GetStatus(ctx context.Context, c *app.RequestContext) {
	status, err := h.cacheService.Status(ctx)
	if err != nil {
		errors.InternalError(c, "获取缓存状态失败")
		return
	}
	errors.Success(c, status)
}

// ClearAll DELETE /api/cache - 清空所有缓存
func (h *CacheHandler) ClearAll(ctx context.Context, c *app.RequestContext) {
	if err := h.cacheService.ClearAll(ctx); err != nil {
		errors.InternalError(c, "清空缓存失败")
		return
	}
	errors.Success(c, map[string]interface{}{"cleared": true})
}

// Invalidate DELETE /api/cache/:entityType?productId=100 - 失效指定缓存
func (h *CacheHandler) Invalidate(ctx context.Context, c *app.RequestContext) {
	entityType := c.Param("entityType")
	if entityType == "" {
		errors.BadRequest(c, "缺少 entityType 参数")
		return
	}
	productIDStr := c.Query("productId")
	productID, _ := strconv.Atoi(string(productIDStr))
	if err := h.cacheService.Invalidate(ctx, entityType, productID); err != nil {
		errors.InternalError(c, "失效缓存失败")
		return
	}
	errors.Success(c, map[string]interface{}{"invalidated": true})
}
