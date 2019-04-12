package comparator_test

import (
	"testing"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	comparator "github.com/agoda-com/samsahai/internal/samsahai/component/comparator"
	. "github.com/onsi/gomega"
)

func TestGetChangedComponents(t *testing.T) {
	g := NewGomegaWithT(t)
	currentComponents := map[string]component.ValuesFile{
		"comp1": {Image: component.Image{Repository: "some-docker/docker", Tag: "1.0.0"}},
		"comp2": {Image: component.Image{Repository: "some-docker/docker2", Tag: "1.0.0"}},
		"comp3": {Image: component.Image{Repository: "some-docker/docker3", Tag: "1.0.0"}},
		"comp4": {Image: component.Image{Repository: "some-docker/docker4", Tag: "1.0.0"}},
	}

	updatedComponents := map[string]component.ValuesFile{
		"comp1": {Image: component.Image{Repository: "any-docker/docker", Tag: "1.0.0"}},
		"comp2": {Image: component.Image{Repository: "some-docker/docker2", Tag: "0.9.9"}},
		"comp3": {Image: component.Image{Repository: "some-docker/docker3", Tag: "1.0.0"}},
		"comp5": {Image: component.Image{Repository: "some-docker/docker5", Tag: "1.0.0"}},
	}

	expectedChangedComponents := map[string]component.ValuesFile{
		"comp1": {Image: component.Image{Repository: "any-docker/docker", Tag: "1.0.0"}},
		"comp2": {Image: component.Image{Repository: "some-docker/docker2", Tag: "0.9.9"}},
		"comp5": {Image: component.Image{Repository: "some-docker/docker5", Tag: "1.0.0"}},
	}

	changedComponents := comparator.GetChangedComponents(updatedComponents, currentComponents)
	g.Expect(changedComponents).Should(Equal(expectedChangedComponents))
}
