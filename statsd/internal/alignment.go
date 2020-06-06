package internal

import (
	"fmt"
	"io"
)

// FieldOffset is a preprocessor representation of a struct field alignment.
type FieldOffset struct {
	// Name of the field.
	Name string

	// Offset of the field in bytes.
	//
	// To compute this at compile time use unsafe.Offsetof.
	Offset uintptr
}

// Aligned8Byte returns if all fields are aligned modulo 8-bytes.
//
// Error messaging is printed to out for any fileds determined misaligned.
func Aligned8Byte(fields []FieldOffset, out io.Writer) bool {
	misaligned := make([]FieldOffset, 0)
	for _, f := range fields {
		if f.Offset%8 != 0 {
			misaligned = append(misaligned, f)
		}
	}

	if len(misaligned) == 0 {
		return true
	}

	fmt.Fprintln(out, "struct fields not aligned for 64-bit atomic operations:")
	for _, f := range misaligned {
		fmt.Fprintf(out, "  %s: %d-byte offset\n", f.Name, f.Offset)
	}

	return false
}

