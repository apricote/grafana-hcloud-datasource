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

// NameCache is a cache for resource names. It is used to avoid sending unnecessary API requests. Right now there is no
// expiry for entries, so if names are changed this is not reflected in queries.
type NameCache[R HCloudResource] struct {
	client       *hcloud.Client
	getFn        GetResourceFn[R]
	identifierFn IdentifierFn[R]

	cache map[int64]string
	sync.Mutex
}

// Get will retrieve the name from the cache or query the API in case it is unknown.
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

// Insert will insert the given resources into the cache, updating any existing entries.
// This should be called whenever API requests are made, to keep the cache reasonable full & up to date.
func (c *NameCache[R]) Insert(resources ...*R) {
	c.Lock()
	defer c.Unlock()

	for _, resource := range resources {
		id, name := c.identifierFn(resource)
		c.cache[id] = name
	}
}
