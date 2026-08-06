package persistence

import (
	"bufio"
	"fmt"
	"io"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

// _ interface implementation check
var _ repository.LTSVRepository = (*ltsvRepository)(nil)

type ltsvRepository struct{}

// NewLTSVRepository return LTSVRepository
func NewLTSVRepository() repository.LTSVRepository {
	return &ltsvRepository{}
}

// Dump write contents of DB table to an LTSV writer. LTSV records are plain
// "label:value" tokens separated by tabs; it has no quoting, so a value
// containing a tab or newline cannot be represented losslessly and is rejected
// before writing. Writing each token directly (rather than through a CSV writer
// with a tab delimiter) keeps the output re-importable, since a CSV writer would
// quote the whole "label:value" token and break the label/value split.
func (lr *ltsvRepository) Dump(f io.Writer, table *model.Table) error {
	// Reject invalid labels and unrepresentable values before writing, so the
	// exported file stays a valid, round-trippable LTSV record set and a value
	// this cannot hold fails the export whole rather than part way through.
	if err := table.EnsureLTSVWritable(); err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for _, v := range table.Rows {
		for i := range v.Len() {
			data := v.At(i)
			label := table.ColumnName(i)
			if i > 0 {
				if err := w.WriteByte('\t'); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(w, "%s:%s", label, data); err != nil {
				return err
			}
		}
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
	}
	return w.Flush()
}
