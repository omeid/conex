package conex

import (
	"context"
	"fmt"
	"testing"

	docker "github.com/fsouza/go-dockerclient"
)

func init() {
	var _ Container = (*container)(nil)
}

type container struct {
	j *docker.Container
	c *docker.Client

	t testing.TB
}

func (c *container) ID() string {
	return c.j.ID
}

func (c *container) Image() string {
	return c.j.Image
}

func (c *container) Name() string {
	return c.j.Name
}

func (c *container) Address() string {
	return c.j.NetworkSettings.IPAddress
}

func (c *container) Ports() []string {
	// return c.j.NetworkSettings.Ports
	return nil
}

func (c *container) Drop() {
	err := c.c.StopContainer(c.j.ID, 10)

	if err != nil {
		fmt.Println("failed ", c.j.ID)
		c.t.Fatal(err)
	}

	err = c.c.RemoveContainer(docker.RemoveContainerOptions{
		ID:            c.j.ID,
		RemoveVolumes: true,
		Force:         true,
		Context:       context.Background(),
	})
	if err != nil {
		c.t.Fatal(err)
	}

}
