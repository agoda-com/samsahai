package config

import s2hv1 "github.com/agoda-com/samsahai/api/v1"

func Convert(dependency *s2hv1.Dependency,
	parent *s2hv1.Component,
) *s2hv1.Component {
	if dependency == nil {
		return nil
	}

	c := &s2hv1.Component{
		Name:      dependency.Name,
		Image:     dependency.Image,
		Source:    dependency.Source,
		Schedules: dependency.Schedules,
	}

	if parent != nil {
		c.Parent = parent.Name
	}

	return c
}
