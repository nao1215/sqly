<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-2-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->
  
![Coverage](https://raw.githubusercontent.com/nao1215/octocovs-central-repo/main/badges/nao1215/sqly/coverage.svg)
[![Build](https://github.com/nao1215/sqly/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/sqly/actions/workflows/build.yml)
[![reviewdog](https://github.com/nao1215/sqly/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/sqly/actions/workflows/reviewdog.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nao1215/sqly)](https://goreportcard.com/report/github.com/nao1215/sqly)
![GitHub](https://img.shields.io/github/license/nao1215/sqly)  
![demo](../img/demo.gif)  

[English](../../README.md) | [日本語](../ja/README.md) | [Русский](../ru/README.md) | [中文](../zh-cn/README.md) | [한국어](../ko/README.md) | [Español](../es/README.md)

sqly est un outil en ligne de commande qui execute du SQL sur des fichiers CSV, TSV, LTSV, JSON, JSONL, Parquet, Microsoft Excel, ACH et Fedwire. Il importe ces fichiers dans une base de donnees en memoire [SQLite3](https://www.sqlite.org/index.html). Les fichiers compresses (.gz, .bz2, .xz, .zst, .z, .snappy, .s2, .lz4) sont egalement pris en charge. CTE (clause WITH) est disponible pour les requetes complexes.

sqly dispose d'un shell interactif (sqly-shell) avec l'autocompletion SQL et l'historique des commandes. Vous pouvez aussi executer SQL directement depuis la ligne de commande sans le shell.

```shell
sqly --sql "SELECT * FROM data" data.csv.gz
sqly --sql "SELECT * FROM logs WHERE level='ERROR'" logs.tsv.bz2
```

## Comment installer
### Utiliser "go install"
```shell
go install github.com/nao1215/sqly@latest
```

### Utiliser homebrew
```shell
brew install nao1215/tap/sqly
```

## OS supportés et version go
- Windows
- macOS
- Linux
- go1.25.0 ou ultérieur

## Comment utiliser
sqly importe automatiquement les fichiers CSV/TSV/LTSV/JSON/JSONL/Parquet/Excel/ACH/Fedwire (y compris les versions compressées) dans la base de données lorsque vous passez des chemins de fichier ou des chemins de répertoire comme arguments. Vous pouvez également mélanger fichiers et répertoires dans la même commande. Le nom de la table de la base de données est identique au nom du fichier ou nom de feuille (par exemple, si vous importez user.csv, la commande sqly crée la table user).

**Note** : Les noms de tables sont assainis pour la compatibilite SQL. Les espaces, traits d'union (`-`) et points (`.`) sont remplaces par des traits de soulignement (`_`). Les autres caracteres speciaux (par exemple, `@`, `#`, `$`) sont supprimes. Si le nom resultant commence par un chiffre, le prefixe `sheet_` est ajoute.

Exemples :
- `bug-syntax-error.csv` → table `bug_syntax_error`
- `2023-data.csv` → table `sheet_2023_data`
- `data@v2.csv` → table `datav2`

### Noms des feuilles Excel
Lors de l'importation de fichiers Excel, les noms de tables sont créés au format `nomfichier_nomfeuille`. Les noms de feuilles sont également traités pour la compatibilité SQL :
- Les espaces, traits d'union et points sont remplacés par des traits de soulignement
- Les caractères non-ASCII (comme les caractères accentués `é`) sont supprimés

Par exemple :
- Fichier `data.xlsx` avec feuille `A test` → table `data_A_test`
- Fichier `report.xlsx` avec feuille `Café` → table `report_Caf`

Vous pouvez spécifier un nom de feuille en utilisant l'option `--sheet` avec le nom original (avant traitement) :
```shell
$ sqly data.xlsx --sheet="A test"
$ sqly report.xlsx --sheet="Café"
```

sqly détermine automatiquement le format du fichier à partir de l'extension, y compris les fichiers comprimés.

### Fichiers ACH
Les fichiers ACH (Automated Clearing House) (`.ach`) sont chargés sous forme de plusieurs tables pour faciliter les requêtes :
- `{filename}_file_header` — en-tête au niveau du fichier (1 ligne)
- `{filename}_batches` — informations d'en-tête de lot
- `{filename}_entries` — enregistrements de détail d'entrée (données principales de transaction)
- `{filename}_addenda` — enregistrements d'addenda

Pour les IAT (International ACH Transactions), des tables supplémentaires sont créées : `{filename}_iat_batches`, `{filename}_iat_entries`, `{filename}_iat_addenda`.

```shell
$ sqly ppd-debit.ach
$ sqly --sql "SELECT * FROM ppd_debit_entries WHERE amount > 10000" ppd-debit.ach
```

### Fichiers Fedwire
Les fichiers Fedwire (`.fed`) sont chargés sous forme d'une seule table de message :
- `{filename}_message` — table plate avec tous les champs FEDWireMessage

```shell
$ sqly customer-transfer.fed
$ sqly --sql "SELECT * FROM customer_transfer_message" customer-transfer.fed
```

### Exécuter SQL dans le terminal : option --sql
L'option --sql prend une instruction SQL comme argument optionnel.

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

### Importation de répertoires
Vous pouvez importer des répertoires entiers contenant des fichiers supportés. sqly détecte automatiquement tous les fichiers CSV, TSV, LTSV, Excel, ACH et Fedwire (y compris les versions compressées) dans le répertoire et les importe :

```shell
# Importer tous les fichiers d'un répertoire
$ sqly ./data_directory

# Mélanger fichiers et répertoires
$ sqly file1.csv ./data_directory file2.tsv

# Utiliser avec l'option --sql
$ sqly ./data_directory --sql "SELECT * FROM users"
```

### Changer le format de sortie
sqly affiche les résultats des requêtes SQL dans les formats suivants :
- Format de table ASCII (par défaut)
- Format CSV (option --csv)
- Format TSV (option --tsv)
- Format LTSV (option --ltsv)

```shell
$ sqly --sql "SELECT * FROM user LIMIT 2" --csv testdata/user.csv 
user_name,identifier,first_name,last_name
booker12,1,Rachel,Booker
jenkins46,2,Mary,Jenkins
```

### Shell interactif : commande .import
Dans le shell sqly, vous pouvez utiliser la commande `.import` pour importer des fichiers ou répertoires :

```shell
sqly:~/data$ .import ./csv_files
Importation réussie de 3 tables depuis le répertoire ./csv_files : [users products orders]

sqly:~/data$ .import file1.csv ./directory file2.tsv
# Importe file1.csv, tous les fichiers du répertoire, et file2.tsv

sqly:~/data$ .tables
orders
products
users
```

### Exécuter sqly shell
Le shell sqly démarre lorsque vous exécutez la commande sqly sans l'option --sql. Lorsque vous exécutez la commande sqly avec un chemin de fichier, sqly-shell démarre après avoir importé le fichier dans la base de données en mémoire SQLite3.

```shell
$ sqly 
sqly v0.10.0

entrez "requête SQL" ou "commande sqly qui commence par un point".
.help affiche l'utilisation, .exit quitte sqly.

sqly:~/github/github.com/nao1215/sqly(table)$ 
```
  
Le shell sqly fonctionne de manière similaire à un client SQL commun (par exemple, la commande `sqlite3` ou `mysql`). Le shell sqly a des commandes d'aide qui commencent par un point. Le sqly-shell prend également en charge l'historique des commandes et l'autocomplétion d'entrée.

Le sqly-shell a les commandes d'aide suivantes :

```shell
sqly:~/github/github.com/nao1215/sqly(table)$ .help
        .cd: changer de répertoire
      .dump: vider la table de la base de données vers un fichier dans un format selon le mode de sortie (par défaut : csv)
      .exit: quitter sqly
    .header: afficher l'en-tête de table
      .help: afficher le message d'aide
    .import: importer des fichiers et/ou répertoires
        .ls: afficher le contenu du répertoire
      .mode: changer le mode de sortie
       .pwd: afficher le répertoire de travail actuel
    .tables: afficher les tables
```

### Sortir le résultat SQL vers un fichier
#### Pour les utilisateurs Linux
sqly peut sauvegarder les résultats d'exécution SQL vers le fichier en utilisant la redirection shell. L'option --csv affiche les résultats d'exécution SQL au format CSV au lieu du format tableau.

```shell
$ sqly --sql "SELECT * FROM user" --csv testdata/user.csv > test.csv
```

#### Pour les utilisateurs Windows

sqly peut sauvegarder les résultats d'exécution SQL vers le fichier en utilisant l'option --output. L'option --output spécifie le chemin de destination pour les résultats SQL spécifiés dans l'option --sql.

```shell
$ sqly --sql "SELECT * FROM user" --output=test.csv testdata/user.csv 
```

### Raccourcis clavier pour sqly-shell
|Raccourci clavier	|Description|
|:--|:--|
|Ctrl + A	|Aller au début de la ligne (Début)|
|Ctrl + E	|Aller à la fin de la ligne (Fin)|
|Ctrl + P	|Commande précédente (Flèche haut)|
|Ctrl + N	|Commande suivante (Flèche bas)|
|Ctrl + F	|Avancer d'un caractère|
|Ctrl + B	|Reculer d'un caractère|
|Ctrl + D	|Supprimer le caractère sous le curseur|
|Ctrl + H	|Supprimer le caractère avant le curseur (Retour arrière)|
|Ctrl + W	|Couper le mot avant le curseur vers le presse-papiers|
|Ctrl + K	|Couper la ligne après le curseur vers le presse-papiers|
|Ctrl + U	|Couper la ligne avant le curseur vers le presse-papiers|
|Ctrl + L	|Effacer l'écran|  
|TAB        |Autocomplétion|
|↑          |Commande précédente|
|↓          |Commande suivante|

### Formats de fichiers pris en charge

| Format | Extensions | Remarques |
|:--|:--|:--|
| CSV | `.csv` | |
| TSV | `.tsv` | |
| LTSV | `.ltsv` | |
| JSON | `.json` | Stocke dans la colonne `data` ; utilisez `json_extract()` pour interroger |
| JSONL | `.jsonl` | Stocke dans la colonne `data` ; utilisez `json_extract()` pour interroger |
| Parquet | `.parquet` | |
| Excel | `.xlsx` | Chaque feuille devient une table separee |
| ACH | `.ach` | Cree plusieurs tables (_file_header, _batches, _entries, _addenda) |
| Fedwire | `.fed` | Cree une seule table _message |

CSV/TSV/LTSV/JSON/JSONL/Parquet/Excel prennent egalement en charge les extensions de compression suivantes: `.gz`, `.bz2`, `.xz`, `.zst`, `.z`, `.snappy`, `.s2`, `.lz4`
(par exemple: `.csv.gz`, `.tsv.bz2`, `.ltsv.xz`)

## Benchmark
CPU: AMD Ryzen 5 3400G with Radeon Vega Graphics  
Exécuter : 
```sql
SELECT * FROM `table` WHERE `Index` BETWEEN 1000 AND 2000 ORDER BY `Index` DESC LIMIT 1000
```

|Enregistrements  | Colonnes | Temps par opération | Mémoire allouée par opération | Allocations par opération |
|---------|----|-------------------|--------------------------------|---------------------------|
|100,000|   12|  1715818835 ns/op  |      441387928 B/op   |4967183 allocs/op | 
|1,000,000|   9|   11414332112 ns/op |      2767580080 B/op | 39131122 allocs/op |


## Outils alternatifs
|Nom| Description|
|:--|:--|
|[nao1215/sqluv](https://github.com/nao1215/sqluv)|Interface terminale simple pour SGBD et CSV/TSV/LTSV local|
|[harelba/q](https://github.com/harelba/q)|Executer SQL directement sur des fichiers delimites et des bases de donnees sqlite multi-fichiers|
|[dinedal/textql](https://github.com/dinedal/textql)|Exécuter SQL contre du texte structuré comme CSV ou TSV|
|[noborus/trdsql](https://github.com/noborus/trdsql)|Outil CLI qui peut exécuter des requêtes SQL sur CSV, LTSV, JSON, YAML et TBLN. Peut sortir vers divers formats.|
|[mithrandie/csvq](https://github.com/mithrandie/csvq)|Langage de requête de type SQL pour csv|


## Limitations (Non supporté)

- DDL comme CREATE
- DML comme GRANT
- TCL comme les Transactions

## Contribuer

Tout d'abord, merci de prendre le temps de contribuer ! Voir [CONTRIBUTING.md](../../CONTRIBUTING.md) pour plus d'informations. Les contributions ne sont pas seulement liées au développement. Par exemple, GitHub Star me motive à développer !

[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/sqly&type=Date)](https://star-history.com/#nao1215/sqly&Date)

## Comment développer

Veuillez consulter la [documentation](https://nao1215.github.io/sqly/), section "Documentation pour les développeurs".

Lors de l'ajout de nouvelles fonctionnalités ou de la correction de bugs, veuillez écrire des tests unitaires. sqly est testé unitairement pour tous les packages comme le montre la carte arborescente des tests unitaires ci-dessous.

![treemap](../img/cover-tree.svg)


### Contact
Si vous souhaitez envoyer des commentaires tels que "trouver un bug" ou "demande de fonctionnalités supplémentaires" au développeur, veuillez utiliser l'un des contacts suivants.

- [GitHub Issue](https://github.com/nao1215/sqly/issues)

## Bibliothèques utilisées

**sqly** exploite de puissantes bibliothèques Go pour fournir ses fonctionnalités :
- [filesql](https://github.com/nao1215/filesql) - Fournit une interface de base de données SQL pour les fichiers CSV/TSV/LTSV/JSON/JSONL/Parquet/Excel avec détection automatique des types et support des fichiers compressés
- [prompt](https://github.com/nao1215/prompt) - Alimente le shell interactif avec des fonctionnalités d'autocomplétion SQL et d'historique des commandes

## LICENCE
Le projet sqly est sous licence selon les termes de [MIT LICENSE](../../LICENSE).

## Contributeurs ✨

Merci à ces merveilleuses personnes ([clé emoji](https://allcontributors.org/docs/en/emoji-key)) :

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
          <a href="https://all-contributors.js.org/docs/en/bot/usage">Ajouter vos contributions</a>
        </img>
      </td>
    </tr>
  </tfoot>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

Ce projet suit la spécification [all-contributors](https://github.com/all-contributors/all-contributors). Les contributions de tout type sont les bienvenues !