package variantregistry

import (
	"fmt"

	v1 "github.com/openshift/sippy/pkg/apis/config/v1"
	sippyv1 "github.com/openshift/sippy/pkg/apis/sippy/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// BuildSyntheticReleaseJobOverrides builds a map of job names to release names for all
// jobs explicitly listed in synthetic releases. This map is used to give
// synthetic releases priority over name-based version matching when determining
// which release a job belongs to.
//
// releaseConfigs identifies which releases are synthetic (from BigQuery),
// while the job-to-release mappings come from the YAML config.
func BuildSyntheticReleaseJobOverrides(releases map[string]v1.ReleaseConfig, releaseConfigs []sippyv1.Release) (map[string]string, error) {
	syntheticSet := syntheticReleaseNames(releaseConfigs)
	overrides := map[string]string{}
	for releaseName, releaseCfg := range releases {
		if !syntheticSet.Has(releaseName) {
			continue
		}
		for jobName := range releaseCfg.Jobs {
			if existing, conflict := overrides[jobName]; conflict {
				return nil, fmt.Errorf(
					"job %q is claimed by synthetic releases %q and %q",
					jobName, existing, releaseName,
				)
			}
			overrides[jobName] = releaseName
		}
	}
	return overrides, nil
}

func syntheticReleaseNames(releaseConfigs []sippyv1.Release) sets.Set[string] {
	names := sets.New[string]()
	for _, rc := range releaseConfigs {
		if rc.Synthetic {
			names.Insert(rc.Release)
		}
	}
	return names
}
