package conex

import (
	"errors"
	"fmt"
	"net"
	"time"

	docker "github.com/fsouza/go-dockerclient"
)

// ErrPortWaitTimedOut is returned when Container.Wait reaches maxWait before the
// port accepts connections.
var ErrPortWaitTimedOut = errors.New("waiting on port timedout")

func wait(host string, port string, maxWait time.Duration) error {

	portset := docker.Port(port)

	timeout := time.After(maxWait)
	tick := time.NewTicker(time.Second)

	defer tick.Stop()

	addr := fmt.Sprintf("%s:%s", host, portset.Port())
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
