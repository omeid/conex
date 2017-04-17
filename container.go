package conex

import (
	"github.com/docker/docker/api/types"
)

func init() {
	var _ Container = (*container)(nil)
}

type container struct {
	c types.ContainerJSON
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
