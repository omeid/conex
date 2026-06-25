package runtime

import (
	"errors"
	"net"
	"time"

	"github.com/docker/go-connections/nat"
)

// ErrPortWaitTimedOut is returned when Container.Wait reaches maxWait before the
// port accepts connections.
var ErrPortWaitTimedOut = errors.New("wait timeout")

// Wait blocks until a connection can be established to the specified ip and port,
// or the timeout is reached.
func Wait(ip string, port string, duration time.Duration) error {

	portset := nat.Port(port)

	timeout := time.After(duration)
	tick := time.NewTicker(time.Second)

	defer tick.Stop()

	addr := net.JoinHostPort(ip, portset.Port())
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
