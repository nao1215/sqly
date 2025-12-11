<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-1-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->

![Coverage](https://raw.githubusercontent.com/nao1215/octocovs-central-repo/main/badges/nao1215/sqly/coverage.svg)
[![Build](https://github.com/nao1215/sqly/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/sqly/actions/workflows/build.yml)
[![reviewdog](https://github.com/nao1215/sqly/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/sqly/actions/workflows/reviewdog.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nao1215/sqly)](https://goreportcard.com/report/github.com/nao1215/sqly)
![GitHub](https://img.shields.io/github/license/nao1215/sqly)  
![demo](../img/demo.gif)  

[English](../../README.md) | [日本語](../ja/README.md) | [Русский](../ru/README.md) | [中文](../zh-cn/README.md) | [Español](../es/README.md) | [Français](../fr/README.md)

**sqly**는 CSV, TSV, LTSV, Microsoft Excel™ 파일에 대해 SQL을 실행할 수 있는 강력한 명령줄 도구입니다. sqly는 이러한 파일들을 [SQLite3](https://www.sqlite.org/index.html) 인메모리 데이터베이스로 가져옵니다.

sqly에는 **sqly-shell**이 있습니다. SQL 자동완성과 명령 기록을 통해 대화식으로 SQL을 실행할 수 있습니다. 물론 sqly-shell을 실행하지 않고도 SQL을 실행할 수 있습니다.

```shell
# 압축 파일에서도 작동합니다!
sqly --sql "SELECT * FROM data" data.csv.gz
sqly --sql "SELECT * FROM logs WHERE level='ERROR'" logs.tsv.bz2
```

## 설치 방법
### "go install" 사용
```shell
go install github.com/nao1215/sqly@latest
```

### homebrew 사용
```shell
brew install nao1215/tap/sqly
```

## 지원되는 OS 및 go 버전
- Windows
- macOS
- Linux
- go1.24.0 이상

## 사용 방법
sqly는 파일 경로나 디렉토리 경로를 인수로 전달하면 CSV/TSV/LTSV/Excel 파일(압축 버전 포함)을 자동으로 DB로 가져옵니다. 같은 명령어에서 파일과 디렉토리를 혼합할 수도 있습니다. DB 테이블 이름은 파일명 또는 시트명과 동일합니다(예: user.csv를 가져오면 sqly 명령이 user 테이블을 생성함).

**참고**: 파일명에 SQL 구문 오류를 일으킬 수 있는 문자(하이픈 `-`, 점 `.`, 기타 특수 문자 등)가 포함되어 있으면 자동으로 밑줄 `_`로 대체됩니다. 예를 들어, `bug-syntax-error.csv`는 `bug_syntax_error` 테이블이 됩니다.

### Excel 시트 이름
Excel 파일을 가져올 때 테이블 이름은 `파일명_시트명` 형식으로 생성됩니다. 시트 이름도 SQL 호환성을 위해 정제됩니다:
- 공백, 하이픈, 점은 밑줄로 대체됩니다
- 비ASCII 문자(`é`와 같은 악센트 문자)는 제거됩니다

예:
- 파일 `data.xlsx`의 시트 `A test` → 테이블 `data_A_test`
- 파일 `report.xlsx`의 시트 `Café` → 테이블 `report_Caf`

`--sheet` 옵션을 사용하여 원래 시트 이름(정제 전)으로 시트를 지정할 수 있습니다:
```shell
$ sqly data.xlsx --sheet="A test"
$ sqly report.xlsx --sheet="Café"
```

sqly는 압축 파일을 포함하여 파일 확장자에서 파일 형식을 자동으로 결정합니다.

### 터미널에서 SQL 실행: --sql 옵션
--sql 옵션은 SQL 문을 선택적 인수로 받습니다.

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

### 디렉토리 가져오기
지원되는 파일이 포함된 전체 디렉토리를 가져올 수 있습니다. sqly는 디렉토리에서 모든 CSV, TSV, LTSV, Excel 파일(압축 버전 포함)을 자동으로 감지하고 가져옵니다:

```shell
# 디렉토리에서 모든 파일 가져오기
$ sqly ./data_directory

# 파일과 디렉토리 혼합
$ sqly file1.csv ./data_directory file2.tsv

# --sql 옵션과 함께 사용
$ sqly ./data_directory --sql "SELECT * FROM users"
```

### 대화형 셸: .import 명령어
sqly 셸에서 `.import` 명령어를 사용하여 파일이나 디렉토리를 가져올 수 있습니다:

```shell
sqly:~/data$ .import ./csv_files
Successfully imported 3 tables from directory ./csv_files: [users products orders]

sqly:~/data$ .import file1.csv ./directory file2.tsv
# file1.csv, 디렉토리의 모든 파일, file2.tsv 가져오기

sqly:~/data$ .tables
orders
products
users
```

### 출력 형식 변경
sqly는 SQL 쿼리 결과를 다음 형식으로 출력합니다:
- ASCII 테이블 형식 (기본값)
- CSV 형식 (--csv 옵션)
- TSV 형식 (--tsv 옵션)
- LTSV 형식 (--ltsv 옵션)

```shell
$ sqly --sql "SELECT * FROM user LIMIT 2" --csv testdata/user.csv 
user_name,identifier,first_name,last_name
booker12,1,Rachel,Booker
jenkins46,2,Mary,Jenkins
```

### sqly shell 실행
--sql 옵션 없이 sqly 명령을 실행하면 sqly shell이 시작됩니다. 파일 경로와 함께 sqly 명령을 실행하면 파일을 SQLite3 인메모리 데이터베이스로 가져온 후 sqly-shell이 시작됩니다.

```shell
$ sqly 
sqly v0.10.0

"SQL 쿼리" 또는 "점으로 시작하는 sqly 명령"을 입력하세요.
.help는 사용법 출력, .exit는 sqly 종료입니다.

sqly:~/github/github.com/nao1215/sqly(table)$ 
```
  
sqly shell은 일반적인 SQL 클라이언트(`sqlite3` 명령어나 `mysql` 명령어 등)와 유사하게 작동합니다. sqly shell에는 점으로 시작하는 도우미 명령이 있습니다. sqly-shell은 명령 기록과 입력 자동완성도 지원합니다.

sqly-shell에는 다음과 같은 도우미 명령이 있습니다:

```shell
sqly:~/github/github.com/nao1215/sqly(table)$ .help
        .cd: 디렉토리 변경
      .dump: 출력 모드에 따른 형식으로 DB 테이블을 파일에 덤프 (기본값: csv)
      .exit: sqly 종료
    .header: 테이블 헤더 출력
      .help: 도움말 메시지 출력
    .import: 파일 및/또는 디렉토리 가져오기
        .ls: 디렉토리 내용 출력
      .mode: 출력 모드 변경
       .pwd: 현재 작업 디렉토리 출력
    .tables: 테이블 출력
```

### SQL 결과를 파일로 출력
#### Linux 사용자용
sqly는 셸 리다이렉션을 사용하여 SQL 실행 결과를 파일에 저장할 수 있습니다. --csv 옵션은 SQL 실행 결과를 테이블 형식이 아닌 CSV 형식으로 출력합니다.

```shell
$ sqly --sql "SELECT * FROM user" --csv testdata/user.csv > test.csv
```

#### Windows 사용자용

sqly는 --output 옵션을 사용하여 SQL 실행 결과를 파일에 저장할 수 있습니다. --output 옵션은 --sql 옵션에서 지정된 SQL 결과의 대상 경로를 지정합니다.

```shell
$ sqly --sql "SELECT * FROM user" --output=test.csv testdata/user.csv 
```

### sqly-shell 키 바인딩
|키 바인딩	|설명|
|:--|:--|
|Ctrl + A	|줄 시작으로 이동 (Home)|
|Ctrl + E	|줄 끝으로 이동 (End)|
|Ctrl + P	|이전 명령 (위쪽 화살표)|
|Ctrl + N	|다음 명령 (아래쪽 화살표)|
|Ctrl + F	|한 문자 앞으로|
|Ctrl + B	|한 문자 뒤로|
|Ctrl + D	|커서 아래 문자 삭제|
|Ctrl + H	|커서 앞 문자 삭제 (Backspace)|
|Ctrl + W	|커서 앞 단어를 클립보드로 잘라내기|
|Ctrl + K	|커서 뒤 줄을 클립보드로 잘라내기|
|Ctrl + U	|커서 앞 줄을 클립보드로 잘라내기|
|Ctrl + L	|화면 지우기|  
|TAB        |자동완성|
|↑          |이전 명령|
|↓          |다음 명령|

## 📋 최근 변경 사항


- 사용자 및 개발자를 위한 공식 문서: [https://nao1215.github.io/sqly/](https://nao1215.github.io/sqly/)
- 같은 개발자가 만든 대체 도구: [DBMS 및 로컬 CSV/TSV/LTSV를 위한 간단한 터미널 UI](https://github.com/nao1215/sqluv)

### 신기능: 압축 파일 지원

**sqly**는 이제 압축 파일을 지원합니다! 다음을 직접 처리할 수 있습니다:
- **Gzip** 압축 파일 (`.csv.gz`, `.tsv.gz`, `.ltsv.gz`, `.xlsx.gz`)
- **Bzip2** 압축 파일 (`.csv.bz2`, `.tsv.bz2`, `.ltsv.bz2`, `.xlsx.bz2`)
- **XZ** 압축 파일 (`.csv.xz`, `.tsv.xz`, `.ltsv.xz`, `.xlsx.xz`)
- **Zstandard** 압축 파일 (`.csv.zst`, `.tsv.zst`, `.ltsv.zst`, `.xlsx.zst`)


### 추가된 기능
- **CTE(공통 테이블 표현식) 지원**: 이제 WITH 절을 사용한 복잡한 쿼리와 재귀 작업을 지원합니다
- **filesql 통합**: [filesql](https://github.com/nao1215/filesql) 라이브러리를 사용한 향상된 성능과 기능
- **향상된 성능**: 더 빠른 파일 처리를 위한 트랜잭션 일괄처리로 대량 삽입 작업
- **향상된 타입 처리**: 자동 타입 검지가 적절한 숫자 정렬과 계산을 보장
- **압축 파일 지원**: `.gz`, `.bz2`, `.xz`, `.zst` 압축 파일에 대한 네이티브 지원

### 제거된 기능
- **JSON 지원**: 구조화된 데이터 형식(CSV, TSV, LTSV, Excel)에 집중하기 위해 JSON 파일 형식 지원이 제거되었습니다
  - sqly로 JSON 데이터를 처리해야 하는 경우 JSON 도구에서 CSV 내보내기를 사용하십시오
  - 제거를 통해 핵심 파일 형식의 더 나은 최적화가 가능합니다

### 호환성을 깨는 변경
- `--json` 플래그가 제거되었습니다
- JSON 파일(`.json`)이 더 이상 입력으로 지원되지 않습니다
- 향상된 타입 검지로 인해 출력의 숫자 형식이 약간 다를 수 있습니다

## 벤치마크
CPU: AMD Ryzen 5 3400G with Radeon Vega Graphics  
실행: 
```sql
SELECT * FROM `table` WHERE `Index` BETWEEN 1000 AND 2000 ORDER BY `Index` DESC LIMIT 1000
```

|레코드 수  | 컬럼 수 | 작업당 시간 | 작업당 메모리 할당 | 작업당 할당 횟수 |
|---------|----|-------------------|--------------------------------|---------------------------|
|100,000|   12|  1715818835 ns/op  |      441387928 B/op   |4967183 allocs/op | 
|1,000,000|   9|   11414332112 ns/op |      2767580080 B/op | 39131122 allocs/op |


## 대체 도구
|이름| 설명|
|:--|:--|
|[harelba/q](https://github.com/harelba/q)|구분된 파일과 다중 파일 sqlite 데이터베이스에 대해 직접 SQL 실행|
|[dinedal/textql](https://github.com/dinedal/textql)|CSV나 TSV 같은 구조화된 텍스트에 대해 SQL 실행|
|[noborus/trdsql](https://github.com/noborus/trdsql)|CSV, LTSV, JSON, YAML, TBLN에 대해 SQL 쿼리를 실행할 수 있는 CLI 도구. 다양한 형식으로 출력 가능.|
|[mithrandie/csvq](https://github.com/mithrandie/csvq)|CSV용 SQL과 같은 쿼리 언어|


## 제한사항 (지원하지 않음)

- CREATE 등의 DDL
- GRANT 등의 DML
- 트랜잭션 등의 TCL

## 기여하기

우선, 기여해 주셔서 감사합니다! 자세한 내용은 [CONTRIBUTING.md](../../CONTRIBUTING.md)를 참조하세요. 기여는 개발과 관련된 것만이 아닙니다. 예를 들어, GitHub Star는 개발 동기를 부여합니다!

[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/sqly&type=Date)](https://star-history.com/#nao1215/sqly&Date)

## 개발 방법

[문서](https://nao1215.github.io/sqly/)의 "개발자를 위한 문서" 섹션을 참조하세요.

새로운 기능을 추가하거나 버그를 수정할 때는 단위 테스트를 작성해 주세요. sqly는 아래의 단위 테스트 트리 맵이 보여주는 것처럼 모든 패키지에 대해 단위 테스트가 작성되어 있습니다.

![treemap](../img/cover-tree.svg)


### 연락처
"버그 발견" 또는 "추가 기능 요청"과 같은 의견을 개발자에게 보내고 싶다면 다음 연락처 중 하나를 사용해 주세요.

- [GitHub Issue](https://github.com/nao1215/sqly/issues)

## 사용된 라이브러리

**sqly**는 기능을 제공하기 위해 강력한 Go 라이브러리를 활용합니다:
- [filesql](https://github.com/nao1215/filesql) - CSV/TSV/LTSV/Excel 파일에 대한 SQL 데이터베이스 인터페이스를 제공하고 자동 타입 검지 및 압축 파일 지원을 제공합니다
- [prompt](https://github.com/nao1215/prompt) - SQL 자동완성 및 명령 기록 기능으로 대화형 셸을 구현합니다

## 라이선스
sqly 프로젝트는 [MIT LICENSE](../../LICENSE) 조건에 따라 라이선스가 부여됩니다.

## 기여자 ✨

이 훌륭한 분들에게 감사드립니다 ([이모지 키](https://allcontributors.org/docs/en/emoji-key)):

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
          <a href="https://all-contributors.js.org/docs/en/bot/usage">기여 추가하기</a>
        </img>
      </td>
    </tr>
  </tfoot>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

이 프로젝트는 [all-contributors](https://github.com/all-contributors/all-contributors) 사양을 따릅니다. 어떤 종류의 기여든 환영합니다!