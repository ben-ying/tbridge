//go:build darwin

package main

import (
	"bytes"
	"os"
	"syscall"
	"unsafe"
)

const windowSizeRequest = 0x80087467

func openPTY() (*os.File, *os.File, error) {
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	if err = ioctl(m.Fd(), 0x20007454, nil); err != nil {
		m.Close()
		return nil, nil, err
	}
	if err = ioctl(m.Fd(), 0x20007452, nil); err != nil {
		m.Close()
		return nil, nil, err
	}
	var name [128]byte
	if err = ioctl(m.Fd(), 0x40807453, unsafe.Pointer(&name[0])); err != nil {
		m.Close()
		return nil, nil, err
	}
	s, err := os.OpenFile(string(bytes.TrimRight(name[:], "\x00")), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		m.Close()
		return nil, nil, err
	}
	return m, s, nil
}
