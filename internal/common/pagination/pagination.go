package pagination

import (
	"math"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

const (
	DefaultPage  = 1
	DefaultLimit = 20
	MaxLimit     = 100
)

type PaginationQuery struct {
	Page  int
	Limit int
	Q     string
	Sort  string
	Order string
}

type PaginationMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

type PaginatedList[T any] struct {
	Items      []T            `json:"items"`
	Pagination PaginationMeta `json:"pagination"`
}

func ParsePagination(c echo.Context) PaginationQuery {
	return PaginationQuery{
		Page:  SanitizePage(queryInt(c, "page")),
		Limit: SanitizeLimit(queryInt(c, "limit")),
		Q:     strings.TrimSpace(c.QueryParam("q")),
		Sort:  strings.TrimSpace(c.QueryParam("sort")),
		Order: strings.TrimSpace(c.QueryParam("order")),
	}
}

func queryInt(c echo.Context, key string) int {
	v, _ := strconv.Atoi(c.QueryParam(key))
	return v
}

func SanitizePage(page int) int {
	if page < 1 {
		return DefaultPage
	}
	return page
}

func SanitizeLimit(limit int) int {
	if limit < 1 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}

func Sanitize(page, limit int) (int, int) {
	return SanitizePage(page), SanitizeLimit(limit)
}

func NewPaginationMeta(page, limit int, total int64) PaginationMeta {
	page = SanitizePage(page)
	limit = SanitizeLimit(limit)
	totalPages := 1
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(limit)))
	}
	return PaginationMeta{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}

func Offset(page, limit int) int {
	page, limit = Sanitize(page, limit)
	return (page - 1) * limit
}

func ResolveSort(sort string, allowed map[string]string, defaultSort string) string {
	if sort == "" {
		return defaultSort
	}
	if col, ok := allowed[sort]; ok {
		return col
	}
	return defaultSort
}

func ResolveOrder(order string, defaultDesc bool) string {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "asc":
		return "ASC"
	case "desc":
		return "DESC"
	default:
		if defaultDesc {
			return "DESC"
		}
		return "ASC"
	}
}

func OrderClause(sortCol, orderDir string) string {
	return sortCol + " " + orderDir
}

func NewList[T any](items []T, page, limit int, total int64) PaginatedList[T] {
	page, limit = Sanitize(page, limit)
	return PaginatedList[T]{
		Items:      items,
		Pagination: NewPaginationMeta(page, limit, total),
	}
}
