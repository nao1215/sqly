<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-1-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->
  
![Coverage](https://raw.githubusercontent.com/nao1215/octocovs-central-repo/main/badges/nao1215/sqly/coverage.svg)
[![Build](https://github.com/nao1215/sqly/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/sqly/actions/workflows/build.yml)
[![reviewdog](https://github.com/nao1215/sqly/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/sqly/actions/workflows/reviewdog.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nao1215/sqly)](https://goreportcard.com/report/github.com/nao1215/sqly)
![GitHub](https://img.shields.io/github/license/nao1215/sqly)  
![demo](../img/demo.gif)  

[English](../../README.md) | [日本語](../ja/README.md) | [Русский](../ru/README.md) | [한국어](../ko/README.md) | [Español](../es/README.md) | [Français](../fr/README.md)


**sqly** 是一个强大的命令行工具，可以对CSV、TSV、LTSV和Microsoft Excel™文件执行SQL查询。sqly将这些文件导入[SQLite3](https://www.sqlite.org/index.html)内存数据库。

sqly拥有 **sqly-shell**。您可以通过SQL自动完成和命令历史记录交互式执行SQL。当然，您也可以在不运行sqly-shell的情况下执行SQL。

```shell
# 对压缩文件也能工作！
sqly --sql "SELECT * FROM data" data.csv.gz
sqly --sql "SELECT * FROM logs WHERE level='ERROR'" logs.tsv.bz2
```

## 如何安装
### 使用"go install"
```shell
go install github.com/nao1215/sqly@latest
```

### 使用homebrew
```shell
brew install nao1215/tap/sqly
```

## 支持的操作系统和go版本
- Windows
- macOS
- Linux
- go1.24.0或更高版本

## 使用方法
当您将文件路径作为参数传递时，sqly会自动将CSV/TSV/LTSV/Excel文件（包括压缩版本）导入数据库。数据库表名与文件名或工作表名相同（例如，如果导入user.csv，sqly命令将创建user表）。

**注意**：如果文件名包含可能导致SQL语法错误的字符（如连字符 `-`、点号 `.` 或其他特殊字符），它们会自动替换为下划线 `_`。例如，`bug-syntax-error.csv` 会变成表 `bug_syntax_error`。

sqly根据文件扩展名自动确定文件格式，包括压缩文件。

### 在终端中执行SQL：--sql选项
--sql选项接受SQL语句作为可选参数。

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

### 更改输出格式
sqly以以下格式输出SQL查询结果：
- ASCII表格式（默认）
- CSV格式（--csv选项）
- TSV格式（--tsv选项）
- LTSV格式（--ltsv选项）

```shell
$ sqly --sql "SELECT * FROM user LIMIT 2" --csv testdata/user.csv 
user_name,identifier,first_name,last_name
booker12,1,Rachel,Booker
jenkins46,2,Mary,Jenkins
```

### 运行sqly shell
在没有--sql选项的情况下运行sqly命令时，sqly shell会启动。当您使用文件路径执行sqly命令时，在将文件导入SQLite3内存数据库后，sqly-shell将启动。

```shell
$ sqly 
sqly v0.10.0

输入"SQL查询"或"以点开头的sqly命令"。
.help 打印使用方法，.exit 退出sqly。

sqly:~/github/github.com/nao1215/sqly(table)$ 
```
  
sqly shell的功能类似于常见的SQL客户端（例如`sqlite3`命令或`mysql`命令）。sqly shell有以点开头的帮助命令。sqly-shell还支持命令历史记录和输入自动完成。

sqly-shell具有以下帮助命令：

```shell
sqly:~/github/github.com/nao1215/sqly(table)$ .help
        .cd: 更改目录
      .dump: 根据输出模式将数据库表转储到文件（默认：csv）
      .exit: 退出sqly
    .header: 打印表头
      .help: 打印帮助消息
    .import: 导入文件
        .ls: 打印目录内容
      .mode: 更改输出模式
       .pwd: 打印当前工作目录
    .tables: 打印表
```

### 将SQL结果输出到文件
#### 对于Linux用户
sqly可以使用shell重定向将SQL执行结果保存到文件。--csv选项以CSV格式输出SQL执行结果，而不是表格格式。

```shell
$ sqly --sql "SELECT * FROM user" --csv testdata/user.csv > test.csv
```

#### 对于Windows用户

sqly可以使用--output选项将SQL执行结果保存到文件。--output选项指定--sql选项中指定的SQL结果的目标路径。

```shell
$ sqly --sql "SELECT * FROM user" --output=test.csv testdata/user.csv 
```

### sqly-shell的键绑定
|键绑定	|描述|
|:--|:--|
|Ctrl + A	|转到行首（Home）|
|Ctrl + E	|转到行尾（End）|
|Ctrl + P	|上一个命令（向上箭头）|
|Ctrl + N	|下一个命令（向下箭头）|
|Ctrl + F	|前进一个字符|
|Ctrl + B	|后退一个字符|
|Ctrl + D	|删除光标下的字符|
|Ctrl + H	|删除光标前的字符（Backspace）|
|Ctrl + W	|将光标前的单词剪切到剪贴板|
|Ctrl + K	|将光标后的行剪切到剪贴板|
|Ctrl + U	|将光标前的行剪切到剪贴板|
|Ctrl + L	|清除屏幕|  
|TAB        |自动完成|
|↑          |上一个命令|
|↓          |下一个命令|

## 📋 最近的变更


- 用户和开发者官方文档：[https://nao1215.github.io/sqly/](https://nao1215.github.io/sqly/)
- 同一开发者创建的替代工具：[DBMS和本地CSV/TSV/LTSV的简单终端UI](https://github.com/nao1215/sqluv)

### 新功能：压缩文件支持

**sqly** 现在支持压缩文件！您可以直接处理：
- **Gzip** 压缩文件 (`.csv.gz`、`.tsv.gz`、`.ltsv.gz`、`.xlsx.gz`)
- **Bzip2** 压缩文件 (`.csv.bz2`、`.tsv.bz2`、`.ltsv.bz2`、`.xlsx.bz2`)
- **XZ** 压缩文件 (`.csv.xz`、`.tsv.xz`、`.ltsv.xz`、`.xlsx.xz`)
- **Zstandard** 压缩文件 (`.csv.zst`、`.tsv.zst`、`.ltsv.zst`、`.xlsx.zst`)


### 新增功能
- **CTE（公用表表达式）支持**：现在支持 WITH 子句进行复杂查询和递归操作
- **filesql 集成**：使用 [filesql](https://github.com/nao1215/filesql) 库提高性能和功能
- **性能改进**：通过事务批处理进行批量插入操作，以实现更快的文件处理
- **更好的类型处理**：自动类型检测确保正确的数值排序和计算
- **压缩文件支持**：原生支持 `.gz`、`.bz2`、`.xz` 和 `.zst` 压缩文件

### 移除功能
- **JSON 支持**：为了专注于结构化数据格式（CSV、TSV、LTSV、Excel），JSON 文件格式支持已被移除
  - 如果您需要使用 sqly 处理 JSON 数据，请使用 JSON 工具的 CSV 导出功能
  - 此移除允许对核心文件格式进行更好的优化

### 破坏性变更
- `--json` 标志已被移除
- JSON 文件（`.json`）不再作为输入支持
- 由于改进了类型检测，输出中的数值格式可能会略有不同

## 基准测试
CPU: AMD Ryzen 5 3400G with Radeon Vega Graphics  
执行: 
```sql
SELECT * FROM `table` WHERE `Index` BETWEEN 1000 AND 2000 ORDER BY `Index` DESC LIMIT 1000
```

|记录数  | 列数 | 每次操作时间 | 每次操作内存分配 | 每次操作分配次数 |
|---------|----|-------------------|--------------------------------|---------------------------|
|100,000|   12|  1715818835 ns/op  |      441387928 B/op   |4967183 allocs/op | 
|1,000,000|   9|   11414332112 ns/op |      2767580080 B/op | 39131122 allocs/op |


## 替代工具
|名称| 描述|
|:--|:--|
|[harelba/q](https://github.com/harelba/q)|直接对分隔文件和多文件sqlite数据库运行SQL|
|[dinedal/textql](https://github.com/dinedal/textql)|对结构化文本（如CSV或TSV）执行SQL|
|[noborus/trdsql](https://github.com/noborus/trdsql)|可以对CSV、LTSV、JSON、YAML和TBLN执行SQL查询的CLI工具。可输出到各种格式。|
|[mithrandie/csvq](https://github.com/mithrandie/csvq)|CSV的类SQL查询语言|


## 限制（不支持）

- CREATE等DDL
- GRANT等DML
- 事务等TCL

## 贡献

首先，感谢您花时间贡献！有关更多信息，请参阅[CONTRIBUTING.md](../../CONTRIBUTING.md)。贡献不仅与开发相关。例如，GitHub Star激励我开发！

[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/sqly&type=Date)](https://star-history.com/#nao1215/sqly&Date)

## 如何开发

请参阅[文档](https://nao1215.github.io/sqly/)的"开发者文档"部分。

添加新功能或修复错误时，请编写单元测试。sqly对所有包都进行单元测试，如下面的单元测试树状图所示。

![treemap](../img/cover-tree.svg)


### 联系方式
如果您想向开发者发送"发现错误"或"请求附加功能"等评论，请使用以下联系方式之一。

- [GitHub Issue](https://github.com/nao1215/sqly/issues)

## 使用的库

**sqly** 利用强大的 Go 库来提供其功能：
- [filesql](https://github.com/nao1215/filesql) - 为 CSV/TSV/LTSV/Excel 文件提供 SQL 数据库接口，具有自动类型检测和压缩文件支持
- [prompt](https://github.com/nao1215/prompt) - 为交互式 shell 提供 SQL 自动完成和命令历史功能

## 许可证
sqly项目根据[MIT LICENSE](../../LICENSE)条款许可。

## 贡献者 ✨

感谢这些优秀的人们（[表情符号键](https://allcontributors.org/docs/en/emoji-key)）：

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
          <a href="https://all-contributors.js.org/docs/en/bot/usage">添加您的贡献</a>
        </img>
      </td>
    </tr>
  </tfoot>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

该项目遵循[all-contributors](https://github.com/all-contributors/all-contributors)规范。欢迎任何形式的贡献！