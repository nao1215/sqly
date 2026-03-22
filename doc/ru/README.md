<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-1-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->
  
![Coverage](https://raw.githubusercontent.com/nao1215/octocovs-central-repo/main/badges/nao1215/sqly/coverage.svg)
[![Build](https://github.com/nao1215/sqly/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/sqly/actions/workflows/build.yml)
[![reviewdog](https://github.com/nao1215/sqly/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/sqly/actions/workflows/reviewdog.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nao1215/sqly)](https://goreportcard.com/report/github.com/nao1215/sqly)
![GitHub](https://img.shields.io/github/license/nao1215/sqly)  
![demo](../img/demo.gif)  

[English](../../README.md) | [日本語](../ja/README.md) | [中文](../zh-cn/README.md) | [한국어](../ko/README.md) | [Español](../es/README.md) | [Français](../fr/README.md)

**sqly** - мощный инструмент командной строки, который может выполнять SQL-запросы к CSV, TSV, LTSV и Microsoft Excel™ файлам. sqly импортирует эти файлы в базу данных [SQLite3](https://www.sqlite.org/index.html) в памяти.

sqly имеет **sqly-shell**. Вы можете интерактивно выполнять SQL с автодополнением SQL и историей команд. Конечно, вы также можете выполнять SQL, не запуская sqly-shell.

```shell
# Работает со сжатыми файлами!
sqly --sql "SELECT * FROM data" data.csv.gz
sqly --sql "SELECT * FROM logs WHERE level='ERROR'" logs.tsv.bz2
```

## Как установить
### Использование "go install"
```shell
go install github.com/nao1215/sqly@latest
```

### Использование homebrew
```shell
brew install nao1215/tap/sqly
```

## Поддерживаемые ОС и версии Go
- Windows
- macOS
- Linux
- go1.25.0 или позднее

## Как использовать
sqly автоматически импортирует CSV/TSV/LTSV/JSON/JSONL/Parquet/Excel файлы (включая сжатые версии) в базу данных, когда вы передаете пути к файлам или директориям в качестве аргументов. Вы также можете смешивать файлы и директории в одной команде. Имя таблицы в базе данных совпадает с именем файла или листа (например, если вы импортируете user.csv, команда sqly создает таблицу user).

**Примечание**: Если имя файла содержит символы, которые могут вызвать синтаксические ошибки SQL (такие как дефисы `-`, точки `.` или другие специальные символы), они автоматически заменяются на подчеркивания `_`. Например, `bug-syntax-error.csv` становится таблицей `bug_syntax_error`.

### Имена листов Excel
При импорте файлов Excel имена таблиц создаются в формате `имяфайла_имялиста`. Имена листов также обрабатываются для совместимости с SQL:
- Пробелы, дефисы и точки заменяются подчеркиваниями
- Символы не-ASCII (такие как акцентированные буквы `é`) удаляются

Примеры:
- Файл `data.xlsx` с листом `A test` → таблица `data_A_test`
- Файл `report.xlsx` с листом `Café` → таблица `report_Caf`

Вы можете указать имя листа с помощью опции `--sheet`, используя оригинальное имя (до обработки):
```shell
$ sqly data.xlsx --sheet="A test"
$ sqly report.xlsx --sheet="Café"
```

sqly автоматически определяет формат файла по расширению, включая сжатые файлы.

### Выполнение SQL в терминале: опция --sql
Опция --sql принимает SQL-выражение в качестве необязательного аргумента.

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

### Импорт директорий
Вы можете импортировать целые директории, содержащие поддерживаемые файлы. sqly автоматически обнаруживает все CSV, TSV, LTSV и Excel файлы (включая сжатые версии) в директории и импортирует их:

```shell
# Импорт всех файлов из директории
$ sqly ./data_directory

# Смешивание файлов и директорий
$ sqly file1.csv ./data_directory file2.tsv

# Использование с опцией --sql
$ sqly ./data_directory --sql "SELECT * FROM users"
```

### Интерактивная оболочка: команда .import
В sqly shell вы можете использовать команду `.import` для импорта файлов или директорий:

```shell
sqly:~/data$ .import ./csv_files
Successfully imported 3 tables from directory ./csv_files: [users products orders]

sqly:~/data$ .import file1.csv ./directory file2.tsv
# Импортирует file1.csv, все файлы из directory и file2.tsv

sqly:~/data$ .tables
orders
products
users
```

### Изменение формата вывода
sqly выводит результаты SQL-запросов в следующих форматах:
- Формат ASCII-таблицы (по умолчанию)
- Формат CSV (опция --csv)
- Формат TSV (опция --tsv)
- Формат LTSV (опция --ltsv)

```shell
$ sqly --sql "SELECT * FROM user LIMIT 2" --csv testdata/user.csv 
user_name,identifier,first_name,last_name
booker12,1,Rachel,Booker
jenkins46,2,Mary,Jenkins
```

### Запуск sqly shell
Sqly shell запускается при выполнении команды sqly без опции --sql. Когда вы выполняете команду sqly с путем к файлу, sqly-shell запускается после импорта файла в базу данных SQLite3 в памяти.

```shell
$ sqly 
sqly v0.10.0

введите "SQL-запрос" или "команду sqly, начинающуюся с точки".
.help выводит справку, .exit завершает sqly.

sqly:~/github/github.com/nao1215/sqly(table)$ 
```
  
Sqly shell функционирует аналогично обычному SQL-клиенту (например, команде `sqlite3` или `mysql`). Sqly shell имеет вспомогательные команды, начинающиеся с точки. Sqly-shell также поддерживает историю команд и автодополнение ввода.

Sqly-shell имеет следующие вспомогательные команды:

```shell
sqly:~/github/github.com/nao1215/sqly(table)$ .help
        .cd: изменить каталог
      .dump: дамп таблицы БД в файл в формате согласно режиму вывода (по умолчанию: csv)
      .exit: выйти из sqly
    .header: печать заголовка таблицы
      .help: печать справочного сообщения
    .import: импорт файлов и/или директорий
        .ls: печать содержимого каталога
      .mode: изменить режим вывода
       .pwd: печать текущего рабочего каталога
    .tables: печать таблиц
```

### Вывод результатов SQL в файл
#### Для пользователей Linux
sqly может сохранять результаты выполнения SQL в файл, используя перенаправление shell. Опция --csv выводит результаты выполнения SQL в формате CSV вместо табличного формата.

```shell
$ sqly --sql "SELECT * FROM user" --csv testdata/user.csv > test.csv
```

#### Для пользователей Windows

sqly может сохранять результаты выполнения SQL в файл, используя опцию --output. Опция --output указывает путь назначения для результатов SQL, указанных в опции --sql.

```shell
$ sqly --sql "SELECT * FROM user" --output=test.csv testdata/user.csv 
```

### Привязки клавиш для sqly-shell
|Привязка клавиш	|Описание|
|:--|:--|
|Ctrl + A	|Перейти к началу строки (Home)|
|Ctrl + E	|Перейти к концу строки (End)|
|Ctrl + P	|Предыдущая команда (Стрелка вверх)|
|Ctrl + N	|Следующая команда (Стрелка вниз)|
|Ctrl + F	|Переместиться на один символ вперед|
|Ctrl + B	|Переместиться на один символ назад|
|Ctrl + D	|Удалить символ под курсором|
|Ctrl + H	|Удалить символ перед курсором (Backspace)|
|Ctrl + W	|Вырезать слово перед курсором в буфер обмена|
|Ctrl + K	|Вырезать строку после курсора в буфер обмена|
|Ctrl + U	|Вырезать строку перед курсором в буфер обмена|
|Ctrl + L	|Очистить экран|  
|TAB        |Автодополнение|
|↑          |Предыдущая команда|
|↓          |Следующая команда|

## 📋 Последние изменения


- Официальная документация для пользователей и разработчиков: [https://nao1215.github.io/sqly/](https://nao1215.github.io/sqly/)
- Альтернативный инструмент, созданный тем же разработчиком: [простой терминальный интерфейс для СУБД и локальных CSV/TSV/LTSV](https://github.com/nao1215/sqluv)

### Новое: Поддержка сжатых файлов

**sqly** теперь поддерживает сжатые файлы! Вы можете напрямую обрабатывать:
- Сжатые файлы **Gzip** (`.csv.gz`, `.tsv.gz`, `.ltsv.gz`, `.xlsx.gz`)
- Сжатые файлы **Bzip2** (`.csv.bz2`, `.tsv.bz2`, `.ltsv.bz2`, `.xlsx.bz2`)
- Сжатые файлы **XZ** (`.csv.xz`, `.tsv.xz`, `.ltsv.xz`, `.xlsx.xz`)
- Сжатые файлы **Zstandard** (`.csv.zst`, `.tsv.zst`, `.ltsv.zst`, `.xlsx.zst`)


### Добавленные функции
- **Поддержка CTE (Общие табличные выражения)**: Теперь поддерживаются WITH клаузулы для сложных запросов и рекурсивных операций
- **Интеграция с filesql**: Улучшенная производительность и функциональность с использованием библиотеки [filesql](https://github.com/nao1215/filesql)
- **Улучшенная производительность**: Операции массовой вставки с пакетной обработкой транзакций для более быстрой обработки файлов
- **Лучшая обработка типов**: Автоматическое определение типов обеспечивает правильную числовую сортировку и вычисления
- **Поддержка сжатых файлов**: Нативная поддержка сжатых файлов `.gz`, `.bz2`, `.xz` и `.zst`

### Удаленные функции
- **Поддержка JSON**: Поддержка формата JSON файлов была удалена, чтобы сосредоточиться на форматах структурированных данных (CSV, TSV, LTSV, Excel)
  - Используйте экспорт CSV из JSON-инструментов, если вам нужно обрабатывать JSON-данные с помощью sqly
  - Удаление позволяет лучше оптимизировать основные форматы файлов

### Нарушающие совместимость изменения
- Флаг `--json` был удален
- JSON-файлы (`.json`) больше не поддерживаются как входные
- Числовое форматирование в выводе может немного отличаться из-за улучшенного определения типов

## Бенчмарк
CPU: AMD Ryzen 5 3400G with Radeon Vega Graphics  
Выполнение: 
```sql
SELECT * FROM `table` WHERE `Index` BETWEEN 1000 AND 2000 ORDER BY `Index` DESC LIMIT 1000
```

|Записи  | Столбцы | Время на операцию | Выделенная память на операцию | Выделения на операцию |
|---------|----|-------------------|--------------------------------|---------------------------|
|100,000|   12|  1715818835 ns/op  |      441387928 B/op   |4967183 allocs/op | 
|1,000,000|   9|   11414332112 ns/op |      2767580080 B/op | 39131122 allocs/op |


## Альтернативные инструменты
|Имя| Описание|
|:--|:--|
|[harelba/q](https://github.com/harelba/q)|Запуск SQL напрямую к файлам с разделителями и многофайловым базам данных sqlite|
|[dinedal/textql](https://github.com/dinedal/textql)|Выполнение SQL к структурированному тексту, например CSV или TSV|
|[noborus/trdsql](https://github.com/noborus/trdsql)|CLI-инструмент, который может выполнять SQL-запросы к CSV, LTSV, JSON, YAML и TBLN. Может выводить в различных форматах.|
|[mithrandie/csvq](https://github.com/mithrandie/csvq)|SQL-подобный язык запросов для csv|


## Ограничения (не поддерживается)

- DDL такие как CREATE
- DML такие как GRANT
- TCL такие как транзакции

## Участие в разработке

Прежде всего, спасибо за то, что нашли время для участия! См. [CONTRIBUTING.md](../../CONTRIBUTING.md) для получения дополнительной информации. Участие касается не только разработки. Например, GitHub Star мотивирует меня на разработку!

[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/sqly&type=Date)](https://star-history.com/#nao1215/sqly&Date)

## Как разрабатывать

Пожалуйста, см. [документацию](https://nao1215.github.io/sqly/), раздел "Документация для разработчиков".

При добавлении новых функций или исправлении ошибок, пожалуйста, пишите модульные тесты. sqly имеет модульные тесты для всех пакетов, как показано в приведенной ниже карте дерева модульных тестов.

![treemap](../img/cover-tree.svg)


### Контакты
Если вы хотите отправить комментарии, такие как "найти ошибку" или "запрос дополнительных функций" разработчику, пожалуйста, используйте один из следующих контактов.

- [GitHub Issue](https://github.com/nao1215/sqly/issues)

## Используемые библиотеки

**sqly** использует мощные Go библиотеки для предоставления своей функциональности:
- [filesql](https://github.com/nao1215/filesql) - Предоставляет SQL интерфейс базы данных для файлов CSV/TSV/LTSV/JSON/JSONL/Parquet/Excel с автоматическим определением типов и поддержкой сжатых файлов
- [prompt](https://github.com/nao1215/prompt) - Обеспечивает интерактивную оболочку с SQL автодополнением и историей команд

## ЛИЦЕНЗИЯ
Проект sqly лицензирован в соответствии с условиями [MIT LICENSE](../../LICENSE).

## Участники ✨

Спасибо этим замечательным людям ([ключ эмодзи](https://allcontributors.org/docs/en/emoji-key)):

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
          <a href="https://all-contributors.js.org/docs/en/bot/usage">Добавить ваш вклад</a>
        </img>
      </td>
    </tr>
  </tfoot>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

Этот проект следует спецификации [all-contributors](https://github.com/all-contributors/all-contributors). Приветствуется любой вид участия!