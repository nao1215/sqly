package persistence

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/infrastructure"
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
// quote the whole "label:value" token and break the label/value split. Ref #383.
func (lr *ltsvRepository) Dump(f io.Writer, table *model.Table) error {
	w := bufio.NewWriter(f)
	for _, v := range table.Records() {
		for i, data := range v {
			label := table.Header()[i]
			if strings.ContainsAny(label, "\t\n\r") {
				return fmt.Errorf("ltsv: column name %q contains a tab or newline, which LTSV cannot represent", label)
			}
			if strings.ContainsAny(data, "\t\n\r") {
				return fmt.Errorf("ltsv: value for column %q contains a tab or newline, which LTSV cannot represent; use csv/tsv/json for such values", label)
			}
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

// labelAndData split label and data.
func (lr *ltsvRepository) labelAndData(s string) (string, string, error) {
	idx := strings.Index(s, ":")
	if idx == -1 || idx == 0 {
		return "", "", infrastructure.ErrNoLabel
	}
	return s[:idx], s[idx+1:], nil
}
