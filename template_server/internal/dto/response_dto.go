package dto

// Response 通用响应结构体
type Response struct {
	Code      int         `json:"code"`
	Timestamp int64       `json:"timestamp"`
	Msg       string      `json:"msg"`
	Data      interface{} `json:"data,omitempty"`
}

// PaginationRequest 分页请求
type PaginationRequest struct {
	Page     int    `json:"page" query:"page" validate:"min=1"`
	PageSize int    `json:"pageSize" query:"page_size" validate:"min=1,max=100"`
	SortBy   string `json:"sortBy" query:"sort_by"`
	Order    string `json:"order" query:"order" validate:"oneof=asc desc"`
}

// PaginationResponse 分页响应
type PaginationResponse struct {
	Total       int64       `json:"total"`
	Page        int         `json:"page"`
	PageSize    int         `json:"pageSize"`
	TotalPages  int         `json:"totalPages"`
	HasNext     bool        `json:"hasNext"`
	HasPrevious bool        `json:"hasPrevious"`
	Data        interface{} `json:"data"`
}
