package shell

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nao1215/filesql"
	"github.com/nao1215/sqly/domain/model"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

func isTextImportPath(path string) bool {
	compressionFactory := filesql.NewCompressionFactory()
	base := strings.ToLower(compressionFactory.RemoveCompressionExtension(path))
	switch filepath.Ext(base) {
	case model.ExtCSV, model.ExtTSV, model.ExtLTSV, model.ExtJSON, model.ExtJSONL:
		return true
	default:
		return false
	}
}

func (s *Shell) prepareImportLoadPath(path string) (string, func(), error) {
	if !isTextImportPath(path) || s.state.importEncoding == model.TextEncodingUTF8 {
		return path, nil, nil
	}

	compressionFactory := filesql.NewCompressionFactory()
	reader, cleanupReader, err := compressionFactory.CreateReaderForFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("open import reader for %s: %w", path, err)
	}

	dir, err := os.MkdirTemp("", "sqly-text-")
	if err != nil {
		_ = cleanupReader()
		return "", nil, fmt.Errorf("create temp dir for %s: %w", path, err)
	}

	readerClosed := false
	cleanup := func() {
		if !readerClosed {
			_ = cleanupReader()
			readerClosed = true
		}
		_ = os.RemoveAll(dir)
	}

	stagedPath := filepath.Join(dir, filepath.Base(compressionFactory.RemoveCompressionExtension(path)))
	file, err := os.Create(stagedPath) //nolint:gosec // stagedPath is under a sqly-created temp dir
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("create staging file for %s: %w", path, err)
	}

	_, copyErr := io.Copy(file, transform.NewReader(reader, newImportDecoder(s.state.importEncoding)))
	closeErr := file.Close()
	if !readerClosed {
		if err := cleanupReader(); err != nil {
			readerClosed = true
			cleanup()
			return "", nil, fmt.Errorf("close import reader for %s: %w", path, err)
		}
		readerClosed = true
	}
	if copyErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("decode %s as %s: %w", path, s.state.importEncoding, copyErr)
	}
	if closeErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("close staging file for %s: %w", path, closeErr)
	}

	return stagedPath, cleanup, nil
}

// encodingAdvice is the way out of an import that stopped because the file is
// not UTF-8, or "" for a failure that is about something else.
//
// The file used to load: SQLite stores TEXT as UTF-8, so bytes in a legacy
// encoding went in as they were and came back as mojibake with exit 0. filesql
// refuses them now, which is the right answer but not an actionable one on its
// own — the reader is told a byte is invalid, not that sqly can decode the file
// for them. Nothing here guesses which encoding it is: the bytes do not say, and
// a wrong guess would put the corruption back in a different shape.
//
// The encoding is fixed for the session at startup, so an .import typed into a
// running shell is told to restart with the flag rather than pointed at a
// command that does not exist.
func (s *Shell) encodingAdvice(err error) string {
	// A session that already named an encoding knows the flag; its failure is
	// about which encoding, and sqly has nothing to add about that.
	if s.state.importEncoding != model.TextEncodingUTF8 || !errors.Is(err, filesql.ErrInvalidUTF8) {
		return ""
	}
	if s.importingStartupInputs {
		return fmt.Sprintf("\nhint: this file is not UTF-8. If it is Shift-JIS, EUC-JP, or another legacy encoding,"+
			" load it with %s (one of: %s), e.g. %s shift-jis.",
			encodingFlag, model.TextEncodingHelp(), encodingFlag)
	}
	return fmt.Sprintf("\nhint: this file is not UTF-8. The encoding is chosen when sqly starts,"+
		" so restart with %s (one of: %s), e.g. sqly %s shift-jis FILE.",
		encodingFlag, model.TextEncodingHelp(), encodingFlag)
}

func newImportDecoder(enc model.TextEncoding) transform.Transformer {
	var fallback transform.Transformer
	switch enc {
	case model.TextEncodingShiftJIS:
		fallback = japanese.ShiftJIS.NewDecoder()
	case model.TextEncodingEUCJP:
		fallback = japanese.EUCJP.NewDecoder()
	case model.TextEncodingISO2022JP:
		fallback = japanese.ISO2022JP.NewDecoder()
	case model.TextEncodingUTF16LE:
		fallback = unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
	case model.TextEncodingUTF16BE:
		fallback = unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder()
	default:
		fallback = transform.Nop
	}
	return unicode.BOMOverride(fallback)
}
