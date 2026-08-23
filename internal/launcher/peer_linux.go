//go:build linux

package launcher

import (
	"fmt"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

func peerUID(conn net.Conn) (int, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, fmt.Errorf("expected Unix connection")
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return 0, err
	}
	var credentials *unix.Ucred
	var controlErr error
	err = raw.Control(func(fd uintptr) {
		credentials, controlErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	})
	if err != nil {
		return 0, err
	}
	if controlErr != nil {
		return 0, controlErr
	}
	if credentials == nil {
		return 0, syscall.EINVAL
	}
	return int(credentials.Uid), nil
}
