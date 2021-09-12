package main

import (
	"fmt"
	"sync"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// IPersister is interface for persister
type IPersister interface {
	WhereSP(model interface{}, sortexpr string, pageLimit int, page int, expr string, args ...interface{}) ( /*result*/ interface{}, error)
	WhereS(model interface{}, sortexpr string, expr string, args ...interface{}) ( /*result*/ interface{}, error)
	WhereP(model interface{}, pageLimit int, page int, expr string, args ...interface{}) ( /*result*/ interface{}, error)
	Where(model interface{}, expr string, args ...interface{}) ( /*result*/ interface{}, error)
	FindOne(model interface{}, idColumn string, id string) ( /*result*/ interface{}, error)
	Create(model interface{}) error
	Update(model interface{}) error
	CreateInBatch(models interface{}, bulkSize int) error
	Exec(sql string, args ...interface{}) error
	TableExists(model interface{}) (bool, error)
}

// IPersisterConfig is interface for persister
type IPersisterConfig interface {
	Endpoint() string
	Port() string
	DB() string
	Username() string
	Password() string
	Charset() string
}

// Persister is persister
type Persister struct {
	config  IPersisterConfig
	db      *gorm.DB
	dbMutex sync.Mutex
}

// NewPersister return new persister
func NewPersister(config IPersisterConfig) *Persister {
	return &Persister{
		config: config,
	}
}

func (pst *Persister) getConnectionString() (string, error) {
	cfg := pst.config

	// connection string refer here https://github.com/go-sql-driver/mysql
	// [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True",
		cfg.Username(),
		cfg.Password(),
		cfg.Endpoint(),
		cfg.Port(),
		cfg.DB(),
		cfg.Charset()), nil
}

func (pst *Persister) getClient() (*gorm.DB, error) {
	if pst.db != nil {
		return pst.db, nil
	}

	pst.dbMutex.Lock()
	defer pst.dbMutex.Unlock()

	connection, err := pst.getConnectionString()
	if err != nil {
		return nil, err
	}
	db, err := gorm.Open(mysql.Open(connection), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	pst.db = db

	return db, nil
}

// TableExists check if table exists
func (pst *Persister) TableExists(model interface{}) (bool, error) {
	db, err := pst.getClient()
	if err != nil {
		return false, err
	}

	has := db.Migrator().HasTable(model)

	return has, nil
}

// Exec execute sql
func (pst *Persister) Exec(sql string, args ...interface{}) error {
	db, err := pst.getClient()
	if err != nil {
		return err
	}

	if err := db.Exec(sql, args).Error; err != nil {
		return err
	}
	return nil
}

func (pst *Persister) calcOffset(page int, pageLimit int) int {
	offset := 0
	if pageLimit > 0 {
		if page < 1 {
			page = 1
		}
		offset = (page - 1) * pageLimit
	}
	return offset
}

// WhereSP find objects by expressions and sorting with paging
func (pst *Persister) WhereSP(model interface{}, sortexpr string, pageLimit int, page int, expr string, args ...interface{}) ( /*result*/ interface{}, error) {
	db, err := pst.getClient()
	if err != nil {
		return nil, err
	}

	offset := pst.calcOffset(page, pageLimit)

	if len(sortexpr) > 0 && pageLimit > 0 {
		// Sorting and paging
		if err := db.Offset(offset).Limit(pageLimit).Order(sortexpr).Where(expr, args...).Find(model).Error; err != nil {
			return nil, err
		}
	} else if len(sortexpr) > 0 {
		// Sorting
		if err := db.Order(sortexpr).Where(expr, args...).Find(model).Error; err != nil {
			return nil, err
		}
	} else if pageLimit > 0 {
		// Paging
		if err := db.Offset(offset).Limit(pageLimit).Where(expr, args...).Find(model).Error; err != nil {
			return nil, err
		}
	} else {
		// No Sorting, No Paging
		if err := db.Where(expr, args...).Find(model).Error; err != nil {
			return nil, err
		}
	}
	return model, nil
}

// WhereS find objects by expressions and sorting
func (pst *Persister) WhereS(model interface{}, sortexpr string, expr string, args ...interface{}) ( /*result*/ interface{}, error) {
	return pst.WhereSP(model, sortexpr, -1, -1, expr, args...)
}

// WhereP find objects by expression and paging
func (pst *Persister) WhereP(model interface{}, pageLimit int, page int, expr string, args ...interface{}) ( /*result*/ interface{}, error) {
	return pst.WhereSP(model, "", pageLimit, page, expr, args...)
}

// Where find objects by expressions
func (pst *Persister) Where(model interface{}, expr string, args ...interface{}) ( /*result*/ interface{}, error) {
	return pst.WhereSP(model, "", -1, -1, expr, args...)
}

// FindOne find object by id
func (pst *Persister) FindOne(model interface{}, idColumn string, id string) ( /*result*/ interface{}, error) {
	db, err := pst.getClient()
	if err != nil {
		return nil, err
	}

	where := fmt.Sprintf("%s = ?", idColumn)
	if err := db.Where(where, id).First(model).Error; err != nil {
		return nil, err
	}
	return model, nil
}

// Create create the object
func (pst *Persister) Create(model interface{}) error {
	db, err := pst.getClient()
	if err != nil {
		return err
	}

	err = db.Create(model).Error
	if err != nil {
		return err
	}

	return nil
}

// Update update the object
func (pst *Persister) Update(model interface{}) error {
	db, err := pst.getClient()
	if err != nil {
		return err
	}

	err = db.Save(model).Error
	if err != nil {
		return err
	}

	return nil
}

// CreateInBatch create the objects in batch
func (pst *Persister) CreateInBatch(models interface{}, bulkSize int) error {
	db, err := pst.getClient()
	if err != nil {
		return err
	}

	db.CreateInBatches(models, bulkSize)

	return nil
}
