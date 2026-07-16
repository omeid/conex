package conex

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func init() {
	var _ Container = (*dockerContainer)(nil)
}

// dockerContainer implements Container for Docker network-based access.
type dockerContainer struct {
	json     container.InspectResponse
	client   client.APIClient
	t        testing.TB
	name     string
	address  string
	dropOnce sync.Once
}

func (c *dockerContainer) ID() string {
	return c.json.ID
}

func (c *dockerContainer) Image() string {
	return c.json.Config.Image
}

func (c *dockerContainer) Name() string {
	return c.json.Name
}

func (c *dockerContainer) Address() string {
	return c.address
}

func (c *dockerContainer) Drop() {
	c.dropOnce.Do(func() {
		// Try to stop the container, but don't fail if it's already stopped
		timeout := 10
		_, _ = c.client.ContainerStop(context.Background(), c.json.ID, client.ContainerStopOptions{Timeout: &timeout})

		_, err := c.client.ContainerRemove(context.Background(), c.json.ID, client.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		})
		if err != nil {
			c.t.Fatal(err)
		}
	})
}

func (c *dockerContainer) Wait(port string, timeout time.Duration) error {
	err := wait(c.Address(), port, timeout)
	if err != nil && testing.Verbose() {
		Logf(nil, "conex", "=== Container %s Logs ===\n", c.Name())
		_ = c.Logs(os.Stdout, os.Stderr)
		Logf(nil, "conex", "=========================\n")
	}
	return err
}

func (c *dockerContainer) Exec(cmd ...string) *Cmd {
	return newDockerCmd(c.t, c.client, c.json.ID, cmd)
}

// Logs writes the container logs to the provided stdout and stderr writers.
func (c *dockerContainer) Logs(stdout io.Writer, stderr io.Writer) error {
	reader, err := c.client.ContainerLogs(c.t.Context(), c.json.ID, client.ContainerLogsOptions{
		ShowStdout: stdout != nil,
		ShowStderr: stderr != nil,
		Follow:     false,
	})
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	if c.json.Config != nil && c.json.Config.Tty {
		w := stdout
		if w == nil {
			w = stderr
		}
		if w == nil {
			w = io.Discard
		}
		_, err := io.Copy(w, reader)
		return err
	}

	_, err = stdcopy.StdCopy(stdout, stderr, reader)
	return err
}

func newDockerCmd(t testing.TB, cli client.APIClient, containerID string, cmd []string) *Cmd {
	if len(cmd) == 0 {
		return nil
	}

	c := &Cmd{
		Path:   cmd[0],
		Args:   cmd,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}

	var execID string
	errCh := make(chan error, 1)

	c.start = func() error {
		opts := client.ExecCreateOptions{
			Cmd:          c.Args,
			Env:          c.Env,
			WorkingDir:   c.Dir,
			AttachStdin:  c.Stdin != nil,
			AttachStdout: true,
			AttachStderr: true,
		}

		execInfo, err := cli.ExecCreate(t.Context(), containerID, opts)
		if err != nil {
			return err
		}
		execID = execInfo.ID

		go func() {
			resp, err := cli.ExecAttach(t.Context(), execID, client.ExecAttachOptions{
				TTY: false,
			})
			if err != nil {
				errCh <- err
				return
			}
			defer resp.Close()

			if c.Stdin != nil {
				go func() {
					_, _ = io.Copy(resp.Conn, c.Stdin)
					_ = resp.CloseWrite()
				}()
			}

			_, copyErr := stdcopy.StdCopy(c.Stdout, c.Stderr, resp.Reader)
			errCh <- copyErr
		}()
		return nil
	}

	c.wait = func() error {
		err := <-errCh
		if err != nil && err != io.EOF {
			return err
		}
		execInspect, err := cli.ExecInspect(t.Context(), execID, client.ExecInspectOptions{})
		if err != nil {
			return err
		}
		if execInspect.ExitCode != 0 {
			return fmt.Errorf("exit status %d", execInspect.ExitCode)
		}
		return nil
	}

	return c
}
