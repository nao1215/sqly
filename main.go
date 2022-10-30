package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nao1215/sqly/infrastructure/persistence/csv"
	"github.com/nao1215/sqly/infrastructure/persistence/sqlite3"
	"github.com/nao1215/sqly/usecase"
)

// Version is sqly command version. Version value is assigned by LDFLAGS.
var Version string

func main() {
	os.Remove("./foo.db")
	db, err := sql.Open("sqlite3", "foo.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	csvInteractor := usecase.NewCSVInteractor(csv.NewCSVRepository())
	sqlite3Inreractor := usecase.NewSQLite3Interactor(sqlite3.NewSQLite3Repository(db))

	csv, err := csvInteractor.List("testdata/sample.csv")
	if err != nil {
		log.Fatal(err)
	}
	table := csv.ToTable()

	if err := sqlite3Inreractor.CreateTable(table); err != nil {
		log.Fatal(err)
	}

	if err := sqlite3Inreractor.Insert(table); err != nil {
		log.Fatal(err)
	}
	fmt.Println("success")
}
