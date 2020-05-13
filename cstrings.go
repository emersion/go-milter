package milter

import (
	"bytes"
	"strings"
)

// NULL terminator
const null = "\x00"

// DecodeCStrings splits a C style strings into a Go slice
func decodeCStrings(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	return strings.Split(strings.Trim(string(data), null), null)
}

// ReadCString reads and returns a C style string from []byte
func readCString(data []byte) string {
	pos := bytes.IndexByte(data, 0)
	if pos == -1 {
		return string(data)
	}
	return string(data[0:pos])
}

// appendCString appends a C style string to the buffer and returns it (like append does).
func appendCString(dest []byte, s string) []byte {
	dest = append(dest, []byte(s)...)
	dest = append(dest, 0x00)
	return dest
}
