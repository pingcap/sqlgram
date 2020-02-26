# sg - sqlgram generator

a sqlgram/bnf generator for pingcap/parser.


### Install

```bash
go get github.com/pingcap/sqlgram/tools/sg
export $PATH=$GOPATH/bin:$PATH
```

### How to use

```bash
./sg help
generate bnf and sqlgram

Usage:
  sg [command]

Available Commands:
  bnf         Generate EBNF from parser.y.
  help        Help about any command
  svg         Generate SVG diagrams from SQL grammar

Flags:
  -h, --help    help for sg
  -q, --quiet   suppress output where possible

Use "sg [command] --help" for more information about a command.
```

#### Generate bnf from parser.y

```bash
./sg bnf --help
Usage:
  sg bnf [dir] [flags]

Flags:
      --addr string   Location of parser.y file. Can also specify an http address. (default "./github.com/pingcap/parser/parser.y")
  -h, --help          help for bnf
      --spec string   Location of spec.json file. Can also specify an http address.

Global Flags:
  -q, --quiet   suppress output where possible
```

for example:

```bash
./sg bnf /tmp/bnf --addr https://raw.githubusercontent.com/pingcap/parser/master/parser.y --spec https://raw.githubusercontent.com/pingcap/sqlgram/gh-pages/tools/sg/spec-example.json
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

### generate svg from bnf

```bash
./sg svg --help
Usage:
  sg svg [bnf dir] [svg dir] [flags]

Flags:
  -h, --help              help for svg
      --max-workers int   maximum number of concurrent workers (default 1)
      --snippet           generate html code snippet that can be embedded to another page.(default: false)
      --spec string       Location of spec.json file. Can also specify an http address.

Global Flags:
  -q, --quiet   suppress output where possible
