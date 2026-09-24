package dx

import (
	"context"
	"github.com/WnJee/gorig/apix/load"
	"github.com/WnJee/gorig/domainx"
	"github.com/WnJee/gorig/utils/errors"
)

type (
	dx[T any] struct {
		ctx     context.Context
		complex *domainx.Complex[T]
		matches *domainx.Matches
	}

	DTable interface {
		DConfig() (conType domainx.ConType, dbName string, tableName string)
	}

	DQuery[T any] interface {
		WithContext(ctx context.Context) DQuery[T]

		Complex() *domainx.Complex[T]
		GetData() *T
		GetCon() *domainx.Con
		SetID(id int64)
		GetID() domainx.ID
		GenerateID() DQuery[T]
		isNil() bool
		IsZero() bool

		WithID(id int64) DQuery[T]
		// Eq Ne Gt Gte Lt Lte Like In NotIn ignore is used to ignore the field empty check
		Eq(field string, value interface{}, ignore ...bool) DQuery[T]
		Ne(field string, value interface{}, ignore ...bool) DQuery[T]
		Gt(field string, value interface{}, ignore ...bool) DQuery[T]
		Gte(field string, value interface{}, ignore ...bool) DQuery[T]
		Lt(field string, value interface{}, ignore ...bool) DQuery[T]
		Lte(field string, value interface{}, ignore ...bool) DQuery[T]
		Like(field string, value string, ignore ...bool) DQuery[T]
		In(field string, value interface{}, ignore ...bool) DQuery[T]
		NotIn(field string, value interface{}, ignore ...bool) DQuery[T]
		// Array helpers for []T fields (e.g., Tags []string)
		Has(field string, value interface{}, ignore ...bool) DQuery[T]
		HasAny(field string, value interface{}, ignore ...bool) DQuery[T]
		HasAll(field string, value interface{}, ignore ...bool) DQuery[T]
		NEmpty(field string) DQuery[T]
		Near(latField, lngField string, lat, lng, distance float64) DQuery[T]
		NearLoc(localField string, lat, lng, distance float64) DQuery[T]
		AddMatch(m *domainx.Match) DQuery[T]
		AddMatches(ms *domainx.Matches) DQuery[T]
		Sort(field string, asc ...bool) DQuery[T]
		Limit(limit int) DQuery[T]
		Select(fields ...string) DQuery[T]
		Omit(fields ...string) DQuery[T]

		Save(t ...*T) (id int64, err *errors.Error)
		checkMatches() *errors.Error
		Update(field string, value any) *errors.Error
		Updates(data map[string]interface{}) *errors.Error
		Delete() *errors.Error
		First() (*domainx.Complex[T], *errors.Error)
		Get() (*domainx.Complex[T], *errors.Error)
		Find() (domainx.ComplexList[T], *errors.Error)
		FindEach(handle func(*domainx.Complex[T]) *errors.Error) *errors.Error
		AllEach(handle func(*domainx.Complex[T]) *errors.Error) *errors.Error
		Count() (int64, *errors.Error)
		Exists() (bool, *errors.Error)
		Sum(field string) (float64, *errors.Error)
		Page(page, size int64, lastID ...int64) (*load.PageRespT[*domainx.Complex[T]], *errors.Error)
	}
)

func On[T any, PT interface {
	*T
	DTable
}](ctx context.Context, t ...*T) DQuery[T] {
	var inst T
	if len(t) > 0 && any(t[0]) != nil && t[0] != nil {
		inst = *t[0]
	} else {
		inst = *new(T)
	}

	ptr := PT(&inst)

	conType, dbName, TableName := ptr.DConfig()
	return &dx[T]{
		ctx:     ctx,
		complex: domainx.CreateComplex(ctx, conType, dbName, TableName, &inst),
		matches: domainx.NewMatches(),
	}
}

func (d *dx[T]) WithContext(ctx context.Context) DQuery[T] {
	d.ctx = ctx
	if d.complex != nil && d.complex.Con != nil {
		d.complex.Con.Ctx = ctx
		if tx := domainx.GetTxFromContext(ctx, d.complex.Con.DBName); tx != nil {
			d.complex.Con.MysqlDB = tx
		}
	}
	return d
}

func (d *dx[T]) Complex() *domainx.Complex[T] {
	if d == nil {
		return nil
	}
	return d.complex
}

func (d *dx[T]) GetData() *T {
	if d == nil || d.complex == nil {
		return nil
	}
	return d.complex.Data
}

func (d *dx[T]) GetCon() *domainx.Con {
	if d == nil || d.complex == nil {
		return nil
	}
	return d.complex.Con
}

func (d *dx[T]) SetID(id int64) {
	if con := d.GetCon(); con != nil {
		con.SetID(id)
	}
}

func (d *dx[T]) GetID() domainx.ID {
	if con := d.GetCon(); con != nil {
		return con.GetID()
	}
	return 0
}

func (d *dx[T]) WithID(id int64) DQuery[T] {
	d.SetID(id)
	return d
}

func (d *dx[T]) GenerateID() DQuery[T] {
	if con := d.GetCon(); con != nil {
		con.GenerateID()
	}
	return d
}

