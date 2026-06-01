package usecase

import (
	"context"

	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../interactor/mock/$GOFILE -package mock

// MetadataUsecase inspects table metadata: the list of tables, a table's
// header, and a table's records. Commands that report on tables without
// executing SQL depend on this interface.
type MetadataUsecase interface {
	// TablesName return all table name.
	TablesName(ctx context.Context) ([]*model.Table, error)
	// SchemaObjects returns every queryable table and view (including TEMP
	// tables and views) for enumeration by .tables.
	SchemaObjects(ctx context.Context) ([]*model.Table, error)
	// Header get table header name.
	Header(ctx context.Context, tableName string) (*model.Table, error)
	// List get records in the specified table
	List(ctx context.Context, tableName string) (*model.Table, error)
}
