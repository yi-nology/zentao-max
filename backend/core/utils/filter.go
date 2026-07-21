package utils

import (
	"slices"
	"strings"
)

// FilterFunc 过滤函数类型
type FilterFunc[T any] func(item T) bool

// Filter 过滤切片
func Filter[T any](slice []T, filterFunc FilterFunc[T]) []T {
	if len(slice) == 0 {
		return []T{}
	}

	result := make([]T, 0)
	for _, item := range slice {
		if filterFunc(item) {
			result = append(result, item)
		}
	}
	return result
}

// FilterByDateRange 按日期范围过滤
// dateField 是获取日期字段的函数
func FilterByDateRange[T any](slice []T, startDate, endDate string, getDateFunc func(T) string) []T {
	if startDate == "" && endDate == "" {
		return slice
	}

	return Filter(slice, func(item T) bool {
		date := getDateFunc(item)
		if date == "" {
			return false
		}

		// 提取日期部分（去除时间）
		if len(date) > 10 {
			date = date[:10]
		}

		if startDate != "" && date < startDate {
			return false
		}
		if endDate != "" && date > endDate {
			return false
		}

		return true
	})
}

// FilterBySpecificDate 按具体日期过滤
func FilterBySpecificDate[T any](slice []T, specificDate string, getDateFunc func(T) string) []T {
	if specificDate == "" {
		return slice
	}

	return Filter(slice, func(item T) bool {
		date := getDateFunc(item)
		return strings.HasPrefix(date, specificDate)
	})
}

// FilterByDateRangeOrSpecific 按日期范围或具体日期过滤
// 如果有具体日期，优先使用具体日期过滤
func FilterByDateRangeOrSpecific[T any](slice []T, startDate, endDate, specificDate string, getDateFunc func(T) string) []T {
	if specificDate != "" {
		return FilterBySpecificDate(slice, specificDate, getDateFunc)
	}
	return FilterByDateRange(slice, startDate, endDate, getDateFunc)
}

// FilterByField 按字段值过滤
func FilterByField[T any, V comparable](slice []T, value V, getValueFunc func(T) V) []T {
	return Filter(slice, func(item T) bool {
		return getValueFunc(item) == value
	})
}

// FilterByOptionalField 按可选字段值过滤
// 如果value为零值，则不过滤
func FilterByOptionalField[T any, V comparable](slice []T, value V, getValueFunc func(T) V) []T {
	var zero V
	if value == zero {
		return slice
	}
	return FilterByField(slice, value, getValueFunc)
}

// FilterByStringField 按字符串字段过滤（不区分大小写）
func FilterByStringField[T any](slice []T, value string, getStringFunc func(T) string) []T {
	if value == "" {
		return slice
	}

	return Filter(slice, func(item T) bool {
		return strings.EqualFold(getStringFunc(item), value)
	})
}

// SortFunc 排序函数类型
type SortFunc[T any] func(a, b T) bool

// Sort 对切片进行排序
func Sort[T any](slice []T, less SortFunc[T]) []T {
	if len(slice) <= 1 {
		return slice
	}

	result := make([]T, len(slice))
	copy(result, slice)

	slices.SortFunc(result, func(a, b T) int {
		if less(a, b) {
			return -1
		}
		if less(b, a) {
			return 1
		}
		return 0
	})

	return result
}

// SortByStringField 按字符串字段排序（升序）
func SortByStringField[T any](slice []T, getStringFunc func(T) string) []T {
	return Sort(slice, func(a, b T) bool {
		return getStringFunc(a) < getStringFunc(b)
	})
}

// SortByIntField 按整数字段排序（升序）
func SortByIntField[T any](slice []T, getIntFunc func(T) int) []T {
	return Sort(slice, func(a, b T) bool {
		return getIntFunc(a) < getIntFunc(b)
	})
}

// SortByDateField 按日期字段排序（升序，最新的在前）
func SortByDateField[T any](slice []T, getDateFunc func(T) string) []T {
	return Sort(slice, func(a, b T) bool {
		return getDateFunc(a) > getDateFunc(b)
	})
}

// ChainFilter 链式过滤器
type ChainFilter[T any] struct {
	data []T
}

// NewChainFilter 创建链式过滤器
func NewChainFilter[T any](data []T) *ChainFilter[T] {
	return &ChainFilter[T]{data: data}
}

// Filter 执行过滤
func (cf *ChainFilter[T]) Filter(filterFunc FilterFunc[T]) *ChainFilter[T] {
	cf.data = Filter(cf.data, filterFunc)
	return cf
}

// FilterByDate 按日期范围过滤
func (cf *ChainFilter[T]) FilterByDate(startDate, endDate string, getDateFunc func(T) string) *ChainFilter[T] {
	cf.data = FilterByDateRange(cf.data, startDate, endDate, getDateFunc)
	return cf
}

// FilterBySpecificDate 按具体日期过滤
func (cf *ChainFilter[T]) FilterBySpecificDate(specificDate string, getDateFunc func(T) string) *ChainFilter[T] {
	cf.data = FilterBySpecificDate(cf.data, specificDate, getDateFunc)
	return cf
}

// FilterByFieldFunc 按字段过滤（使用函数）
func (cf *ChainFilter[T]) FilterByFieldFunc(filterFunc FilterFunc[T]) *ChainFilter[T] {
	cf.data = Filter(cf.data, filterFunc)
	return cf
}

// FilterByString 按字符串字段过滤
func (cf *ChainFilter[T]) FilterByString(value string, getStringFunc func(T) string) *ChainFilter[T] {
	cf.data = FilterByStringField(cf.data, value, getStringFunc)
	return cf
}

// Sort 执行排序
func (cf *ChainFilter[T]) Sort(less SortFunc[T]) *ChainFilter[T] {
	cf.data = Sort(cf.data, less)
	return cf
}

// SortByDate 按日期排序
func (cf *ChainFilter[T]) SortByDate(getDateFunc func(T) string) *ChainFilter[T] {
	cf.data = SortByDateField(cf.data, getDateFunc)
	return cf
}

// Paginate 执行分页
func (cf *ChainFilter[T]) Paginate(page, pageSize int) *ChainFilter[T] {
	cf.data = PaginateSlice(cf.data, page, pageSize)
	return cf
}

// Result 获取结果
func (cf *ChainFilter[T]) Result() []T {
	return cf.data
}

// Count 获取数量
func (cf *ChainFilter[T]) Count() int {
	return len(cf.data)
}
