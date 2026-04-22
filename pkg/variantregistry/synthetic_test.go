package variantregistry

import (
	"testing"

	v1 "github.com/openshift/sippy/pkg/apis/config/v1"
	sippyv1 "github.com/openshift/sippy/pkg/apis/sippy/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSyntheticReleaseJobOverrides(t *testing.T) {
	tests := []struct {
		name              string
		releases          map[string]v1.ReleaseConfig
		releaseConfigs    []sippyv1.Release
		expectedOverrides map[string]string
		expectError       bool
	}{
		{
			name: "no synthetic releases",
			releases: map[string]v1.ReleaseConfig{
				"4.22": {
					Jobs: map[string]bool{"job-a": true, "job-b": true},
				},
			},
			expectedOverrides: map[string]string{},
		},
		{
			name: "single synthetic release",
			releases: map[string]v1.ReleaseConfig{
				"rosa-stage": {
					Jobs: map[string]bool{"job-a": true, "job-b": true},
				},
			},
			releaseConfigs: []sippyv1.Release{{Release: "rosa-stage", Synthetic: true}},
			expectedOverrides: map[string]string{
				"job-a": "rosa-stage",
				"job-b": "rosa-stage",
			},
		},
		{
			name: "multiple synthetic releases no overlap",
			releases: map[string]v1.ReleaseConfig{
				"rosa-stage": {
					Jobs: map[string]bool{"job-a": true},
				},
				"aro-integration": {
					Jobs: map[string]bool{"job-b": true},
				},
			},
			releaseConfigs: []sippyv1.Release{
				{Release: "rosa-stage", Synthetic: true},
				{Release: "aro-integration", Synthetic: true},
			},
			expectedOverrides: map[string]string{
				"job-a": "rosa-stage",
				"job-b": "aro-integration",
			},
		},
		{
			name: "conflict same job in two synthetic releases",
			releases: map[string]v1.ReleaseConfig{
				"rosa-stage": {
					Jobs: map[string]bool{"job-a": true},
				},
				"rrp-integration": {
					Jobs: map[string]bool{"job-a": true},
				},
			},
			releaseConfigs: []sippyv1.Release{
				{Release: "rosa-stage", Synthetic: true},
				{Release: "rrp-integration", Synthetic: true},
			},
			expectError: true,
		},
		{
			name: "synthetic release with no jobs",
			releases: map[string]v1.ReleaseConfig{
				"rosa-production": {},
			},
			releaseConfigs:    []sippyv1.Release{{Release: "rosa-production", Synthetic: true}},
			expectedOverrides: map[string]string{},
		},
		{
			name: "standard release jobs not in overrides",
			releases: map[string]v1.ReleaseConfig{
				"4.22": {
					Jobs: map[string]bool{"job-a": true},
				},
				"rosa-stage": {
					Jobs: map[string]bool{"job-b": true},
				},
			},
			releaseConfigs: []sippyv1.Release{
				{Release: "4.22", Synthetic: false},
				{Release: "rosa-stage", Synthetic: true},
			},
			expectedOverrides: map[string]string{
				"job-b": "rosa-stage",
			},
		},
		{
			name: "job in both synthetic and standard release only appears once",
			releases: map[string]v1.ReleaseConfig{
				"4.22": {
					Jobs: map[string]bool{"job-a": true},
				},
				"rosa-stage": {
					Jobs: map[string]bool{"job-a": true},
				},
			},
			releaseConfigs: []sippyv1.Release{
				{Release: "4.22", Synthetic: false},
				{Release: "rosa-stage", Synthetic: true},
			},
			expectedOverrides: map[string]string{
				"job-a": "rosa-stage",
			},
		},
		{
			name: "disabled jobs in synthetic release are excluded",
			releases: map[string]v1.ReleaseConfig{
				"rosa-stage": {
					Jobs: map[string]bool{"job-a": true, "job-b": false},
				},
			},
			releaseConfigs: []sippyv1.Release{{Release: "rosa-stage", Synthetic: true}},
			expectedOverrides: map[string]string{
				"job-a": "rosa-stage",
			},
		},
		{
			name: "release in releaseConfigs but not in config is ignored",
			releases: map[string]v1.ReleaseConfig{
				"4.22": {
					Jobs: map[string]bool{"job-a": true},
				},
			},
			releaseConfigs:    []sippyv1.Release{{Release: "rosa-stage", Synthetic: true}},
			expectedOverrides: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overrides, err := BuildSyntheticReleaseJobOverrides(tt.releases, tt.releaseConfigs)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedOverrides, overrides)
		})
	}
}