func (d *dx[T]) isNil() bool {
	return d == nil || d.complex == nil || d.complex.Con == nil || d.GetData() == nil
}

func (d *dx[T]) ready() *errors.Error {
	if d == nil || d.complex == nil || d.complex.Con == nil {
		return errors.Sys("database connection is not initialized")
	}
	return nil
}

func (d *dx[T]) IsZero() bool {
	return !d.isNil() && d.complex.GetID().IsZero()
}

func (d *dx[T]) Eq(field string, value interface{}, ignore ...bool) DQuery[T] {
	d.matches.Eq(field, value, ignore...)
	return d
}

func (d *dx[T]) Ne(field string, value interface{}, ignore ...bool) DQuery[T] {
	d.matches.Ne(field, value, ignore...)
	return d
}

func (d *dx[T]) Gt(field string, value interface{}, ignore ...bool) DQuery[T] {
	d.matches.Gt(field, value, ignore...)
	return d
}

func (d *dx[T]) Gte(field string, value interface{}, ignore ...bool) DQuery[T] {
	d.matches.Gte(field, value, ignore...)
	return d
}

func (d *dx[T]) Lt(field string, value interface{}, ignore ...bool) DQuery[T] {
	d.matches.Lt(field, value, ignore...)
	return d
}

func (d *dx[T]) Lte(field string, value interface{}, ignore ...bool) DQuery[T] {
	d.matches.Lte(field, value, ignore...)
	return d
}

func (d *dx[T]) Like(field string, value string, ignore ...bool) DQuery[T] {
	d.matches.Like(field, value, ignore...)
	return d
}

func (d *dx[T]) In(field string, value interface{}, ignore ...bool) DQuery[T] {
	d.matches.In(field, value, ignore...)
	return d
}

func (d *dx[T]) NotIn(field string, value interface{}, ignore ...bool) DQuery[T] {
	d.matches.NotIn(field, value, ignore...)
	return d
}

func (d *dx[T]) Has(field string, value interface{}, ignore ...bool) DQuery[T] {
	d.matches.Has(field, value, ignore...)
	return d
}

func (d *dx[T]) HasAny(field string, value interface{}, ignore ...bool) DQuery[T] {
	d.matches.HasAny(field, value, ignore...)
	return d
}

func (d *dx[T]) HasAll(field string, value interface{}, ignore ...bool) DQuery[T] {
	d.matches.HasAll(field, value, ignore...)
	return d
}

func (d *dx[T]) NEmpty(field string) DQuery[T] {
	d.matches.NEmpty(field)
	return d
}

func (d *dx[T]) Near(latField, lngField string, lat, lng, distance float64) DQuery[T] {
	d.matches.Near(latField, lngField, lat, lng, distance)
	return d
}

func (d *dx[T]) NearLoc(localField string, lat, lng, distance float64) DQuery[T] {
	d.matches.NearLoc(localField, lat, lng, distance)
	return d
}

func (d *dx[T]) AddMatch(m *domainx.Match) DQuery[T] {
	d.matches.AddMatch(m)
	return d
}

func (d *dx[T]) AddMatches(ms *domainx.Matches) DQuery[T] {
	d.matches.AddMatches(ms)
	return d
}

func (d *dx[T]) Sort(field string, asc ...bool) DQuery[T] {
	if field == "" || d == nil || d.complex == nil || d.complex.Con == nil {
		return d
	}
	d.complex.Sort.AddSort(field, len(asc) > 0 && asc[0])
	return d
}

func (d *dx[T]) Limit(limit int) DQuery[T] {
	if d.complex != nil && d.complex.Con != nil {
		d.complex.Con.SetLimit(limit)
	}
	return d
}

func (d *dx[T]) Select(fields ...string) DQuery[T] {
	if d.complex != nil && d.complex.Con != nil {
		d.complex.Con.SetSelectFields(fields...)
	}
	return d
}

func (d *dx[T]) Omit(fields ...string) DQuery[T] {
	if d.complex != nil && d.complex.Con != nil {
		d.complex.Con.SetOmitFields(fields...)
	}
	return d
}

func (d *dx[T]) Save(t ...*T) (id int64, err *errors.Error) {
	if err := d.ready(); err != nil {
		return 0, err
	}
	if len(t) > 0 && any(t[0]) != nil {
		d.complex.Data = t[0]
	}
	return domainx.Save(d.complex.Con, d.complex, 0)
}

func (d *dx[T]) checkMatches() *errors.Error {
	if err := d.ready(); err != nil {
		return err
	}
	if d.IsZero() && (d.matches == nil || len(*d.matches) == 0) {
		return errors.Sys("id is zero or matches not set")
	}
	if d.matches != nil && len(*d.matches) == 0 {
		return errors.Sys("matches cannot be empty")
	}
	return nil
}

