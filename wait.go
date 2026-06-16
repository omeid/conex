package conex

import (
	"errors"
	"net"
	"time"

	"github.com/docker/go-connections/nat"
)

// ErrPortWaitTimedOut is returned when Container.Wait reaches maxWait before the
// port accepts connections.
var ErrPortWaitTimedOut = errors.New("wait timeout")

func wait(host string, port string, maxWait time.Duration) error {

	portset := nat.Port(port)

	timeout := time.After(maxWait)
	tick := time.NewTicker(time.Second)

	defer tick.Stop()

	addr := net.JoinHostPort(host, portset.Port())
	for {

		select {

		case <-timeout:
			return ErrPortWaitTimedOut

		case <-tick.C:
			conn, err := net.Dial(portset.Proto(), addr)
			if err == nil {
				return conn.Close()
			}

		}
	}
}
