package valuesutil

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal/util"
)

// GetStableComponentsMap returns map of StableComponents in the namespace
func GetStableComponentsMap(c client.Client, namespace string) (stableMap map[string]s2hv1.StableComponent, err error) {
	// get stable
	stableList := &s2hv1.StableComponentList{}
	err = c.List(context.Background(), stableList, &client.ListOptions{Namespace: namespace})
	if err != nil {
		return
	}

	// create StableComponentMap
	stableMap = map[string]s2hv1.StableComponent{}

	for _, stable := range stableList.Items {
		stable := stable
		stableMap[stable.Name] = stable
	}

	return
}

// GenStableComponentValues returns Values of the component combine with stable version of itself and its dependencies
func GenStableComponentValues(
	comp *s2hv1.Component,
	stableMap map[string]s2hv1.StableComponent,
	baseValues map[string]interface{},
) s2hv1.ComponentValues {
	var values map[string]interface{}
	if len(baseValues) > 0 {
		values = util.CopyMap(baseValues)
		values = MergeValues(values, comp.Values)
	} else {
		values = util.CopyMap(comp.Values)
	}

	// merge with StableComponent
	if stableComp, exist := stableMap[comp.Name]; exist {
		values = MergeValues(values, genCompValueFromStableComponent(&stableComp))
	}

	// merge dependencies with StableComponent
	for _, dep := range comp.Dependencies {
		if stableComp, exist := stableMap[dep.Name]; exist {
			values = MergeValues(values, map[string]interface{}{
				dep.Name: genCompValueFromStableComponent(&stableComp),
			})
		}
	}

	return values
}

func genCompValueFromStableComponent(stableComp *s2hv1.StableComponent) map[string]interface{} {
	return map[string]interface{}{
		"image": map[string]interface{}{
			"repository": stableComp.Spec.Repository,
			"tag":        stableComp.Spec.Version,
		},
	}
}

// MergeValues merges source and destination map, preferring values from the source map
func MergeValues(base map[string]interface{}, target map[string]interface{}) map[string]interface{} {
	for k, v := range target {
		// If the key doesn't exist already, then just set the key to that value
		if _, exists := base[k]; !exists {
			base[k] = v
			continue
		}
		nextMap, ok := v.(map[string]interface{})
		// If it isn't another map, overwrite the value
		if !ok {
			base[k] = v
			continue
		}
		// Edge case: If the key exists in the destination, but isn't a map
		destMap, isMap := base[k].(map[string]interface{})
		// If the source map has a map for this key, prefer it
		if !isMap {
			base[k] = v
			continue
		}
		// If we got to this point, it is a map in both, so merge them
		base[k] = MergeValues(destMap, nextMap)
	}
	return base
}