func (d *dx[T]) Update(field string, value any) *errors.Error {
	if field == "" {
		return errors.Sys("field name cannot be empty")
	}
	if value == nil {
		return errors.Sys("value cannot be nil")
	}
	if err := d.ready(); err != nil {
		return err
	}
	if !d.IsZero() {
		return domainx.UpdatePart(d.complex.Con, d.GetID().Int64(), map[string]interface{}{field: value})
	}

	if err := d.checkMatches(); err != nil {
		return err
	}
	return domainx.UpdateByMatch(d.complex.Con, *d.matches, map[string]interface{}{field: value})
}

func (d *dx[T]) Updates(data map[string]interface{}) *errors.Error {
	if len(data) == 0 {
		return errors.Sys("data map cannot be empty")
	}
	if err := d.ready(); err != nil {
		return err
	}
	if !d.IsZero() {
		return domainx.UpdatePart(d.complex.Con, d.GetID().Int64(), data)
	}

	if err := d.checkMatches(); err != nil {
		return err
	}
	return domainx.UpdateByMatch(d.complex.Con, *d.matches, data)
}

func (d *dx[T]) Delete() *errors.Error {
	if err := d.ready(); err != nil {
		return err
	}
	if !d.IsZero() {
		return domainx.Delete(d.complex.Con, d)
	}

	if err := d.checkMatches(); err != nil {
		return err
	}
	return domainx.DeleteByMatch(d.complex.Con, *d.matches)
}

func (d *dx[T]) First() (*domainx.Complex[T], *errors.Error) {
	return d.Get()
}

func (d *dx[T]) Get() (*domainx.Complex[T], *errors.Error) {
	if err := d.ready(); err != nil {
		return nil, err
	}
	if !d.IsZero() {
		if err := domainx.GetByID(d.complex.Con, d.GetID().Int64(), d.complex); err != nil {
			return nil, err
		}
		return d.complex, nil
	}

	if err := d.checkMatches(); err != nil {
		return nil, err
	}
	if err := domainx.GetByMatch(d.complex.Con, *d.matches, d.complex); err != nil {
		return nil, err
	}
	return d.complex, nil
}

func (d *dx[T]) Find() (domainx.ComplexList[T], *errors.Error) {
	if err := d.ready(); err != nil {
		return nil, err
	}
	var matchList []domainx.Match
	if d.matches != nil && len(*d.matches) > 0 {
		matchList = *d.matches
	} else if d.complex.Con.Limit <= 0 {
		d.complex.Con.Limit = 500
	}
	var result []*domainx.Complex[T]
	if err := domainx.FindByMatch(d.complex.Con, matchList, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (d *dx[T]) FindEach(handle func(*domainx.Complex[T]) *errors.Error) *errors.Error {
	find, e := d.Find()
	if e != nil {
		return e
	}
	for _, item := range find {
		if err := handle(item); err != nil {
			return err
		}
	}
	return nil
}

func (d *dx[T]) AllEach(handle func(*domainx.Complex[T]) *errors.Error) *errors.Error {
	if err := d.ready(); err != nil {
		return err
	}
	var lastID int64
	pageSize := int64(1000)
	for {
		pageResp, err := d.Page(1, pageSize, lastID)
		if err != nil {
			return err
		}
		if pageResp == nil || pageResp.Result == nil || len(*pageResp.Result) == 0 {
			break
		}
		for _, item := range *pageResp.Result {
			if err := handle(item); err != nil {
				return err
			}
			lastID = item.GetID().Int64()
		}
		if int64(len(*pageResp.Result)) < pageSize {
			break
		}
		pageResp = nil
	}
	return nil
}

func (d *dx[T]) Count() (int64, *errors.Error) {
	if err := d.ready(); err != nil {
		return 0, err
	}
	count, err := domainx.CountByMatch(d.complex.Con, *d.matches)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (d *dx[T]) Exists() (bool, *errors.Error) {
	if err := d.ready(); err != nil {
		return false, err
	}
	if !d.IsZero() {
		if err := domainx.GetByID(d.complex.Con, d.GetID().Int64(), d.complex); err != nil {
			return false, err
		}
		return !d.isNil(), nil
	}
	if err := d.checkMatches(); err != nil {
		return false, err
	}
	exists, err := domainx.ExistsByMatch(d.complex.Con, *d.matches)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (d *dx[T]) Sum(field string) (float64, *errors.Error) {
	if field == "" {
		return 0, errors.Sys("field name cannot be empty")
	}
	if err := d.ready(); err != nil {
		return 0, err
	}
	sum, err := domainx.SumByMatch(d.complex.Con, *d.matches, field)
	if err != nil {
		return 0, err
	}
	return sum, nil
}

func (d *dx[T]) Page(page, size int64, lastID ...int64) (*load.PageRespT[*domainx.Complex[T]], *errors.Error) {
	if err := d.ready(); err != nil {
		return nil, err
	}
	var lID int64
	if len(lastID) > 0 {
		lID = lastID[0]
	}
	pageLoad := load.BuildPage(d.ctx, page, size, lID)
	resp := &load.PageRespT[*domainx.Complex[T]]{Result: &[]*domainx.Complex[T]{}}
	if err := domainx.FindByPageMatchT(d.complex.Con, *d.matches, pageLoad, resp, resp.Result); err != nil {
		return nil, err
	}
	return resp, nil
}
