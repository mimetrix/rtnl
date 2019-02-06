package rtnl

import (
	"encoding/binary"
)

func htons(val uint16) uint16 {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, val)
	return binary.BigEndian.Uint16(buf)
}

func ntohs(val uint16) uint16 {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, val)
	return binary.LittleEndian.Uint16(buf)
}
