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
	cmds = append(cmds, cmdBNF)
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
