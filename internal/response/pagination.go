package response

import (
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	DefaultPageSize = 15
	MaxPageSize     = 1000
)

// Pagination 表示响应中的分页详情。
type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

// PaginatedResponse 是所有分页 API 响应的标准结构。
type PaginatedResponse struct {
	Items      any        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

// Paginate 对 GORM 查询执行分页并返回标准化响应。
// 接收 Gin 上下文、GORM 查询构建器和结果的目标切片。
func Paginate(c *gin.Context, query *gorm.DB, dest any) (*PaginatedResponse, error) {
	// 1. 从查询参数获取页码和每页大小
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", strconv.Itoa(DefaultPageSize)))
	if err != nil || pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}

	// 2. 获取项目总数
	var totalItems int64
	if err := query.Count(&totalItems).Error; err != nil {
		return nil, err
	}

	// 3. 计算偏移量和总页数
	offset := (page - 1) * pageSize
	totalPages := int(math.Ceil(float64(totalItems) / float64(pageSize)))

	// 4. 检索当前页的数据
	if err := query.Limit(pageSize).Offset(offset).Find(dest).Error; err != nil {
		return nil, err
	}

	// 5. 构建分页响应
	paginatedData := &PaginatedResponse{
		Items: dest,
		Pagination: Pagination{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: totalItems,
			TotalPages: totalPages,
		},
	}

	return paginatedData, nil
}