``` 

for example:

```bash
./sg svg /tmp/bnf /tmp/svg --spec https://raw.githubusercontent.com/pingcap/sqlgram/gh-pages/tools/sg/spec-example.json 
```

will generate a html page for each bnf files in `/tmp/bnf`.

if add optional `--snippet` like this:

```bash
./sg svg /tmp/bnf /tmp/svg --snippet --spec https://raw.githubusercontent.com/pingcap/sqlgram/gh-pages/tools/sg/spec-example.json 
```

it will generate a code snippet wrapped with `<div>...</div>` just like this:

```html
<div><svg width="761" height="213">
<polygon points="9 61 1 57 1 65"></polygon>
<polygon points="17 61 9 57 9 65"></polygon>
<rect x="31" y="47" width="70" height="32" rx="10"></rect>
<rect x="29" y="45" width="70" height="32" class="terminal" rx="10"></rect>
<text class="terminal" x="39" y="65">SELECT</text><a xlink:href="sql-grammar.html#SelectStmtOpts" xlink:title="SelectStmtOpts">
<rect x="121" y="47" width="118" height="32"></rect>
<rect x="119" y="45" width="118" height="32" class="nonterminal"></rect>
<text class="nonterminal" x="129" y="65">SelectStmtOpts</text></a><a xlink:href="sql-grammar.html#SelectStmtFieldList" xlink:title="SelectStmtFieldList">
<rect x="259" y="47" width="140" height="32"></rect>
<rect x="257" y="45" width="140" height="32" class="nonterminal"></rect>
<text class="nonterminal" x="267" y="65">SelectStmtFieldList</text></a><rect x="419" y="47" width="60" height="32" rx="10"></rect>
<rect x="417" y="45" width="60" height="32" class="terminal" rx="10"></rect>
<text class="terminal" x="427" y="65">FROM</text><a xlink:href="sql-grammar.html#EscapedTableRef" xlink:title="EscapedTableRef">
<rect x="519" y="47" width="128" height="32"></rect>
<rect x="517" y="45" width="128" height="32" class="nonterminal"></rect>
<text class="nonterminal" x="527" y="65">EscapedTableRef</text></a><rect x="519" y="3" width="24" height="32" rx="10"></rect>
<rect x="517" y="1" width="24" height="32" class="terminal" rx="10"></rect>
<text class="terminal" x="527" y="21">,</text>
<rect x="25" y="113" width="70" height="32" rx="10"></rect>
<rect x="23" y="111" width="70" height="32" class="terminal" rx="10"></rect>
<text class="terminal" x="33" y="131">WHERE</text><a xlink:href="sql-grammar.html#Expression" xlink:title="Expression">
<rect x="115" y="113" width="90" height="32"></rect>
<rect x="113" y="111" width="90" height="32" class="nonterminal"></rect>
<text class="nonterminal" x="123" y="131">Expression</text></a><rect x="225" y="113" width="68" height="32" rx="10"></rect>
<rect x="223" y="111" width="68" height="32" class="terminal" rx="10"></rect>
<text class="terminal" x="233" y="131">GROUP</text>
<rect x="313" y="113" width="38" height="32" rx="10"></rect>
<rect x="311" y="111" width="38" height="32" class="terminal" rx="10"></rect>
<text class="terminal" x="321" y="131">BY</text><a xlink:href="sql-grammar.html#ByList" xlink:title="ByList">
<rect x="371" y="113" width="58" height="32"></rect>
<rect x="369" y="111" width="58" height="32" class="nonterminal"></rect>
<text class="nonterminal" x="379" y="131">ByList</text></a><rect x="449" y="113" width="74" height="32" rx="10"></rect>
<rect x="447" y="111" width="74" height="32" class="terminal" rx="10"></rect>
<text class="terminal" x="457" y="131">HAVING</text><a xlink:href="sql-grammar.html#Expression" xlink:title="Expression">
<rect x="543" y="113" width="90" height="32"></rect>
<rect x="541" y="111" width="90" height="32" class="nonterminal"></rect>
<text class="nonterminal" x="551" y="131">Expression</text></a><rect x="653" y="113" width="86" height="32" rx="10"></rect>
<rect x="651" y="111" width="86" height="32" class="terminal" rx="10"></rect>
<text class="terminal" x="661" y="131">WINDOW</text><a xlink:href="sql-grammar.html#WindowDefinitionList" xlink:title="WindowDefinitionList">
<rect x="581" y="179" width="152" height="32"></rect>
<rect x="579" y="177" width="152" height="32" class="nonterminal"></rect>
<text class="nonterminal" x="589" y="197">WindowDefinitionList</text></a><path class="line" d="m17 61 h2 m0 0 h10 m70 0 h10 m0 0 h10 m118 0 h10 m0 0 h10 m140 0 h10 m0 0 h10 m60 0 h10 m20 0 h10 m128 0 h10 m-168 0 l20 0 m-1 0 q-9 0 -9 -10 l0 -24 q0 -10 10 -10 m148 44 l20 0 m-20 0 q10 0 10 -10 l0 -24 q0 -10 -10 -10 m-148 0 h10 m24 0 h10 m0 0 h104 m22 44 l2 0 m2 0 l2 0 m2 0 l2 0 m-686 66 l2 0 m2 0 l2 0 m2 0 l2 0 m2 0 h10 m70 0 h10 m0 0 h10 m90 0 h10 m0 0 h10 m68 0 h10 m0 0 h10 m38 0 h10 m0 0 h10 m58 0 h10 m0 0 h10 m74 0 h10 m0 0 h10 m90 0 h10 m0 0 h10 m86 0 h10 m2 0 l2 0 m2 0 l2 0 m2 0 l2 0 m-202 66 l2 0 m2 0 l2 0 m2 0 l2 0 m2 0 h10 m152 0 h10 m3 0 h-3"></path>
<polygon points="751 193 759 189 759 197"></polygon>
<polygon points="751 193 743 189 743 197"></polygon></svg></div>
```

and we can embedded them into other html page and customize CSS-style in parent pages.
