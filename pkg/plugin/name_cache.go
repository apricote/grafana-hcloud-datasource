package plugin

import (
	"context"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"sync"
)

func NewNameCache(client *hcloud.Client) *NameCache {
	return &NameCache{
		client: client,
		cache:  map[int64]string{},
	}
}

type NameCache struct {
	client *hcloud.Client
	cache  map[int64]string
	sync.Mutex
}

func (c *NameCache) Get(ctx context.Context, id int64) (string, error) {
	c.Lock()
	defer c.Unlock()
	name, ok := c.cache[id]
	if ok {
		return name, nil
	}

	server, _, err := c.client.Server.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	c.cache[id] = server.Name
	return server.Name, nil
}

func (c *NameCache) Insert(servers ...*hcloud.Server) {
	c.Lock()
	defer c.Unlock()

	for _, server := range servers {
		c.cache[server.ID] = server.Name
	}
}
