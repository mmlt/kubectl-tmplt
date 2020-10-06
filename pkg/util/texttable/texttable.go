package texttable

import (
	"bufio"
	"io"
	"strings"
)

type Table struct {
	Header Header
	Rows   [][]string
}

type Header []Pos

type Pos struct {
	Text string
	Col  int
}

func (t *Table) ColCnt() int {
	return len(t.Header)
}

func (t *Table) RowCnt() int {
	return len(t.Rows)
}

// Read reads a text formatted table.
// The input text is expected to have headers.
//   minCols set the minimum number of columns for a rows to be included in the result.
func Read(r io.Reader, minCols int) *Table {
	scanner := bufio.NewScanner(r)

	// scan Header
	ok := scanner.Scan()
	if !ok {
		return &Table{}
	}
	hdr := scanHeader([]byte(scanner.Text()))

	answer := &Table{Header: hdr}
	for scanner.Scan() {
		ln := []byte(scanner.Text())
		if len(ln) == 0 {
			// stop on first empty line
			break
		}

		strt := 0
		var row []string
		for _, p := range hdr {
			if p.Col+1 >= len(ln) {
				break
			}

			var s []byte
			if p.Col == -1 {
				s = ln[strt:]
			} else {
				s = ln[strt:p.Col]
			}

			row = append(row, strings.TrimSpace(string(s)))
			strt = p.Col
		}
		if len(row) >= minCols {
			answer.Rows = append(answer.Rows, row)
		}
	}

	return answer
}

func scanHeader(s []byte) Header {
	var isSpace bool
	var answer Header

	start := 0
	for i := start; i < len(s); i++ {
		if isSpace && !isWhitepace(s[i]) {
			// right side of column found
			t := strings.TrimSpace(string(s[start:i]))
			answer = append(answer, Pos{Text: t, Col: i - 1})
			start = i
		}
		isSpace = isWhitepace(s[i])
	}
	answer = append(answer, Pos{Text: strings.TrimSpace(string(s[start:])), Col: -1})

	return answer
}

func isWhitepace(b byte) bool {
	return b == 32
}

// Write writes a table formatted as text.
//  columns selects the columns to write.
//  colSep is the columns separator string.
//  hdr includes a header.
func Write(table *Table, columns []int, colSep string, hdr bool, output io.Writer) {
	if len(columns) == 0 {
		// default is all columns
		for i, _ := range table.Header {
			columns = append(columns, i)
		}
	}

	colWidth := colWidth(table)

	last := len(columns)

	if hdr {
		// output Header
		for i, ci := range columns {
			s := table.Header[ci].Text
			io.WriteString(output, s)
			if i < last-1 {
				pad := colWidth[ci] - len(s)
				io.WriteString(output, strings.Repeat(" ", pad))
				io.WriteString(output, colSep)
			}
		}
		io.WriteString(output, "\n")
	}

	// output Rows
	for _, row := range table.Rows {
		for i, ci := range columns {
			s := row[ci]
			io.WriteString(output, s)
			if i < last-1 {
				pad := colWidth[ci] - len(s)
				io.WriteString(output, strings.Repeat(" ", pad))
				io.WriteString(output, colSep)
			}
		}
		io.WriteString(output, "\n")
	}
}

func colWidth(table *Table) []int {
	var result []int

	for ci, _ := range table.Header {
		w := len(table.Header[ci].Text)
		for _, r := range table.Rows {
			w = max(w, len(r[ci]))
		}
		result = append(result, w)
	}

	return result
}

func max(a, b int) (c int) {
	c = b
	if a > b {
		c = a
	}
	return
}
