// Copyright 2017 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/cockroachdb/cockroach/pkg/cmd/docgen/extract"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const topStmt = "Start"

var (
	cmds        []*cobra.Command
	quiet       bool
	filter      string
	invertMatch bool
	addr        string
	spec        string
	maxWorkers  int
)

type stmtSpec struct {
	Name       string            `json:"name,omitempty"`
	Stmt       string            `json:"stmt,omitempty"`
	Inline     []string          `json:"inline,omitempty"`
	Replace    map[string]string `json:"replace,omitempty"`
	RegReplace map[string]string `json:"regreplace,omitempty"`
	Match      []*regexp.Regexp  `json:"match"`
	Exclude    []*regexp.Regexp  `json:"exclude"`
	Unlink     []string          `json:"unlink"`
	Relink     map[string]string `json:"relink"`
	Nosplit    bool              `json:"nosplit"`
}

func init() {
	cmdBNF := &cobra.Command{
		Use:   "bnf [dir]",
		Short: "Generate EBNF from parser.y.",
		Args:  cobra.ExactArgs(1),
		Run:   runBnf,
	}
	cmdBNF.Flags().StringVar(&addr, "addr", "./github.com/pingcap/parser/parser.y", "Location of sql.y file. Can also specify an http address.")
	cmdBNF.Flags().StringVar(&spec, "spec", "", "Location of spec.json file. Can also specify an http address.")

	cmdSVG := &cobra.Command{
		Use:   "svg [bnf dir] [svg dir]",
		Short: "Generate SVG diagrams from SQL grammar",
		Long:  `With no arguments, generates SQL diagrams for all statements.`,
		Args:  cobra.ExactArgs(2),
		Run:   runSVG,
	}
	cmdSVG.Flags().IntVar(&maxWorkers, "max-workers", 1, "maximum number of concurrent workers")
	cmdSVG.Flags().StringVar(&spec, "spec", "", "Location of spec.json file. Can also specify an http address.")

	cmds = append(cmds, cmdBNF, cmdSVG)
}

func runBnf(cmd *cobra.Command, args []string) {
	bnfDir := args[0]

	specs, err := loadSpec()
	if err != nil {
		log.Fatal(err)
	}

	tmpFile, err := preprocess(addr)
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(tmpFile)

	bnf, err := extract.GenerateBNF(tmpFile)
	if err != nil {
		log.Fatal(err)
	}

	br := func() io.Reader {
		return bytes.NewReader(bnf)
	}

	filterRE := regexp.MustCompile(filter)

	if filterRE.MatchString(topStmt) != invertMatch {
		name := topStmt
		if !quiet {
			fmt.Println("processing", name)
		}
		g, err := runParse(br(), nil, name, true, true, nil, nil)
		if err != nil {
			log.Fatalf("%s: %+v", name, err)
		}
		write(filepath.Join(bnfDir, name+".bnf"), g)
	}

	for _, s := range specs {
		if filterRE.MatchString(s.Name) == invertMatch {
			continue
		}
		if !quiet {
			fmt.Println("processing", s.Name)
		}
		if s.Stmt == "" {
			s.Stmt = s.Name
		}
		g, err := runParse(br(), s.Inline, s.Stmt, false, s.Nosplit, s.Match, s.Exclude)
		if err != nil {
			log.Fatalf("%s: %+v", s.Name, err)
		}
		if !quiet {
			fmt.Printf("raw data:\n%s\n", string(g))
		}
		replacements := make([]string, 0, len(s.Replace))
		for from := range s.Replace {
			replacements = append(replacements, from)
		}
		sort.Strings(replacements)
		for _, from := range replacements {
			if !quiet {
				fmt.Printf("replacing: %q -> %q\n", from, s.Replace[from])
			}
			g = bytes.Replace(g, []byte(from), []byte(s.Replace[from]), -1)
		}
		replacements = replacements[:0]
		for from := range s.RegReplace {
			replacements = append(replacements, from)
		}
		sort.Strings(replacements)
		for _, from := range replacements {
			if !quiet {
				fmt.Printf("replacing re: %q -> %q\n", from, s.Replace[from])
			}
			re := regexp.MustCompile(from)
			g = re.ReplaceAll(g, []byte(s.RegReplace[from]))
		}
		if !quiet {
			fmt.Printf("result:\n%s\n", string(g))
		}
		write(filepath.Join(bnfDir, s.Name+".bnf"), g)
	}
}

