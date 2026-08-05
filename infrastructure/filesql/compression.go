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
