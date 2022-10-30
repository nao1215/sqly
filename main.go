package main

import (
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nao1215/sqly/infrastructure/persistence/csv"
	"github.com/nao1215/sqly/usecase"
)

// Version is sqly command version. Version value is assigned by LDFLAGS.
var Version string

func main() {
	csvInteractor := usecase.NewCSVInteractor(csv.NewCSVRepository())

	csv, err := csvInteractor.List("testdata/sample.csv")
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(csv.Header)
	for _, v := range csv.Records {
		fmt.Println(v)
	}
}
