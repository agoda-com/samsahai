package config

import s2hv1 "github.com/agoda-com/samsahai/api/v1"

func New(dependency *s2hv1.Dependency,
	parent *s2hv1.Component,
) *s2hv1.Component {
	if dependency == nil {
		return nil
	}

	c := &s2hv1.Component{
		Name:   dependency.Name,
		Image:  dependency.Image,
		Source: dependency.Source,
	}

	if parent != nil {
		c.Parent = parent.Name
	}

	return c
}
