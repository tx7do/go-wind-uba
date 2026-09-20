package dorisinit

import (
	"fmt"
	"regexp"
	"strings"
)

// sectionHeader matches the numbered comment headers the ETL script uses to group its
// statements: "-- 1. 填充 sessions_fact", "-- 2.2 对当天活跃用户 …". The separator and the
// whitespace after it are both required so prose bullets such as "-- 1) sessions_fact …"
// are not mistaken for headers.
var sectionHeader = regexp.MustCompile(`^[ \t]*--+[ \t]*(\d+(?:\.\d+)*)[.．、]?[ \t]+(\S*)`)

// Section is a numbered block of a script. Sections let the ETL runner pick a subset of a
// file (only the blocks whose source really feeds a reader) without hardcoding line numbers.
type Section struct {
	// Number is the header as written, e.g. "1" or "2.1". It is "" for the statements
	// that precede the first header.
	Number string
	// Line is the 1-based line of the header that opened this section (0 for the
	// statements before the first header).
	Line  int
	Title string
	// Statements belongs to this section, in source order.
	Statements []Statement
}

// SplitSections groups statements under the numbered header that precedes them. Every
// header yields a section, even when it holds no statement, so a caller can tell "the
// header is gone" apart from "the step is empty".
//
// Headers are recognised line by line in the raw script, so a header written inside a
// string literal would be misread; the shipped scripts contain none.
func SplitSections(src []byte, stmts []Statement) []Section {
	headers := findHeaders(src)

	sections := make([]Section, 0, len(headers)+1)
	sections = append(sections, Section{}) // statements before the first header
	for _, h := range headers {
		sections = append(sections, Section{Number: h.number, Line: h.line, Title: h.title})
	}

	at := 0
	for _, s := range stmts {
		for at < len(headers) && headers[at].line <= s.Line {
			at++
		}
		sections[at].Statements = append(sections[at].Statements, s)
	}
	return sections
}

type header struct {
	line   int
	number string
	title  string
}

func findHeaders(src []byte) []header {
	var out []header
	for i, l := range strings.Split(string(src), "\n") {
		m := sectionHeader.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		out = append(out, header{line: i + 1, number: m[1], title: strings.TrimSpace(m[2])})
	}
	return out
}

// SelectSections returns the sections belonging to the given top-level numbers, keeping
// source order. A sub-section header such as "2.2" joins its parent "2", which is what the
// ETL needs: a step's "SET … = true" must stay on the same connection as the INSERT it
// governs.
//
// Only the requested numbers are returned, so the preamble before the first header (which
// in 06_etl.sql holds "USE gw_uba;") is dropped on purpose — the connection is already
// scoped by the DSN.
func SelectSections(sections []Section, numbers ...string) ([]Section, error) {
	want := make(map[string]struct{}, len(numbers))
	for _, n := range numbers {
		want[n] = struct{}{}
	}

	var out []Section
	for _, s := range sections {
		top, _, _ := strings.Cut(s.Number, ".")
		if _, ok := want[top]; !ok {
			continue
		}
		if top == s.Number {
			for _, o := range out {
				if o.Number == top {
					return nil, fmt.Errorf("section %q is declared twice (line %d and line %d): cannot tell which statements belong to it",
						top, o.Line, s.Line)
				}
			}
		}
		out = append(out, s)
	}

	for _, n := range numbers {
		found := false
		for _, s := range out {
			if top, _, _ := strings.Cut(s.Number, "."); top == n {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("section %q not found", n)
		}
	}
	return out, nil
}
