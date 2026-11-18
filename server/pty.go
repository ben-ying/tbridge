package main

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

type windowSize struct{ Rows, Cols, X, Y uint16 }

func ioctl(fd uintptr, req uintptr, p unsafe.Pointer) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(p))
	if e != 0 {
		return e
	}
	return nil
}
func resizePTY(f *os.File, cols, rows int) error {
	if cols < 2 || cols > 500 || rows < 2 || rows > 300 {
		return errors.New("terminal size outside 2..500 columns / 2..300 rows")
	}
	w := windowSize{Rows: uint16(rows), Cols: uint16(cols)}
	return ioctl(f.Fd(), windowSizeRequest, unsafe.Pointer(&w))
}
func startPTY(cmd *exec.Cmd, cols, rows int) (*os.File, error) {
	m, s, err := openPTY()
	if err != nil {
		return nil, err
	}
	defer s.Close()
	if err = resizePTY(m, cols, rows); err != nil {
		m.Close()
		return nil, err
	}
	cmd.Stdin = s
	cmd.Stdout = s
	cmd.Stderr = s
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err = cmd.Start(); err != nil {
		m.Close()
		return nil, err
	}
	return m, nil
}
