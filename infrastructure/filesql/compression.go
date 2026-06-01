package filesql

import (
	"fmt"
	"io"

	libfilesql "github.com/nao1215/filesql"
	"github.com/nao1215/sqly/domain/model"
)

// compressionToLib maps a domain Compression to filesql's CompressionType.
// Bzip2 is intentionally unmapped: filesql has no bzip2 writer, and callers
// reject it before reaching here.
func compressionToLib(c model.Compression) (libfilesql.CompressionType, error) {
	switch c {
	case model.CompressionNone:
		return libfilesql.CompressionNone, nil
	case model.CompressionGzip:
		return libfilesql.CompressionGZ, nil
	case model.CompressionXz:
		return libfilesql.CompressionXZ, nil
	case model.CompressionZstd:
		return libfilesql.CompressionZSTD, nil
	case model.CompressionZlib:
		return libfilesql.CompressionZLIB, nil
	case model.CompressionSnappy:
		return libfilesql.CompressionSNAPPY, nil
	case model.CompressionS2:
		return libfilesql.CompressionS2, nil
	case model.CompressionLz4:
		return libfilesql.CompressionLZ4, nil
	default:
		return libfilesql.CompressionNone, fmt.Errorf("unsupported compression: %v", c)
	}
}

// NewDecompressingReaderForFile opens path and returns a reader that
// transparently decompresses it when the extension names a known codec (.gz, .xz,
// .zst, ...), plus a cleanup that closes the underlying file and decoder. For an
// uncompressed file it returns a plain file reader. It reuses filesql's codecs so
// sqly does not depend on the compression libraries directly, and lets the adapter
// inspect compressed JSON/JSONL inputs (e.g. an empty "[]") the same way it
// inspects uncompressed ones. Ref #452, #453.
func NewDecompressingReaderForFile(path string) (io.Reader, func() error, error) {
	return libfilesql.NewCompressionFactory().CreateReaderForFile(path)
}

// NewCompressingWriter wraps w with the codec for c, reusing filesql's
// compression writers so sqly does not depend on the codec libraries directly.
// The returned close function flushes and finalizes the codec and must be called
// before the underlying destination is closed; the caller still closes that
// destination. For CompressionNone it returns w unchanged with a no-op close.
func NewCompressingWriter(w io.Writer, c model.Compression) (io.Writer, func() error, error) {
	ct, err := compressionToLib(c)
	if err != nil {
		return nil, nil, err
	}
	return libfilesql.NewCompressionHandler(ct).CreateWriter(w)
}
