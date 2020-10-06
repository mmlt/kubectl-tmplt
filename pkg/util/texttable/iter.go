package texttable

func (t *Table) RowIter() rowIter {
	m := make(map[string]int, len(t.Header))
	for i, p := range t.Header {
		m[p.Text] = i
	}
	return rowIter{
		table:  t,
		index:  m,
		rowIdx: -1,
	}
}

func (ri *rowIter) Next() bool {
	if ri.rowIdx >= len(ri.table.Rows)-1 {
		return false
	}
	ri.rowIdx++
	return true
}

func (ri *rowIter) Row() []string {
	return ri.table.Rows[ri.rowIdx]
}

func (ri *rowIter) GetColByName(n string) (string, bool) {
	i, ok := ri.index[n]
	if !ok {
		return "", false
	}

	return ri.Row()[i], true
}

type rowIter struct {
	table  *Table
	index  map[string]int
	rowIdx int
}
