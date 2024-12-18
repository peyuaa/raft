package cluster

import (
	"context"
	"slices"

	"golang.org/x/sync/errgroup"

	"github.com/peyuaa/raft/internal/node"
)

type Cluster struct {
	Nodes []*node.Node
}

func New(n int) (*Cluster, error) {
	nodes := make([]*node.Node, n)
	for i := range n {
		nodes[i] = node.NewNode(slices.Values(nodes[:i]))
		for _, nd := range nodes[:i] {
			if err := nd.Add(nodes[i]); err != nil {
				return nil, err
			}
		}
	}
	return &Cluster{nodes}, nil
}

func (c *Cluster) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)
	for _, n := range c.Nodes {
		g.Go(func() error {
			return n.Run(ctx)
		})
	}
	return g.Wait()
}

func (c *Cluster) Node(id node.ID) *node.Node {
	for _, n := range c.Nodes {
		if n.Id == id {
			return n
		}
	}
	return nil
}
