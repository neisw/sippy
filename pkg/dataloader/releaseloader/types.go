package releaseloader

import (
	"time"
)

// ReleaseTags represents the type returned from a release controller endpoint
// like https://amd64.ocp.releases.ci.openshift.org/api/v1/releasestream/4.9.0-0.nightly/tags
type ReleaseTags struct {
	Name         string       `json:"name"`
	Tags         []ReleaseTag `json:"tags"`
	Architecture string       `json:"architecture"`
}

// ReleaseTag is an individual release tag.
type ReleaseTag struct {
	Name        string `json:"name"`
	Phase       string `json:"phase"`
	PullSpec    string `json:"pullSpec"`
	DownloadURL string `json:"downloadURL"`
}

// JobRunResult represents a job run returned from the release controller.
type JobRunResult struct {
	State          string    `json:"state"`
	URL            string    `json:"url"`
	Retries        int       `json:"retries"`
	TransitionTime time.Time `json:"transitionTime"`
}

// UpgradeResult represents an upgradesTo or upgradesFrom report generated
// by the release controller.
type UpgradeResult struct {
	From    string                  `json:"From"`
	To      string                  `json:"To"`
	Success int                     `json:"Success"`
	Failure int                     `json:"Failure"`
	Total   int                     `json:"Total"`
	History map[string]JobRunResult `json:"History"`
}

// ReleaseDetails represents the details of a release from the release controller.
type ReleaseDetails struct {
	Name          string                             `json:"name"`
	Results       map[string]map[string]JobRunResult `json:"results"`
	UpgradesTo    []UpgradeResult                    `json:"upgradesTo"`
	UpgradesFrom  []UpgradeResult                    `json:"upgradesFrom"`
	ChangeLog     []byte                             `json:"changeLog"`
	ChangeLogJSON ChangeLog                          `json:"changeLogJson"`
}

type ChangeLog struct {
	Components    []ChangeLogComponent `json:"components"`
	From          ChangeLogRelease     `json:"from"`
	To            ChangeLogRelease     `json:"to"`
	UpdatedImages []UpdatedImage       `json:"updatedImages"`
}

type ChangeLogComponent struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	VersionURL string `json:"versionUrl"`
	From       string `json:"from"`
	FromURL    string `json:"fromUrl"`
	DiffURL    string `json:"diffUrl"`
}

type ChangeLogRelease struct {
	Name    string `json:"name"`
	Created string `json:"created"`
	Digest  string `json:"digest"`
}

type UpdatedImage struct {
	Name          string                `json:"name"`
	Path          string                `json:"path"`
	Commits       []UpdatedImageCommits `json:"commits"`
	FullChangeLog string                `json:"fullChangeLog"`
}

type UpdatedImageCommits struct {
	Issues  map[string]string `json:"issues"`
	Bugs    map[string]string `json:"bugs"`
	Subject string            `json:"subject"`
	PullID  int               `json:"pullID"`
	PullURL string            `json:"pullURL"`
}
