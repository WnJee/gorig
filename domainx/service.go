package domainx

import (
	"context"
	"fmt"
	"github.com/jom-io/gorig/apix/load"
	"github.com/jom-io/gorig/utils/sys"
	"sync"
)

var service = &serviceInfo{
	dbService: sync.Map{},
}

type serviceInfo struct {
	dbService sync.Map
}

func (s *serviceInfo) Start(code, port string) error {
	var err error
	s.dbService.Range(func(key, value interface{}) bool {
		err = value.(DBService).Start()
		return err == nil
	})
	if err != nil {
		return err
	}
	for _, m := range MigrationList {
		if err = s.Migrate(m); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	return err
}

func (s *serviceInfo) End(code string, ctx context.Context) error {
	var err error
	s.dbService.Range(func(key, value interface{}) bool {
		err = value.(DBService).End()
		return err == nil
	})
	return err
}

func (s *serviceInfo) Migrate(m *Migration) error {
	value := m.DBFunc()
	if value == nil {
		return nil
	}
	con := value.GetCon()
	if con == nil {
		return nil
	}
	if value == nil {
		return fmt.Errorf("AutoMigrate value is nil, %v", value)
	}
	if value.TableName() == "" {
		return fmt.Errorf("AutoMigrate TableName is nil, %v", value)
	}
	tableName := value.TableName()
	sys.Info(" * AutoMigrate: ", con.GetConType()+" ", tableName)
	dbService := GetDBService(con.GetConType())
	if dbService == nil {
		return fmt.Errorf("database service not registered: %s", con.GetConType())
	}
	return dbService.Migrate(con, tableName, value, m.Index)
}

type DBService interface {
	Start() error
	End() error
	Migrate(con *Con, tableName string, value ConTable, indexList []Index) error
	GetByID(c *Con, id int64, result interface{}) error
	Save(c *Con, data Identifiable, newID int64, version ...int) (id int64, error error)
	UpdatePart(c *Con, id int64, data map[string]interface{}) error
	UpdateByMatch(c *Con, matchList []Match, data map[string]interface{}) error
	Delete(c *Con, data Identifiable) error
	DeleteByMatch(c *Con, matchList []Match) error
	FindByMatch(c *Con, matchList []Match, result interface{}, prefixes ...string) error
	GetByMatch(c *Con, matchList []Match, result interface{}) error
	CountByMatch(c *Con, matchList []Match) (int64, error)
	ExistsByMatch(c *Con, matchList []Match) (bool, error)
	SumByMatch(c *Con, matchList []Match, field string) (float64, error)
	FindByPageMatch(c *Con, matchList []Match, page *load.Page, total *load.Total, result interface{}, prefixes ...string) error
}

func RegisterDBService(conType ConType, s DBService) {
	service.dbService.Store(conType, s)
}

func GetDBService(conType ConType) DBService {
	if v, ok := service.dbService.Load(conType); ok {
		return v.(DBService)
	}
	return nil
}
