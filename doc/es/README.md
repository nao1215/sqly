<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-2-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->
  
![Coverage](https://raw.githubusercontent.com/nao1215/octocovs-central-repo/main/badges/nao1215/sqly/coverage.svg)
[![Build](https://github.com/nao1215/sqly/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/sqly/actions/workflows/build.yml)
[![reviewdog](https://github.com/nao1215/sqly/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/sqly/actions/workflows/reviewdog.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nao1215/sqly)](https://goreportcard.com/report/github.com/nao1215/sqly)
![GitHub](https://img.shields.io/github/license/nao1215/sqly)  
![demo](../img/demo.gif)  

[English](../../README.md) | [日本語](../ja/README.md) | [Русский](../ru/README.md) | [中文](../zh-cn/README.md) | [한국어](../ko/README.md) | [Français](../fr/README.md)

sqly es una herramienta de línea de comandos que ejecuta SQL contra archivos CSV, TSV, LTSV, JSON, JSONL, Parquet, Microsoft Excel, ACH y Fedwire. Importa estos archivos a una base de datos en memoria [SQLite3](https://www.sqlite.org/index.html). Los archivos comprimidos (.gz, .bz2, .xz, .zst, .z, .snappy, .s2, .lz4) tambien son compatibles. CTE (clausula WITH) esta disponible para consultas complejas.

sqly tiene un shell interactivo (sqly-shell) con autocompletado SQL e historial de comandos. Tambien puede ejecutar SQL directamente desde la linea de comandos sin el shell.

```shell
sqly --sql "SELECT * FROM data" data.csv.gz
sqly --sql "SELECT * FROM logs WHERE level='ERROR'" logs.tsv.bz2
```

## Cómo instalar
### Usar "go install"
```shell
go install github.com/nao1215/sqly@latest
```

### Usar homebrew
```shell
brew install nao1215/tap/sqly
```

## SO y versión de go compatibles
- Windows
- macOS
- Linux
- go1.25.0 o posterior

## Cómo usar
sqly importa automáticamente archivos CSV/TSV/LTSV/JSON/JSONL/Parquet/Excel/ACH/Fedwire (incluyendo versiones comprimidas) a la base de datos cuando pasa rutas de archivo o rutas de directorio como argumentos. También puede mezclar archivos y directorios en el mismo comando. El nombre de la tabla de la base de datos es el mismo que el nombre del archivo o nombre de hoja (por ejemplo, si importa user.csv, el comando sqly crea la tabla user).

**Nota**: Si el nombre del archivo contiene caracteres que podrían causar errores de sintaxis SQL (como guiones `-`, puntos `.` u otros caracteres especiales), se reemplazan automáticamente con guiones bajos `_`. Por ejemplo, `bug-syntax-error.csv` se convierte en la tabla `bug_syntax_error`.

Si el nombre resultante comienza con un dígito, se agrega el prefijo `sheet_` (por ejemplo, `2023-data.csv` se convierte en la tabla `sheet_2023_data`).

### Nombres de hojas de Excel
Al importar archivos Excel, los nombres de las tablas se crean en el formato `nombrearchivo_nombrehoja`. Los nombres de las hojas también se procesan para compatibilidad con SQL:
- Los espacios, guiones y puntos se reemplazan con guiones bajos
- Los caracteres no ASCII (como los caracteres acentuados `é`) se eliminan

Por ejemplo:
- Archivo `data.xlsx` con hoja `A test` → tabla `data_A_test`
- Archivo `report.xlsx` con hoja `Café` → tabla `report_Caf`

Puede especificar un nombre de hoja usando la opción `--sheet` con el nombre original (antes del procesamiento):
```shell
$ sqly data.xlsx --sheet="A test"
$ sqly report.xlsx --sheet="Café"
```

sqly determina automáticamente el formato del archivo a partir de la extensión, incluyendo archivos comprimidos.

### Archivos ACH
Los archivos ACH (Automated Clearing House) (`.ach`) se cargan como múltiples tablas para facilitar las consultas:
- `{filename}_file_header` — encabezado a nivel de archivo (1 fila)
- `{filename}_batches` — información de encabezado de lote
- `{filename}_entries` — registros de detalle de entrada (datos principales de transacciones)
- `{filename}_addenda` — registros de adenda

Para IAT (International ACH Transactions), se crean tablas adicionales: `{filename}_iat_batches`, `{filename}_iat_entries`, `{filename}_iat_addenda`.

```shell
$ sqly ppd-debit.ach
$ sqly --sql "SELECT * FROM ppd_debit_entries WHERE amount > 10000" ppd-debit.ach
```

### Archivos Fedwire
Los archivos Fedwire (`.fed`) se cargan como una única tabla de mensaje:
- `{filename}_message` — tabla plana con todos los campos de FEDWireMessage

```shell
$ sqly customer-transfer.fed
$ sqly --sql "SELECT * FROM customer_transfer_message" customer-transfer.fed
```

### Ejecutar SQL en terminal: opción --sql
La opción --sql toma una declaración SQL como argumento opcional.

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

### Importación de directorios
Puede importar directorios completos que contengan archivos compatibles. sqly detecta automáticamente todos los archivos CSV, TSV, LTSV, Excel, ACH y Fedwire (incluyendo versiones comprimidas) en el directorio y los importa:

```shell
# Importar todos los archivos de un directorio
$ sqly ./data_directory

# Mezclar archivos y directorios
$ sqly file1.csv ./data_directory file2.tsv

# Usar con la opción --sql
$ sqly ./data_directory --sql "SELECT * FROM users"
```

### Cambiar formato de salida
sqly muestra los resultados de consultas SQL en los siguientes formatos:
- Formato de tabla ASCII (predeterminado)
- Formato CSV (opción --csv)
- Formato TSV (opción --tsv)
- Formato LTSV (opción --ltsv)

```shell
$ sqly --sql "SELECT * FROM user LIMIT 2" --csv testdata/user.csv 
user_name,identifier,first_name,last_name
booker12,1,Rachel,Booker
jenkins46,2,Mary,Jenkins
```

### Shell interactivo: comando .import
En el shell de sqly, puede usar el comando `.import` para importar archivos o directorios:

```shell
sqly:~/data$ .import ./csv_files
Importadas exitosamente 3 tablas del directorio ./csv_files: [users products orders]

sqly:~/data$ .import file1.csv ./directory file2.tsv
# Importa file1.csv, todos los archivos del directorio, y file2.tsv

sqly:~/data$ .tables
orders
products
users
```

### Ejecutar sqly shell
El shell de sqly se inicia cuando ejecuta el comando sqly sin la opción --sql. Cuando ejecuta el comando sqly con una ruta de archivo, sqly-shell se inicia después de importar el archivo a la base de datos en memoria SQLite3.

```shell
$ sqly 
sqly v0.10.0

ingrese "consulta SQL" o "comando sqly que comience con un punto".
.help muestra uso, .exit sale de sqly.

sqly:~/github/github.com/nao1215/sqly(table)$ 
```
  
El shell de sqly funciona de manera similar a un cliente SQL común (por ejemplo, el comando `sqlite3` o el comando `mysql`). El shell de sqly tiene comandos auxiliares que comienzan con un punto. El sqly-shell también admite historial de comandos y autocompletado de entrada.

El sqly-shell tiene los siguientes comandos auxiliares:

```shell
sqly:~/github/github.com/nao1215/sqly(table)$ .help
        .cd: cambiar directorio
      .dump: volcar tabla de BD a archivo en formato según modo de salida (predeterminado: csv)
      .exit: salir de sqly
    .header: imprimir encabezado de tabla
      .help: imprimir mensaje de ayuda
    .import: importar archivo(s) y/o directorio(s)
        .ls: imprimir contenido del directorio
      .mode: cambiar modo de salida
       .pwd: imprimir directorio de trabajo actual
    .tables: imprimir tablas
```

### Generar resultado SQL a archivo
#### Para usuarios de Linux
sqly puede guardar los resultados de ejecución SQL al archivo usando redirección de shell. La opción --csv genera resultados de ejecución SQL en formato CSV en lugar de formato de tabla.

```shell
$ sqly --sql "SELECT * FROM user" --csv testdata/user.csv > test.csv
```

#### Para usuarios de Windows

sqly puede guardar los resultados de ejecución SQL al archivo usando la opción --output. La opción --output especifica la ruta de destino para los resultados SQL especificados en la opción --sql.

```shell
$ sqly --sql "SELECT * FROM user" --output=test.csv testdata/user.csv 
```

### Vinculaciones de teclas para sqly-shell
|Vinculación de teclas	|Descripción|
|:--|:--|
|Ctrl + A	|Ir al comienzo de la línea (Inicio)|
|Ctrl + E	|Ir al final de la línea (Fin)|
|Ctrl + P	|Comando anterior (Flecha arriba)|
|Ctrl + N	|Comando siguiente (Flecha abajo)|
|Ctrl + F	|Avanzar un carácter|
|Ctrl + B	|Retroceder un carácter|
|Ctrl + D	|Eliminar carácter bajo el cursor|
|Ctrl + H	|Eliminar carácter antes del cursor (Retroceso)|
|Ctrl + W	|Cortar la palabra antes del cursor al portapapeles|
|Ctrl + K	|Cortar la línea después del cursor al portapapeles|
|Ctrl + U	|Cortar la línea antes del cursor al portapapeles|
|Ctrl + L	|Limpiar pantalla|  
|TAB        |Autocompletado|
|↑          |Comando anterior|
|↓          |Comando siguiente|

### Formatos de archivo compatibles

| Formato | Extensiones |
|:--|:--|
| CSV | `.csv` |
| TSV | `.tsv` |
| LTSV | `.ltsv` |
| JSON | `.json` |
| JSONL | `.jsonl` |
| Parquet | `.parquet` |
| Excel | `.xlsx` |

Los datos JSON/JSONL se almacenan en una sola columna `data`. Use `json_extract()` de SQLite para consultar campos individuales.

Cada formato tambien admite las siguientes extensiones de compresion: `.gz`, `.bz2`, `.xz`, `.zst`, `.z`, `.snappy`, `.s2`, `.lz4`
(por ejemplo: `.csv.gz`, `.tsv.bz2`, `.ltsv.xz`)

## Benchmark
CPU: AMD Ryzen 5 3400G with Radeon Vega Graphics  
Ejecutar: 
```sql
SELECT * FROM `table` WHERE `Index` BETWEEN 1000 AND 2000 ORDER BY `Index` DESC LIMIT 1000
```

|Registros  | Columnas | Tiempo por operación | Memoria asignada por operación | Asignaciones por operación |
|---------|----|-------------------|--------------------------------|---------------------------|
|100,000|   12|  1715818835 ns/op  |      441387928 B/op   |4967183 allocs/op | 
|1,000,000|   9|   11414332112 ns/op |      2767580080 B/op | 39131122 allocs/op |


## Herramientas alternativas
|Nombre| Descripción|
|:--|:--|
|[nao1215/sqluv](https://github.com/nao1215/sqluv)|Interfaz de terminal simple para DBMS y CSV/TSV/LTSV local|
|[harelba/q](https://github.com/harelba/q)|Ejecutar SQL directamente en archivos delimitados y bases de datos sqlite multi-archivo|
|[dinedal/textql](https://github.com/dinedal/textql)|Ejecutar SQL contra texto estructurado como CSV o TSV|
|[noborus/trdsql](https://github.com/noborus/trdsql)|Herramienta CLI que puede ejecutar consultas SQL en CSV, LTSV, JSON, YAML y TBLN. Puede generar a varios formatos.|
|[mithrandie/csvq](https://github.com/mithrandie/csvq)|Lenguaje de consulta tipo SQL para csv|


## Limitaciones (No compatible)

- DDL como CREATE
- DML como GRANT
- TCL como Transacciones

## Contribuir

En primer lugar, ¡gracias por tomarte el tiempo para contribuir! Consulte [CONTRIBUTING.md](../../CONTRIBUTING.md) para obtener más información. Las contribuciones no solo están relacionadas con el desarrollo. ¡Por ejemplo, GitHub Star me motiva a desarrollar!

[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/sqly&type=Date)](https://star-history.com/#nao1215/sqly&Date)

## Cómo desarrollar

Consulte la [documentación](https://nao1215.github.io/sqly/), sección "Documentación para desarrolladores".

Al agregar nuevas funciones o corregir errores, escriba pruebas unitarias. sqly se prueba unitariamente para todos los paquetes como muestra el mapa de árbol de pruebas unitarias a continuación.

![treemap](../img/cover-tree.svg)


### Contacto
Si desea enviar comentarios como "encontrar un error" o "solicitar funciones adicionales" al desarrollador, utilice uno de los siguientes contactos.

- [GitHub Issue](https://github.com/nao1215/sqly/issues)

## Bibliotecas utilizadas

**sqly** aprovecha potentes bibliotecas de Go para proporcionar su funcionalidad:
- [filesql](https://github.com/nao1215/filesql) - Proporciona interfaz de base de datos SQL para archivos CSV/TSV/LTSV/JSON/JSONL/Parquet/Excel con detección automática de tipos y soporte para archivos comprimidos
- [prompt](https://github.com/nao1215/prompt) - Potencia el shell interactivo con características de autocompletado SQL e historial de comandos

## LICENCIA
El proyecto sqly está licenciado bajo los términos de [MIT LICENSE](../../LICENSE).

## Colaboradores ✨

Gracias a estas maravillosas personas ([clave de emoji](https://allcontributors.org/docs/en/emoji-key)):

<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<!-- prettier-ignore-start -->
<!-- markdownlint-disable -->
<table>
  <tbody>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://debimate.jp/"><img src="https://avatars.githubusercontent.com/u/22737008?v=4?s=75" width="75px;" alt="CHIKAMATSU Naohiro"/><br /><sub><b>CHIKAMATSU Naohiro</b></sub></a><br /><a href="https://github.com/nao1215/sqly/commits?author=nao1215" title="Code">💻</a> <a href="https://github.com/nao1215/sqly/commits?author=nao1215" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/Wozzardman"><img src="https://avatars.githubusercontent.com/u/128730409?v=4?s=75" width="75px;" alt="Wozzardman"/><br /><sub><b>Wozzardman</b></sub></a><br /><a href="https://github.com/nao1215/sqly/commits?author=Wozzardman" title="Code">💻</a></td>
    </tr>
  </tbody>
  <tfoot>
    <tr>
      <td align="center" size="13px" colspan="7">
        <img src="https://raw.githubusercontent.com/all-contributors/all-contributors-cli/1b8533af435da9854653492b1327a23a4dbd0a10/assets/logo-small.svg">
          <a href="https://all-contributors.js.org/docs/en/bot/usage">Agregar sus contribuciones</a>
        </img>
      </td>
    </tr>
  </tfoot>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

Este proyecto sigue la especificación [all-contributors](https://github.com/all-contributors/all-contributors). ¡Las contribuciones de cualquier tipo son bienvenidas!