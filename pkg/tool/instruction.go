package tool

import (
	"fmt"
	"io"
)

//go:generate stringer -type=instrType

type instrType int

const (
	// InstrApply is an instruction to apply content.
	InstrApply instrType = iota
	// InstrWait is an instruction to wait for a condition.
	InstrWait
)

//
type instruction struct {
	// Name is for easy reference.
	typ instrType
	// Input is the string send on stdin.
	input string
	// Args are the arguments passed to kubectl starting with the command (get, delete, apply, wait etc.).
	args []string
	// ID is an unique number to track instructions.
	id int
	// Origin is a non-unique string to track instructions.
	origin string
}

func (instr instruction) name() string {
	return instr.typ.String()
}

func (instr instruction) fprint(w io.Writer) {
	fmt.Fprintln(w, "---")
	fmt.Fprintf(w, "##%02d: %s %s %s\n", instr.id, instr.typ, instr.args, instr.origin)
	fmt.Fprintln(w, instr.input)
}

type instructions struct {
	items []instruction
	count int
}

// Add add a new instruction to the list.
func (instrs *instructions) Add(instr *instruction) {
	instrs.count++
	instr.id = instrs.count
	instrs.items = append(instrs.items, *instr)
}

func (instrs *instructions) fprint(w io.Writer) {
	for _, in := range instrs.items {
		in.fprint(w)
	}
}
