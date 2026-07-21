package utils

import (
	"context"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/core/errors"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

type PaginationParams struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

func ParsePagination(c *app.RequestContext) PaginationParams {
	pageStr := c.DefaultQuery("page", strconv.Itoa(DefaultPage))
	pageSizeStr := c.DefaultQuery("pageSize", strconv.Itoa(DefaultPageSize))

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = DefaultPage
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > MaxPageSize {
		pageSize = DefaultPageSize
	}

	return PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}
}

func ParsePaginationWithMax(c *app.RequestContext, maxPageSize int) PaginationParams {
	params := ParsePagination(c)
	if params.PageSize > maxPageSize {
		params.PageSize = maxPageSize
	}
	return params
}

func Paginate(total, page, pageSize int) (start, end int) {
	if total == 0 || page <= 0 {
		return 0, 0
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	start = (page - 1) * pageSize
	if start >= total {
		return 0, 0
	}

	end = start + pageSize
	if end > total {
		end = total
	}

	return start, end
}

func PaginateSlice[T any](slice []T, page, pageSize int) []T {
	total := len(slice)
	start, end := Paginate(total, page, pageSize)

	if start >= total {
		return []T{}
	}

	return slice[start:end]
}

func PaginationMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		params := ParsePagination(c)
		c.Set("pagination", params)
		c.Next(ctx)
	}
}

func GetPagination(c *app.RequestContext) PaginationParams {
	if params, exists := c.Get("pagination"); exists {
		return params.(PaginationParams)
	}
	return PaginationParams{
		Page:     DefaultPage,
		PageSize: DefaultPageSize,
	}
}

func ParseIntParam(c *app.RequestContext, paramName string) (int, error) {
	valueStr := c.Query(paramName)
	if valueStr == "" {
		return 0, errors.NewMissingParam(paramName)
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, errors.NewInvalidParam(paramName)
	}

	return value, nil
}

func ParseOptionalIntParam(c *app.RequestContext, paramName string) (int, error) {
	valueStr := c.Query(paramName)
	if valueStr == "" {
		return 0, nil
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, errors.NewInvalidParam(paramName)
	}

	return value, nil
}

func ParseRequiredIntParam(c *app.RequestContext, paramName string, displayName string) (int, error) {
	valueStr := c.Query(paramName)
	if valueStr == "" {
		return 0, errors.NewMissingParam(paramName)
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, errors.New(errors.CodeInvalidID, "无效的"+displayName)
	}

	return value, nil
}
