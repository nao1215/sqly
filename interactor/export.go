package interactor

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/nao1215/sqly/domain/cleanup"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/infrastructure/persistence"
	"github.com/nao1215/sqly/usecase"
)

// _ interface implementation check
var _ usecase.ExportUsecase = (*exportInteractor)(nil)

// exportInteractor writes a table out in any format sqly exports.
//
// It holds nothing. Each format used to arrive as an interface with one
// implementation and one method, injected here through a five-argument
// constructor; nothing ever substituted one, so what the indirection bought was
// four files to change per format rather than one line.
type exportInteractor struct{}

// NewExportInteractor returns an ExportUsecase that can dump tables in any supported format.
func NewExportInteractor() usecase.ExportUsecase {
	return &exportInteractor{}
}

// streamSerializer writes a table to a stream, so its output can be wrapped in a
// compression codec on the way to the file.
type streamSerializer func(io.Writer, *model.Table) error

// pathSerializer writes a table to a path it opens itself. Excel and Parquet
// both build their container in memory and save it, so neither has a stream a
// codec could wrap; callers reject compression for them upstream.
type pathSerializer func(string, *model.Table) error

// streamSerializers names the writer for every format that has one. Formats
// whose file output is their display rendering (Markdown, JSON, JSONL) print
// themselves rather than carrying a second implementation of the same bytes.
//
// A registry rather than a switch so a test can assert that every declared
// ExportFormat resolves to a writer: a format added to the model and forgotten
// here used to fall through to the default branch and be written as CSV, under
// whatever extension was asked for.
var streamSerializers = map[model.ExportFormat]streamSerializer{
	model.ExportCSV:      persistence.DumpCSV,
	model.ExportTSV:      persistence.DumpTSV,
	model.ExportLTSV:     persistence.DumpLTSV,
	model.ExportMarkdown: printSerializer(model.PrintModeMarkdownTable),
	model.ExportJSON:     printSerializer(model.PrintModeJSON),
	model.ExportJSONL:    printSerializer(model.PrintModeJSONL),
}

// pathSerializers names the writer for every format that opens its own file.
var pathSerializers = map[model.ExportFormat]pathSerializer{
	model.ExportExcel:   persistence.DumpExcel,
	model.ExportParquet: filesql.DumpTableToParquet,
}

// printSerializer adapts a display mode into a serializer, for the formats whose
// file output matches what the screen shows.
func printSerializer(mode model.PrintMode) streamSerializer {
	return func(w io.Writer, table *model.Table) error {
		return table.Print(w, mode)
	}
}

// DumpTable exports a table to a file in the specified format. Text and JSON
// formats honor the compression codec and the text encoding; Excel and Parquet
// are binary container formats that state their own encoding and ignore both
// (callers reject compression for them upstream).
func (e *exportInteractor) DumpTable(filePath string, table *model.Table, format model.ExportFormat, compression model.Compression, encoding model.TextEncoding) error {
	if dump, ok := pathSerializers[format]; ok {
		return dump(filepath.Clean(filePath), table)
	}
	dump, ok := streamSerializers[format]
	if !ok {
		// Unreachable from a parsed format: both parsers reject a name they do
		// not know. Reported rather than silently written as CSV, which is what
		// the switch this replaced did with a format nobody had wired up.
		return fmt.Errorf("no serializer for export format %q", format)
	}
	return e.withCompressedWriter(filePath, compression, func(w io.Writer) error {
		// The encoder sits inside the compressor, because what a codec stores is
		// the encoded text: a reader decompresses and then decodes, which is the
		// order the import side already applies.
		encoded, finish := persistence.NewEncodingWriter(w, encoding)
		if err := dump(encoded, table); err != nil {
			return err
		}
		return finish()
	})
}

// withCompressedWriter creates filePath, optionally wraps it in a compression
// codec, and passes the resulting writer to write. The codec is finalized before
// the file is closed (deferred close runs in reverse order), so all buffered
// compressed bytes reach disk.
//
// The bytes are written once, straight to filePath. Every caller already hands
// this a scratch path rather than the user's destination — .dump and --output
// through writeFileAtomically, .save through its own staging — and that is where
// "the previous file survives a failed export" is decided. Serializing into a
// second temporary file first, then copying it across, wrote every exported byte
// twice and made the export depend on free space in the OS temp directory as
// well as in the destination's own filesystem.
func (e *exportInteractor) withCompressedWriter(filePath string, compression model.Compression, write func(io.Writer) error) (err error) {
	file, err := persistence.CreateExportFile(filePath)
	if err != nil {
		return fmt.Errorf("create output file %q: %w", filePath, err)
	}
	defer func() {
		err = cleanup.Join(err, file.Close(), fmt.Sprintf("close output file %q", filePath))
	}()

	w, closeComp, err := filesql.NewCompressingWriter(file, compression)
	if err != nil {
		return fmt.Errorf("init compression for %q: %w", filePath, err)
	}

	compClosed := false
	finalizeComp := func() error {
		if compClosed {
			return nil
		}
		compClosed = true
		return closeComp()
	}

	defer func() {
		err = cleanup.Join(err, finalizeComp(), fmt.Sprintf("finalize compression for %q", filePath))
	}()

	if err = write(w); err != nil {
		return fmt.Errorf("dump to %q: %w", filePath, err)
	}

	// Finalize the codec here rather than leaving it to the deferred call, so a
	// codec that fails while flushing its tail is reported as the export failing
	// instead of as a cleanup note on a success.
	if err = finalizeComp(); err != nil {
		return fmt.Errorf("finalize compression for %q: %w", filePath, err)
	}

	return nil
}
