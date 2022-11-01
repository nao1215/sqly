package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mattn/go-tty"
	"github.com/nao1215/sqly/infrastructure/persistence/csv"
	"github.com/nao1215/sqly/infrastructure/persistence/sqlite3"
	"github.com/nao1215/sqly/usecase"
)

var (
	// Version is sqly command version. Version value is assigned by LDFLAGS.
	Version string
	// Stdout is new instance of Writer which handles escape sequence for stdout.
	Stdout = colorable.NewColorableStdout()
	// Stderr is new instance of Writer which handles escape sequence for stderr.
	Stderr = colorable.NewColorableStderr()
)

func main() {
	os.Remove("./foo.db")
	db, err := sql.Open("sqlite3", ":memory:")
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

	printWelcomeMessage()
	interactive()

	if err := sqlite3Inreractor.Exec("SELECT 配達区分 FROM sample;"); err != nil {
		log.Fatal(err)
	}

	fmt.Println("success")
}

func printWelcomeMessage() {
	fmt.Fprintf(Stdout, "%s %s\n", color.GreenString("sqly"), Version)
	fmt.Fprintf(Stdout, "enter %s for usage hints.\n", color.CyanString("\".help\""))
}

func interactive() {
	tty, err := tty.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer tty.Close()

	fmt.Fprintf(Stdout, "%s>>", color.GreenString("sqly"))
	input := ""
	for {
		r, err := tty.ReadRune()
		if err != nil {
			log.Fatal(err)
		}

		// Enter押下したかの判定と補完処理を追加する
		input += string(r)
		fmt.Fprintf(Stdout, "\r%s>>%s", color.GreenString("sqly"), input)
	}
}
