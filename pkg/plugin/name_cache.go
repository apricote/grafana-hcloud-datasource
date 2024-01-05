package plugin

import (
	"context"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"sync"
)

type HCloudResource interface {
	hcloud.Server | hcloud.LoadBalancer
}

type GetResourceFn[R HCloudResource] func(ctx context.Context, id int64) (*R, error)
type IdentifierFn[R HCloudResource] func(resource *R) (int64, string)

func NewNameCache[R HCloudResource](client *hcloud.Client, getFn GetResourceFn[R], identifierFn IdentifierFn[R]) *NameCache[R] {
	return &NameCache[R]{
		client:       client,
		getFn:        getFn,
		identifierFn: identifierFn,

		cache: map[int64]string{},
	}
}

type NameCache[R HCloudResource] struct {
	client       *hcloud.Client
	getFn        GetResourceFn[R]
	identifierFn IdentifierFn[R]

	cache map[int64]string
	sync.Mutex
}

func (c *NameCache[R]) Get(ctx context.Context, id int64) (string, error) {
	c.Lock()
	defer c.Unlock()
	name, ok := c.cache[id]
	if ok {
		return name, nil
	}

	resource, err := c.getFn(ctx, id)
	if err != nil {
		return "", err
	}
	_, c.cache[id] = c.identifierFn(resource)

	return c.cache[id], nil
}

func (c *NameCache[R]) Insert(resources ...*R) {
	c.Lock()
	defer c.Unlock()

	for _, resource := range resources {
		id, name := c.identifierFn(resource)
		c.cache[id] = name
	}
}
