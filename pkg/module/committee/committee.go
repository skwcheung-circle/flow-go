// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package committee

import (
	"regexp"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/pkg/module/function"
	"github.com/dapperlabs/flow-go/pkg/module/model"
)

// Committee represents the table of staked nodes in the flow system.
type Committee struct {
	me    *model.Node
	nodes map[string]*model.Node
}

// New generates a new committee of consensus nodes.
func New(entries []string, identity string) (*Committee, error) {

	// create committee with node table
	c := &Committee{
		nodes: make(map[string]*model.Node),
	}

	// try to parse the nodes
	rx := regexp.MustCompile(`^(\w+)-(\w+)@([\w\.]+:\d{1,5})$`)
	for _, entry := range entries {

		// try to parse the expression
		fields := rx.FindStringSubmatch(entry)
		if fields == nil {
			return nil, errors.Errorf("invalid node entry (%s)", entry)
		}

		// check for duplicates
		id := fields[2]
		_, ok := c.nodes[id]
		if ok {
			return nil, errors.Errorf("duplicate node identity (%s)", id)
		}

		// add entry to identity table
		node := &model.Node{
			ID:      fields[2],
			Role:    fields[1],
			Address: fields[3],
		}
		c.nodes[id] = node
	}

	// check if we are present in the node identity table
	me, ok := c.nodes[identity]
	if !ok {
		return nil, errors.Errorf("entry for own identity missing (%s)", identity)
	}
	c.me = me

	return c, nil
}

// Me returns our own node identity.
func (c *Committee) Me() *model.Node {
	return c.me
}

// Get will return the node with the given ID.
func (c *Committee) Get(id string) (*model.Node, error) {
	node, ok := c.nodes[id]
	if !ok {
		return nil, errors.New("node not found")
	}
	return node, nil
}

// Select will returns the nodes fulfilling the given filters.
func (c *Committee) Select(filters ...function.NodeFilter) (model.NodeList, error) {
	var nodes model.NodeList
Outer:
	for _, node := range c.nodes {
		for _, filter := range filters {
			if !filter(node) {
				continue Outer
			}
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}
