package roadmap

import "github.com/rierek/lif/pkg/item"

// Node is one roadmap row and the rows nested under it.
type Node struct {
	Item     *item.Item
	Children []*Node
}

// Tree returns the roadmap as nodes, in render order: the order of the CLI's
// roadmap rows. A scoped roadmap (Root set) yields its single root node in
// scheduled and nothing in unscheduled.
func (d *Data) Tree() (scheduled, unscheduled []*Node) {
	if d.Root != nil {
		if d.Root.Epic != nil {
			scheduled = append(scheduled, epicNode(*d.Root.Epic))
		}
		if d.Root.Feature != nil {
			scheduled = append(scheduled, featureNode(*d.Root.Feature))
		}
		return scheduled, nil
	}

	for _, mg := range d.Milestones {
		if mg.Milestone == nil {
			continue
		}
		n := &Node{Item: mg.Milestone}
		n.Children = containerNodes(mg.Containers)
		n.Children = append(n.Children, leafNodes(mg.Other)...)
		scheduled = append(scheduled, n)
	}

	if d.Unscheduled != nil {
		unscheduled = containerNodes(d.Unscheduled.Containers)
		unscheduled = append(unscheduled, leafNodes(d.Unscheduled.Other)...)
	}
	return scheduled, unscheduled
}

func containerNodes(cs []Container) []*Node {
	var out []*Node
	for _, c := range cs {
		switch {
		case c.Epic != nil:
			out = append(out, epicNode(*c.Epic))
		case c.Feature != nil:
			out = append(out, featureNode(*c.Feature))
		}
	}
	return out
}

func epicNode(eg EpicGroup) *Node {
	n := &Node{Item: eg.Epic}
	n.Children = leafNodes(eg.Items)
	for _, fg := range eg.Features {
		n.Children = append(n.Children, featureNode(fg))
	}
	return n
}

func featureNode(fg FeatureGroup) *Node {
	return &Node{Item: fg.Feature, Children: leafNodes(fg.Items)}
}

func leafNodes(items []*item.Item) []*Node {
	var out []*Node
	for _, it := range items {
		out = append(out, &Node{Item: it})
	}
	return out
}
