<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-1-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->
  
![Coverage](https://raw.githubusercontent.com/nao1215/octocovs-central-repo/main/badges/nao1215/sqly/coverage.svg)
[![Build](https://github.com/nao1215/sqly/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/sqly/actions/workflows/build.yml)
[![reviewdog](https://github.com/nao1215/sqly/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/sqly/actions/workflows/reviewdog.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nao1215/sqly)](https://goreportcard.com/report/github.com/nao1215/sqly)
![GitHub](https://img.shields.io/github/license/nao1215/sqly)  
![demo](../img/demo.gif)  

[English](../../README.md) | [Русский](../ru/README.md) | [中文](../zh-cn/README.md) | [한국어](../ko/README.md) | [Español](../es/README.md) | [Français](../fr/README.md)

**sqly** は、CSV、TSV、LTSV、JSON、さらにはMicrosoft Excel™ファイルに対してSQLを実行できる強力なコマンドラインツールです。sqlyはこれらのファイルを[SQLite3](https://www.sqlite.org/index.html)のインメモリデータベースにインポートします。

sqlyには **sqly-shell** があります。SQLの補完とコマンド履歴を使って、対話的にSQLを実行できます。もちろん、sqly-shellを実行せずにSQLを実行することも可能です。

- ユーザー・開発者向け公式ドキュメント: [https://nao1215.github.io/sqly/](https://nao1215.github.io/sqly/)
- 同じ開発者が作成した代替ツール: [DBMS・ローカルCSV/TSV/LTSV用のシンプルなターミナルUI](https://github.com/nao1215/sqluv)

> [!WARNING]
> JSONサポートには制限があります。将来的にJSONサポートを中止する可能性があります。

## インストール方法
### "go install"を使用
```shell
go install github.com/nao1215/sqly@latest
```

### homebrewを使用
```shell
brew install nao1215/tap/sqly
```

## サポートOS・goバージョン
- Windows
- macOS
- Linux
- go1.24.0以降

## 使用方法
sqlyは、ファイルパスを引数として渡すと、CSV/TSV/LTSV/JSON/Excelファイルを自動的にDBにインポートします。DBテーブル名は、ファイル名またはシート名と同じになります（例：user.csvをインポートした場合、sqlyコマンドはuserテーブルを作成します）。

sqlyはファイル拡張子からファイル形式を自動判定します。

### ターミナルでのSQL実行: --sqlオプション
--sqlオプションは、SQL文をオプション引数として受け取ります。

```shell
$ sqly --sql "SELECT user_name, position FROM user INNER JOIN identifier ON user.identifier = identifier.id" testdata/user.csv testdata/identifier.csv 
+-----------+-----------+
| user_name | position  |
+-----------+-----------+
| booker12  | developrt |
| jenkins46 | manager   |
| smith79   | neet      |
+-----------+-----------+
```

### 出力形式の変更
sqlyは、SQLクエリの結果を以下の形式で出力します：
- ASCII表形式（デフォルト）
- CSV形式（--csvオプション）
- TSV形式（--tsvオプション）
- LTSV形式（--ltsvオプション）
- JSON形式（--jsonオプション）

```shell
$ sqly --sql "SELECT * FROM user LIMIT 2" --csv testdata/user.csv 
user_name,identifier,first_name,last_name
booker12,1,Rachel,Booker
jenkins46,2,Mary,Jenkins

$ sqly --sql "SELECT * FROM user LIMIT 2" --json testdata/user.csv 
[
   {
      "first_name": "Rachel",
      "identifier": "1",
      "last_name": "Booker",
      "user_name": "booker12"
   },
   {
      "first_name": "Mary",
      "identifier": "2",
      "last_name": "Jenkins",
      "user_name": "jenkins46"
   }
]

$ sqly --sql "SELECT * FROM user LIMIT 2" --json testdata/user.csv > user.json

$ sqly --sql "SELECT * FROM user LIMIT 2" --csv user.json 
first_name,identifier,last_name,user_name
Rachel,1,Booker,booker12
Mary,2,Jenkins,jenkins46
```

### sqlyシェルの実行
--sqlオプションなしでsqlyコマンドを実行すると、sqlyシェルが開始されます。ファイルパスと共にsqlyコマンドを実行した場合、ファイルをSQLite3インメモリデータベースにインポートした後、sqly-shellが開始されます。

```shell
$ sqly 
sqly v0.10.0

"SQLクエリ" または "ドットで始まるsqlyコマンド" を入力してください。
使用方法は.help、終了は.exitです。

sqly:~/github/github.com/nao1215/sqly(table)$ 
```
  
sqlyシェルは、一般的なSQLクライアント（`sqlite3`コマンドや`mysql`コマンドなど）と同様に機能します。sqlyシェルには、ドットで始まるヘルパーコマンドがあります。sqly-shellは、コマンド履歴と入力補完もサポートしています。

sqly-shellには以下のヘルパーコマンドがあります：

```shell
sqly:~/github/github.com/nao1215/sqly(table)$ .help
        .cd: ディレクトリ変更
      .dump: 出力モードに応じた形式でDBテーブルをファイルにダンプ（デフォルト：csv）
      .exit: sqly終了
    .header: テーブルヘッダー印刷
      .help: ヘルプメッセージ印刷
    .import: ファイルインポート
        .ls: ディレクトリ内容印刷
      .mode: 出力モード変更
       .pwd: 現在の作業ディレクトリ印刷
    .tables: テーブル印刷
```

### SQL結果のファイル出力
#### Linuxユーザー向け
sqlyは、シェルリダイレクションを使用してSQL実行結果をファイルに保存できます。--csvオプションを使用すると、SQL実行結果をテーブル形式ではなくCSV形式で出力します。

```shell
$ sqly --sql "SELECT * FROM user" --csv testdata/user.csv > test.csv
```

#### Windowsユーザー向け

sqlyは--outputオプションを使用してSQL実行結果をファイルに保存できます。--outputオプションは、--sqlオプションで指定されたSQL結果の出力先パスを指定します。

```shell
$ sqly --sql "SELECT * FROM user" --output=test.csv testdata/user.csv 
```

### sqly-shellのキーバインディング
|キーバインディング	|説明|
|:--|:--|
|Ctrl + A	|行の先頭へ移動（Home）|
|Ctrl + E	|行の末尾へ移動（End）|
|Ctrl + P	|前のコマンド（上矢印）|
|Ctrl + N	|次のコマンド（下矢印）|
|Ctrl + F	|一文字前進|
|Ctrl + B	|一文字後退|
|Ctrl + D	|カーソル下の文字を削除|
|Ctrl + H	|カーソル前の文字を削除（Backspace）|
|Ctrl + W	|カーソル前の単語をクリップボードに切り取り|
|Ctrl + K	|カーソル後の行をクリップボードに切り取り|
|Ctrl + U	|カーソル前の行をクリップボードに切り取り|
|Ctrl + L	|画面クリア|  
|TAB        |補完|
|↑          |前のコマンド|
|↓          |次のコマンド|

## ベンチマーク
CPU: AMD Ryzen 5 3400G with Radeon Vega Graphics  
実行: 
```sql
SELECT * FROM `table` WHERE `Index` BETWEEN 1000 AND 2000 ORDER BY `Index` DESC LIMIT 1000
```

|レコード数  | カラム数 | 操作あたりの時間 | 操作あたりのメモリ割り当て | 操作あたりの割り当て回数 |
|---------|----|-------------------|--------------------------------|---------------------------|
|100,000|   12|  1715818835 ns/op  |      441387928 B/op   |4967183 allocs/op | 
|1,000,000|   9|   11414332112 ns/op |      2767580080 B/op | 39131122 allocs/op |


## 代替ツール
|名前| 説明|
|:--|:--|
|[harelba/q](https://github.com/harelba/q)|区切り文字ファイルとマルチファイルsqliteデータベースに対してSQLを直接実行|
|[dinedal/textql](https://github.com/dinedal/textql)|CSVやTSVなどの構造化テキストに対してSQLを実行|
|[noborus/trdsql](https://github.com/noborus/trdsql)|CSV、LTSV、JSON、YAML、TBLNに対してSQLクエリを実行できるCLIツール。さまざまな形式で出力可能。|
|[mithrandie/csvq](https://github.com/mithrandie/csvq)|CSV用のSQL風クエリ言語|


## 制限事項（サポートしない機能）

- CREATE等のDDL
- GRANT等のDML  
- トランザクション等のTCL

## コントリビュート

時間を割いてコントリビュートしていただき、ありがとうございます！詳細については[CONTRIBUTING.md](../../CONTRIBUTING.md)をご覧ください。コントリビュートは開発だけでなく、例えばGitHubスターも開発のモチベーションになります！

[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/sqly&type=Date)](https://star-history.com/#nao1215/sqly&Date)

## 開発方法

[ドキュメント](https://nao1215.github.io/sqly/)の「開発者向けドキュメント」セクションをご覧ください。

新機能の追加やバグ修正時は、単体テストを書いてください。sqlyは、以下の単体テストツリーマップが示すように、すべてのパッケージで単体テストが実施されています。

![treemap](../img/cover-tree.svg)


### 連絡先
「バグを見つけた」や「追加機能のリクエスト」などのコメントを開発者に送りたい場合は、以下の連絡先のいずれかを使用してください。

- [GitHub Issue](https://github.com/nao1215/sqly/issues)

## ライセンス
sqlyプロジェクトは[MIT LICENSE](../../LICENSE)の条項の下でライセンスされています。

## コントリビューター ✨

これらの素晴らしい人々に感謝します（[絵文字キー](https://allcontributors.org/docs/en/emoji-key)）：

<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<!-- prettier-ignore-start -->
<!-- markdownlint-disable -->
<table>
  <tbody>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://debimate.jp/"><img src="https://avatars.githubusercontent.com/u/22737008?v=4?s=75" width="75px;" alt="CHIKAMATSU Naohiro"/><br /><sub><b>CHIKAMATSU Naohiro</b></sub></a><br /><a href="https://github.com/nao1215/sqly/commits?author=nao1215" title="Code">💻</a> <a href="https://github.com/nao1215/sqly/commits?author=nao1215" title="Documentation">📖</a></td>
    </tr>
  </tbody>
  <tfoot>
    <tr>
      <td align="center" size="13px" colspan="7">
        <img src="https://raw.githubusercontent.com/all-contributors/all-contributors-cli/1b8533af435da9854653492b1327a23a4dbd0a10/assets/logo-small.svg">
          <a href="https://all-contributors.js.org/docs/en/bot/usage">コントリビューションを追加</a>
        </img>
      </td>
    </tr>
  </tfoot>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

このプロジェクトは[all-contributors](https://github.com/all-contributors/all-contributors)仕様に従います。どんな種類のコントリビューションも歓迎します！