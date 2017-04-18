package conex

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
)

func init() {
	var _ Container = (*container)(nil)
}

type container struct {
	j types.ContainerJSON
	t *testing.T
	c *docker.Client
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
	return nil
}

func (c *container) Drop() {
	err := c.c.ContainerStop(context.Background(), c.j.ID, nil)

	if err != nil {
		c.t.Fatal(err)
	}

	err = c.c.ContainerRemove(context.Background(), c.j.ID, types.ContainerRemoveOptions{})
	if err != nil {
		c.t.Fatal(err)
	}

}
