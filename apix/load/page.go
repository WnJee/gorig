package load

import (
	"context"
	"encoding/json"

	"github.com/WnJee/gorig/utils/logger"
	"go.uber.org/zap"
)

const (
	DefaultPageNum  int64 = 1
	DefaultPageSize int64 = 10
	MaxPageSize     int64 = 10000
)

// Page represents pagination query parameters.
type Page struct {
	Page   int64 `json:"page" form:"page" query:"page"`
	Size   int64 `json:"size" form:"size" query:"size"`
	LastID int64 `json:"lastID" form:"lastID" query:"lastID"`
}

// Total wraps the total records count.
type Total int64

// Set updates the total value.
func (t *Total) Set(total int64) {
	if t == nil {
		t = new(Total)
	}
	*t = Total(total)
}

// Get returns the total value as int64, safely handling nil.
func (t *Total) Get() int64 {
	if t == nil {
		return 0
	}
	return int64(*t)
}

// NewTotal creates a new Total pointer.
func NewTotal(total int64) *Total {
	t := Total(total)
	return &t
}

// NewPage creates a sanitized Page instance with defaults.
func NewPage(page, size int64, lastID ...int64) *Page {
	if page <= 0 {
		page = DefaultPageNum
	}
	if size <= 0 {
		size = DefaultPageSize
	} else if size > MaxPageSize {
		size = MaxPageSize
	}
	var lID int64
	if len(lastID) > 0 && lastID[0] > 0 {
		lID = lastID[0]
	}
	return &Page{
		Page:   page,
		Size:   size,
		LastID: lID,
	}
}

// DefaultPage returns a Page with standard default values (page 1, size 10).
func DefaultPage() *Page {
	return &Page{
		Page:   DefaultPageNum,
		Size:   DefaultPageSize,
		LastID: 0,
	}
}

// BuildPage creates a Page instance from request params (context is optional).
func BuildPage(ctx context.Context, page, pageSize, lastId int64) *Page {
	return NewPage(page, pageSize, lastId)
}

// Offset returns SQL offset calculation ((page - 1) * size).
func (p *Page) Offset() int64 {
	if p == nil || p.Page <= 1 || p.Size <= 0 {
		return 0
	}
	return (p.Page - 1) * p.Size
}

// Limit returns SQL limit calculation.
func (p *Page) Limit() int64 {
	if p == nil || p.Size <= 0 {
		return DefaultPageSize
	}
	return p.Size
}

// NextPage returns the next page number.
func (p *Page) NextPage() int64 {
	if p == nil {
		return DefaultPageNum + 1
	}
	return p.Page + 1
}

// PrevPage returns the previous page number (min 1).
func (p *Page) PrevPage() int64 {
	if p == nil || p.Page <= 1 {
		return DefaultPageNum
	}
	return p.Page - 1
}

// SetPage sets the current page number.
func (p *Page) SetPage(page int64) {
	if p != nil {
		p.Page = page
	}
}

// TotalPages calculates total page count based on record count.
func (p *Page) TotalPages(total int64) int64 {
	if p == nil || p.Size <= 0 || total <= 0 {
		return 0
	}
	return (total + p.Size - 1) / p.Size
}

// HasNext checks whether there is a next page for given total count.
func (p *Page) HasNext(total int64) bool {
	if p == nil {
		return false
	}
	return p.Page < p.TotalPages(total)
}

// HasPrev checks whether there is a previous page.
func (p *Page) HasPrev() bool {
	if p == nil {
		return false
	}
	return p.Page > 1
}

// Sanitize normalizes page numbers and enforces max size bounds.
func (p *Page) Sanitize(defaultSize, maxSize int64) *Page {
	if p == nil {
		return NewPage(1, defaultSize)
	}
	if p.Page <= 0 {
		p.Page = DefaultPageNum
	}
	if p.Size <= 0 {
		if defaultSize > 0 {
			p.Size = defaultSize
		} else {
			p.Size = DefaultPageSize
		}
	}
	if maxSize > 0 && p.Size > maxSize {
		p.Size = maxSize
	}
	if p.LastID < 0 {
		p.LastID = 0
	}
	return p
}

// PageResp represents untyped pagination response.
type PageResp struct {
	Page   int64  `json:"page"`
	Size   int64  `json:"size"`
	Total  *Total `json:"total,omitempty"`
	LastID int64  `json:"lastID"`
	Result any    `json:"result"`
}

// Build populates untyped PageResp.
func (r *PageResp) Build(page *Page, total *Total, lastID int64, result any) {
	if r == nil {
		return
	}
	if page != nil {
		r.Page = page.Page
		r.Size = page.Size
	}
	r.Total = total
	r.LastID = lastID
	r.Result = result
}

