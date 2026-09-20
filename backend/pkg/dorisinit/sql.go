// Package dorisinit turns the Doris provisioning scripts in backend/sql/doris into an
// idempotent, one-shot apply: cut a script into statements, render its parameters, and
// ensure the Kafka Routine Load jobs it declares without ever dropping an existing job.
//
// The package deliberately depends on nothing but the standard library: the SQL is lexed
// with a state machine instead of a regex, and database access goes through the minimal
// Conn/Source interfaces in exec.go so the logic stays testable without a cluster.
package dorisinit

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"
)

// utf8BOM is a UTF-8 byte-order mark. Placed before the leading "--" of a script it turns
// the first comment marker into garbage and the whole file fails to parse, so every loader
// in this package strips it defensively.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// StripBOM removes a leading UTF-8 byte-order mark, if present.
func StripBOM(src []byte) []byte {
	return bytes.TrimPrefix(src, utf8BOM)
}

// Statement is one top-level SQL statement cut out of a script.
type Statement struct {
	// Line is the 1-based line holding the statement's first code byte.
	Line int
	// Text is the statement verbatim, comments included, without the trailing ';'.
	Text string
	// Code is Text with every comment removed and whitespace collapsed. Use it to
	// recognise a statement; use Text when sending it to the server, so that optimizer
	// hints written as /*+ ... */ survive.
	Code string
}

// FirstWord returns the leading keyword of the statement's code. A statement that starts
// with an opening paren is measured from inside it, so a wrapped statement is still
// recognised by its keyword.
func (s Statement) FirstWord() string {
	fields := strings.Fields(strings.TrimLeft(s.Code, "( \t\n"))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// SplitStatements cuts a script into the statements terminated by ';'.
//
// Quoted text ('..', "..", `..`), '--'/'#' line comments and /* */ block comments are all
// honoured, so a ';' inside a literal never splits a statement — the jsonpaths literal in
// 02_kafka_tables.sql is dense with escaped quotes and cannot be handled by a regex.
// Following MySQL, '--' only opens a comment when whitespace follows it, so "a--b" stays
// two minus operators. Comment-only runs produce no statement.
//
// Caveats: '#'-style comments and /*! ... */ version comments are treated as plain
// comments, and DELIMITER (which this splitter cannot honour) is rejected outright.
func SplitStatements(src []byte) ([]Statement, error) {
	var (
		stmts  []Statement
		code   strings.Builder
		line   = 1
		start  = -1 // byte offset of the current statement's first code byte
		sLine  = 0
		needSp bool // a separator was skipped; emit one space before the next token
	)

	begin := func(i int) {
		if start < 0 {
			start, sLine = i, line
		}
	}

	writeCode := func(word string) {
		if needSp && code.Len() > 0 {
			code.WriteByte(' ')
		}
		code.WriteString(word)
		needSp = false
	}

	flush := func(end int) error {
		if start < 0 {
			// A ';' with no code before it: an empty statement, nothing to run.
			code.Reset()
			needSp = false
			return nil
		}
		stmt := Statement{
			Line: sLine,
			Text: strings.TrimSpace(string(src[start:end])),
			Code: strings.TrimSpace(code.String()),
		}
		if strings.EqualFold(stmt.FirstWord(), "DELIMITER") {
			return fmt.Errorf("line %d: DELIMITER is not supported, split the script into files instead", stmt.Line)
		}
		stmts = append(stmts, stmt)
		code.Reset()
		start, sLine = -1, 0
		needSp = false
		return nil
	}

	for i := 0; i < len(src); {
		c, size := utf8.DecodeRune(src[i:])
		if c == utf8.RuneError && size == 1 {
			return nil, fmt.Errorf("line %d: invalid UTF-8 at offset %d", line, i)
		}

		switch {
		case c == '\n':
			line++
			needSp = true
			i++

		case c == ' ' || c == '\t' || c == '\r':
			needSp = true
			i++

		case c == '#' || (c == '-' && at(src, i+1) == '-' && isSpaceOrEnd(src, i+2)):
			needSp = true
			if nl := bytes.IndexByte(src[i:], '\n'); nl >= 0 {
				i += nl
			} else {
				i = len(src)
			}

		case c == '/' && at(src, i+1) == '*':
			needSp = true
			if end := bytes.Index(src[i+2:], []byte("*/")); end >= 0 {
				line += bytes.Count(src[i:i+2+end], []byte{'\n'})
				i += 2 + end + 2
			} else {
				// Unterminated block comment: swallow the rest of the file.
				i = len(src)
			}

		case c == '\'' || c == '"' || c == '`':
			// MySQL honours backslash escapes inside string literals but not inside
			// backtick-quoted identifiers; both allow the delimiter doubled.
			end, err := scanQuoted(src, i, byte(c), c != '`')
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", line, err)
			}
			begin(i)
			writeCode(string(src[i:end]))
			line += bytes.Count(src[i:end], []byte{'\n'})
			i = end

		case c == ';':
			if err := flush(i); err != nil {
				return nil, err
			}
			i++

		default:
			begin(i)
			writeCode(string(src[i : i+size]))
			i += size
		}
	}

	if err := flush(len(src)); err != nil {
		return nil, err
	}
	return stmts, nil
}

// scanQuoted returns the offset just past the string or identifier literal that starts at
// i. escapes selects MySQL's backslash handling, which applies to '..' and ".." but not to
// `..`.
func scanQuoted(src []byte, i int, quote byte, escapes bool) (int, error) {
	for j := i + 1; j < len(src); {
		switch c := src[j]; {
		case escapes && c == '\\':
			j += 2
		case c == quote:
			if j+1 < len(src) && src[j+1] == quote {
				j += 2
				continue
			}
			return j + 1, nil
		default:
			j++
		}
	}
	return 0, fmt.Errorf("unterminated %c literal starting at offset %d", quote, i)
}

// at returns the byte at i, or 0 past the end so callers can probe src[i+1] freely.
func at(src []byte, i int) byte {
	if i >= len(src) {
		return 0
	}
	return src[i]
}

// isSpaceOrEnd reports whether the byte at i is whitespace or the end of input.
func isSpaceOrEnd(src []byte, i int) bool {
	switch at(src, i) {
	case 0, ' ', '\t', '\r', '\n':
		return true
	}
	return false
}
