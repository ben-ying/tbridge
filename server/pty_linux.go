//go:build linux

package main

import (
	"fmt"
	"os"
	"unsafe"
)

const windowSizeRequest = 0x5414

func openPTY() (*os.File, *os.File, error) {
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR|0x100, 0)
	if err != nil {
		return nil, nil, err
	}
	var unlock int32
	if err = ioctl(m.Fd(), 0x40045431, unsafe.Pointer(&unlock)); err != nil {
		m.Close()
		return nil, nil, err
	}
	var n uint32
	if err = ioctl(m.Fd(), 0x80045430, unsafe.Pointer(&n)); err != nil {
		m.Close()
		return nil, nil, err
	}
	s, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|0x100, 0)
	if err != nil {
		m.Close()
		return nil, nil, err
	}
	return m, s, nil
}