func runSVG(cmd *cobra.Command, args []string) {
	bnfDir := args[0]
	svgDir := args[1]

	filterRE := regexp.MustCompile(filter)
	stripRE := regexp.MustCompile("\n(\n| )+")

	matches, err := filepath.Glob(filepath.Join(bnfDir, "*.bnf"))
	if err != nil {
		log.Fatal(err)
	}

	specs, err := loadSpec()
	if err != nil {
		log.Fatal(err)
	}
	specMap := make(map[string]stmtSpec)
	for _, s := range specs {
		specMap[s.Name] = s
	}
	if len(specs) != len(specMap) {
		log.Fatal("duplicate spec name")
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxWorkers) // max number of concurrent workers
	for _, m := range matches {
		name := strings.TrimSuffix(filepath.Base(m), ".bnf")
		if filterRE.MatchString(name) == invertMatch {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(m, name string) {
			defer wg.Done()
			defer func() { <-sem }()

			if !quiet {
				fmt.Printf("generating svg of %s (%s)\n", name, m)
			}

			f, err := os.Open(m)
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()

			rr, err := runRR(f)
			if err != nil {
				log.Fatalf("%s: %s\n", m, err)
			}

			var body string
			if strings.HasSuffix(m, topStmt+".bnf") {
				body, err = extract.InnerTag(bytes.NewReader(rr), "body")
				body = strings.SplitN(body, "<hr/>", 2)[0]
				body += `<p>generated by <a href="http://www.bottlecaps.de/rr/ui" data-proofer-ignore>Railroad Diagram Generator</a></p>`
				body = fmt.Sprintf("<div>%s</div>", body)
				if err != nil {
					log.Fatal(err)
				}
			} else {
				s, ok := specMap[name]
				if !ok {
					log.Fatalf("unfound spec: %s", name)
				}
				body, err = extract.Tag(bytes.NewReader(rr), "svg")
				if err != nil {
					log.Fatal(err)
				}
				body = strings.Replace(body, `<a xlink:href="#`, `<a xlink:href="sql-grammar.html#`, -1)
				for _, u := range s.Unlink {
					s := fmt.Sprintf(`<a xlink:href="sqlgrammar.html#%s" xlink:title="%s">((?s).*?)</a>`, u, u)
					link := regexp.MustCompile(s)
					body = link.ReplaceAllString(body, "$1")
				}
				for from, to := range s.Relink {
					replaceFrom := fmt.Sprintf(`<a xlink:href="sqlgrammar.html#%s" xlink:title="%s">`, from, from)
					replaceTo := fmt.Sprintf(`<a xlink:href="sqlgrammar.html#%s" xlink:title="%s">`, to, to)
					body = strings.Replace(body, replaceFrom, replaceTo, -1)
				}
				body = fmt.Sprintf(`<div>%s</div>`, body)
				body = stripRE.ReplaceAllString(body, "\n") + "\n"
			}
			write(filepath.Join(svgDir, name+".html"), []byte(body))
		}(m, name)
	}
	wg.Wait()
}

func runRR(r io.Reader) ([]byte, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var html []byte
	html, err = extract.GenerateRRNet(b)
	if err != nil {
		return nil, err
	}
	s, err := extract.XHTMLtoHTML(bytes.NewReader(html))
	return []byte(s), err
}

func loadSpec() ([]stmtSpec, error) {
	if len(spec) == 0 {
		return nil, nil
	}
	var specs []stmtSpec
	b, err := loadFromResource(spec)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, &specs)
	if err != nil {
		return nil, err
	}
	return specs, nil
}

func loadFromResource(addr string) ([]byte, error) {
	var b []byte
	if strings.HasPrefix(addr, "http") {
		resp, err := http.Get(addr)
		if err != nil {
			return nil, err
		}
		b, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		resp.Body.Close()
	} else {
		body, err := ioutil.ReadFile(addr)
		if err != nil {
			return nil, err
		}
		b = body
	}
	return b, nil
}

func preprocess(addr string) (string, error) {
	b, err := loadFromResource(addr)
	if err != nil {
		return "", err
	}
	input := string(b)

	// remove all content before "Start:"
	input = input[strings.Index(input, "Start:"):]

	// 1st pass handle double-quota operator
	replace1 := strings.NewReplacer(
		`"="`, `'='`,
		`">="`, `'>='`,
		`"<="`, `'<='`,
		`"<>"`, `'<>'`,
		`"<=>"`, `'<=>'`,
		`"<<"`, `'<<'`,
		`">>"`, `'>>'`,
		`"!="`, `'!='`,
		`&&`, `'&&'`,
	)
	input = replace1.Replace(input)

	// 2nd pass fix more mismatch grammar.
	replacer2 := strings.NewReplacer(
		"\"", "",
		"GeneratedAlways:\n\n|", "GeneratedAlways: ",
		"EnforcedOrNotOrNotNullOpt:\n	//	 This branch is needed to workaround the need of a lookahead of 2 for the "+
			"grammar:\n	//\n	//	  { [NOT] NULL | CHECK(...) [NOT] ENFORCED } ...", "EnforcedOrNotOrNotNullOpt:",
		`| CHECK`, "CHECK",
	)

	tmpFile := filepath.Join(os.TempDir(), "sqlgram.tmp.bnf")
	write(tmpFile, []byte(replacer2.Replace(input)))
	return tmpFile, nil
}

func runParse(
	r io.Reader,
	inline []string,
	topStmt string,
	descend, nosplit bool,
	match, exclude []*regexp.Regexp,
) ([]byte, error) {
	g, err := extract.ParseGrammar(r)
	if err != nil {
		return nil, errors.Wrap(err, "parse grammar")
	}
	if err := g.Inline(inline...); err != nil {
		return nil, errors.Wrap(err, "inline")
	}
	b, err := g.ExtractProduction(topStmt, descend, nosplit, match, exclude)
	b = bytes.Replace(b, []byte("IDENT"), []byte("identifier"), -1)
	b = bytes.Replace(b, []byte("_LA"), []byte(""), -1)
	return b, err
}

func write(name string, data []byte) {
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		log.Fatal(err)
	}
	if err := ioutil.WriteFile(name, data, 0644); err != nil {
		log.Fatal(err)
	}
}

func main() {
	rootCmd := func() *cobra.Command {
		cmd := &cobra.Command{
			Use:   "sg",
			Short: "generate bnf and sqlgram",
		}
		cmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress output where possible")
		cmd.AddCommand(cmds...)
		return cmd
	}()
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
