package milter

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
)

func TestReadPacketZeroLength(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	go func() {
		c1.Write([]byte{0, 0, 0, 0}) // length = 0
		c1.Close()
	}()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("readPacket panicked on zero-length packet: %v", r)
		}
	}()
	if _, err := readPacket(c2, 0); err == nil {
		t.Fatal("expected an error for zero-length packet")
	}
}

func TestReadPacketOversize(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	go func() {
		binary.Write(c1, binary.BigEndian, uint32(maxPacketSize+1))
		c1.Close()
	}()

	want := fmt.Sprintf("milter: packet length %d exceeds %d bytes", maxPacketSize+1, maxPacketSize)
	if _, err := readPacket(c2, 0); err == nil || err.Error() != want {
		t.Fatalf("Expected packet length error %q, got %v", want, err)
	}
}

func TestReadPacketSizeBoundary(t *testing.T) {
	for _, size := range []int{1, maxPacketSize} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			c1, c2 := net.Pipe()
			defer c1.Close()
			defer c2.Close()

			msg := &Message{Code: byte(CodeBody), Data: bytes.Repeat([]byte{'A'}, size-1)}
			writeErr := make(chan error, 1)
			go func() {
				writeErr <- writePacket(c1, msg, 0)
			}()

			got, err := readPacket(c2, 0)
			if err != nil {
				t.Fatal("Unexpected error:", err)
			}
			if err := <-writeErr; err != nil {
				t.Fatal("Unexpected write error:", err)
			}
			if got.Code != msg.Code || !bytes.Equal(got.Data, msg.Data) {
				t.Fatal("Packet content changed")
			}
		})
	}
}
