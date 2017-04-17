package conex

import (
	"context"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
)

func init() {
	var _ Container = (*container)(nil)
}

type container struct {
	c types.ContainerJSON

	client *docker.Client
}

func (c *container) ID() string {
	return c.c.ID
}

func (c *container) Image() string {
	return c.c.Image
}

func (c *container) Name() string {
	return c.c.Name
}

func (c *container) Address() string {
	return c.c.NetworkSettings.IPAddress
}

func (c *container) Ports() []string {
	return nil
}

func (c *container) Drop() error {

	err := c.client.ContainerStop(context.Background(), c.c.ID, nil)

	if err != nil {
		return err
	}

	return c.client.ContainerRemove(context.Background(), c.c.ID, types.ContainerRemoveOptions{})

}
