package persistence

import (
	"os"
	"testing"

	"github.com/nao1215/sqly/config"
)

func TestMain(m *testing.M) {
	config.InitSQLite3()
	os.Exit(m.Run())
}
