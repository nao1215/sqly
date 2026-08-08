// Package usecase defines interfaces for handling different file formats and operations.
// It follows clean architecture principles to separate business logic from implementation details.
package usecase

import (
	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../interactor/mock/$GOFILE -package mock

// ExportUsecase handles exporting table data to files in various formats.
type ExportUsecase interface {
	// DumpTable exports a table to a file in the specified format, optionally
	// wrapping text and JSON output in a compression codec. Pass
	// model.CompressionNone to write uncompressed.
	//
	// encoding names the text encoding of the output. Pass
	// model.TextEncodingUTF8 for the ordinary case; a write-back passes the
	// encoding its source was read with, so the file it rewrites stays readable
	// the way the caller has been reading it. Excel and Parquet carry their own
	// encoding and ignore it.
	DumpTable(filePath string, table *model.Table, format model.ExportFormat, compression model.Compression, encoding model.TextEncoding) error
}
