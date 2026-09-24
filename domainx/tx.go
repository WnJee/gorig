package domainx

import (
	"context"
	"fmt"
	"strings"

	"github.com/WnJee/gorig/utils/errors"
	"gorm.io/gorm"
)

type txCtxKey struct {
	dbName string
}

// GetTxFromContext retrieves an active *gorm.DB transaction from context for the specified dbName if one exists.
func GetTxFromContext(ctx context.Context, dbName string) *gorm.DB {
	if ctx == nil {
		return nil
	}
	dbName = strings.ToLower(strings.TrimSpace(dbName))
	if dbName == "" {
		dbName = defaultDBName
	}
	val := ctx.Value(txCtxKey{dbName: dbName})
	if tx, ok := val.(*gorm.DB); ok && tx != nil {
		return tx
	}
	return nil
}

// WithTx binds a *gorm.DB transaction or connection to a context for a specific dbName.
func WithTx(ctx context.Context, dbName string, tx *gorm.DB) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	dbName = strings.ToLower(strings.TrimSpace(dbName))
	if dbName == "" {
		dbName = defaultDBName
	}
	return context.WithValue(ctx, txCtxKey{dbName: dbName}, tx)
}

// Transaction executes fn inside a database transaction for the default/specified MySQL database.
// If the context already contains an active transaction for the same database, it reuses it (nested/propagation support).
// If fn returns an error, the transaction is rolled back; otherwise it is committed.
func Transaction(ctx context.Context, fn func(txCtx context.Context) error, dbNames ...string) error {
	dbName := defaultDBName
	if len(dbNames) > 0 && strings.TrimSpace(dbNames[0]) != "" {
		dbName = dbNames[0]
	}
	dbName = strings.ToLower(strings.TrimSpace(dbName))

	if ctx == nil {
		ctx = context.Background()
	}

	// 1. Check if already in transaction for this database
	if existingTx := GetTxFromContext(ctx, dbName); existingTx != nil {
		return fn(ctx)
	}

	// 2. Obtain base DB connection
	db := UseDbConn(dbName)
	if db == nil {
		return errors.Sys(fmt.Sprintf("database connection not found for '%s'", dbName))
	}

	// 3. Begin GORM transaction
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := WithTx(ctx, dbName, tx)
		return fn(txCtx)
	})
}
