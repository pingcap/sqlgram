# sg - sqlgram generator

a sqlgram/bnf generator for pingcap/parser.


### Install

```bash
go get github.com/pingcap/sqlgram/tools/sg
export $PATH=$GOPATH/bin:$PATH
```

### How to use

```bash
./sg
generate bnf and sqlgram

Usage:
  sg [command]

Available Commands:
  bnf         Generate EBNF from parser.y.
  help        Help about any command

Flags:
  -h, --help    help for sg
  -q, --quiet   suppress output where possible

Use "sg [command] --help" for more information about a command.
subcommand is required

```

#### Generate bnf from parser.y

```bash
./sg bnf
Error: accepts 1 arg(s), received 0
Usage:
  sg bnf [dir] [flags]

Flags:
      --addr string   Location of sql.y file. Can also specify an http address. (default "./github.com/pingcap/parser/parser.y")
  -h, --help          help for bnf
      --spec string   Location of spec.json file. Can also specify an http address.

Global Flags:
  -q, --quiet   suppress output where possible
```

for example:

```bash
./sg bnf /tmp/2 --addr https://raw.githubusercontent.com/pingcap/parser/master/parser.y --spec https://raw.githubusercontent.com/pingcap/sqlgram/gh-pages/tools/sg/spec-example.json
```

#### customize spec.json to get part of stmt

by default, `sg bnf` will output all-in-one bnf file named `Start.bnf`, but for some docs we need just a part of them, and we also need inline stmt. 

```json
[
  {
    "name": "SelectStmtFromTable",
    "inline": ["SelectStmtBasic", "TableRefsClause", "WhereClauseOptional", "SelectStmtGroup", "HavingClause", "WindowClauseOptional", "WhereClause", "GroupByClause", "TableRefs"],
    "nosplit": true
  }
]
```

and pass it via `--spec` option, `sg` will extract `SelectStmtFromTable` & inline some elements and generate a new bnf file named "SelectStmtFromTable" like this

```bnf
SelectStmtFromTable ::=
        ( 'SELECT' SelectStmtOpts SelectStmtFieldList ) 'FROM' ( ( ( EscapedTableRef ) ( ( ',' EscapedTableRef ) )* ) ) ( ( 'WHERE' Expression ) ) ( ( 'GROUP' 'BY' ByList ) ) ( 'HAVING' Expression ) ( 'WINDOW' WindowDefinitionList )
```

![select_table](https://raw.githubusercontent.com/pingcap/sqlgram/gh-pages/tools/misc/select.png)