// BuildS populates PageResp without total count.
func (r *PageResp) BuildS(page *Page, lastID int64, result any) {
	r.Build(page, nil, lastID, result)
}

// PageRespT represents generic pagination response compatible with pointer-to-slice result.
type PageRespT[T any] struct {
	Page   int64  `json:"page"`
	Size   int64  `json:"size"`
	Total  *Total `json:"total,omitempty"`
	LastID int64  `json:"lastID"`
	Result *[]T   `json:"result"`
}

// Build populates PageRespT.
func (r *PageRespT[T]) Build(page *Page, total *Total, lastID int64, result *[]T) {
	if r == nil {
		return
	}
	if page != nil {
		r.Page = page.Page
		r.Size = page.Size
	}
	r.Total = total
	r.LastID = lastID
	r.Result = result
}

// Items returns the underlying slice safely, never returning nil.
func (r *PageRespT[T]) Items() []T {
	if r == nil || r.Result == nil || *r.Result == nil {
		return []T{}
	}
	return *r.Result
}

// TotalCount returns the total count safely.
func (r *PageRespT[T]) TotalCount() int64 {
	if r == nil || r.Total == nil {
		return 0
	}
	return r.Total.Get()
}

// TotalPages returns total page count safely.
func (r *PageRespT[T]) TotalPages() int64 {
	if r == nil || r.Size <= 0 || r.Total == nil {
		return 0
	}
	return (r.Total.Get() + r.Size - 1) / r.Size
}

// ParsePageResp copies pagination fields from PageResp.
func (t *PageRespT[T]) ParsePageResp(r *PageResp, result *[]T) *PageRespT[T] {
	if t == nil {
		t = &PageRespT[T]{}
	}
	if r != nil {
		t.Page = r.Page
		t.Size = r.Size
		t.Total = r.Total
		t.LastID = r.LastID
	}
	t.Result = result
	return t
}

// NewPageResp creates a new PageResp with slice result.
func NewPageResp(page *Page, total int64, lastID int64, result any) *PageResp {
	p := page
	if p == nil {
		p = DefaultPage()
	}
	return &PageResp{
		Page:   p.Page,
		Size:   p.Size,
		Total:  NewTotal(total),
		LastID: lastID,
		Result: result,
	}
}

// NewPageRespT creates a generic PageRespT with populated items slice.
func NewPageRespT[T any](page *Page, total int64, lastID int64, items []T) *PageRespT[T] {
	p := page
	if p == nil {
		p = DefaultPage()
	}
	list := make([]T, len(items))
	copy(list, items)
	return &PageRespT[T]{
		Page:   p.Page,
		Size:   p.Size,
		Total:  NewTotal(total),
		LastID: lastID,
		Result: &list,
	}
}

// EmptyPageRespT creates an empty generic PageRespT.
func EmptyPageRespT[T any](page ...*Page) *PageRespT[T] {
	var p *Page
	if len(page) > 0 && page[0] != nil {
		p = page[0]
	} else {
		p = DefaultPage()
	}
	list := make([]T, 0)
	return &PageRespT[T]{
		Page:   p.Page,
		Size:   p.Size,
		Total:  NewTotal(0),
		LastID: 0,
		Result: &list,
	}
}

// Convert converts untyped PageResp to generic PageRespT[T].
func Convert[T any](r *PageResp) *PageRespT[T] {
	if r == nil {
		list := make([]T, 0)
		return &PageRespT[T]{Result: &list}
	}
	if typed, ok := r.Result.(*[]T); ok && typed != nil {
		return &PageRespT[T]{
			Page:   r.Page,
			Size:   r.Size,
			Total:  r.Total,
			LastID: r.LastID,
			Result: typed,
		}
	}
	if typedList, ok := r.Result.([]T); ok {
		list := make([]T, len(typedList))
		copy(list, typedList)
		return &PageRespT[T]{
			Page:   r.Page,
			Size:   r.Size,
			Total:  r.Total,
			LastID: r.LastID,
			Result: &list,
		}
	}

	result := new([]T)
	if r.Result != nil {
		b, err := json.Marshal(r.Result)
		if err == nil {
			if err := json.Unmarshal(b, result); err == nil {
				return &PageRespT[T]{
					Page:   r.Page,
					Size:   r.Size,
					Total:  r.Total,
					LastID: r.LastID,
					Result: result,
				}
			} else {
				logger.Error(nil, "Convert PageResp unmarshal failed", zap.Error(err))
			}
		}
	}

	list := make([]T, 0)
	return &PageRespT[T]{
		Page:   r.Page,
		Size:   r.Size,
		Total:  r.Total,
		LastID: r.LastID,
		Result: &list,
	}
}

// Covert is an alias for Convert (maintaining backward compatibility).
func Covert[T any](r *PageResp) *PageRespT[T] {
	return Convert[T](r)
}
