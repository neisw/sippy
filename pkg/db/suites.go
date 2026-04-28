package db

import (
	"regexp"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/openshift/sippy/pkg/db/models"
)

// testSuites are known test suites we want to import into sippy. tests from other suites will not be
// imported into sippy. Get the list of seen test suites from bigquery with:
//
//	SELECT DISTINCT(testsuite), count(*) count
//		FROM `openshift-gce-devel.ci_analysis_us.junit` \
//		GROUP BY testsuite
//		ORDER BY count desc
var testSuites = []string{
	// Primary origin suite names
	"openshift-tests",
	"openshift-tests-upgrade",

	// Sippy synthetic tests
	"sippy",

	// ARO
	"rp-api-compat-all/parallel",
	"integration/parallel",
	"stage/parallel",
	"prod/parallel",
	"aro-hcp-tests",

	// ROSA
	"OSD e2e suite",
	"ROSA Regional Platform API E2E Suite",

	// Performance
	"olmv1-GCP nightly compare",

	// Other
	"BackendDisruption",
	"Cluster upgrade",
	"E2E Suite",
	"Kubernetes e2e suite",
	"Log Metrics",
	"Operator results",
	"Symptom Detection",
	"Tests Suite",
	"cluster install",
	"cluster nodes ready",
	"cluster nodes",
	"gather core dump",
	"hypershift-e2e",
	"metal infra",
	"step graph",
	"telco-verification",
	"github.com/openshift/console-operator/test/e2e",
	"prowjob-junit",
	"OLM-Catalog-Validation",
	"insights-operator-tests",
	"CNV-lp-interop",
	"ODF-lp-interop",
	"OADP-lp-interop",
	"ACS-lp-interop",
	"ACSLatest-lp-interop",
	"Fusion-access-lp-interop",
	"MTA-lp-interop",
	"Gitops-lp-interop",
	"Quay-lp-interop",
	"Serverless-lp-interop",
	"ServiceMesh-lp-interop",
	"OpenshiftPipelines-lp-interop",
	"tracing-uiplugin",
}

// testSuitePatterns are regular expressions (MatchString) for suite names that should be imported
// without listing every literal name. Patterns are compiled in init().
var testSuitePatterns = []string{
	// LP interop naming: `lp-interop-<product>--<suffix>`.
	`^lp-interop-`,
}

var compiledTestSuitePatterns []*regexp.Regexp

// Invalid regexes panic at process start.
func init() {
	compiledTestSuitePatterns = make([]*regexp.Regexp, len(testSuitePatterns))
	for i, p := range testSuitePatterns {
		compiledTestSuitePatterns[i] = regexp.MustCompile(p)
	}
}

// CheckForDynamicSuite checks if the name matches any dynamic suite patterns.
// If a match is found, creates or retrieves the suite and returns its ID.
// Returns nil if the name doesn't match any patterns or if creation fails.
func CheckForDynamicSuite(db *gorm.DB, name string) *uint {
	if name == "" {
		return nil
	}

	for _, re := range compiledTestSuitePatterns {
		if re.MatchString(name) {
			return getOrCreateSuite(db, name)
		}
	}

	return nil
}

// getOrCreateSuite finds or creates a suite by name. Returns the suite ID on success, nil on error.
// Uses FirstOrCreate for thread-safe upsert behavior.
func getOrCreateSuite(db *gorm.DB, name string) *uint {
	suite := models.Suite{Name: name}
	result := db.Where("name = ?", name).FirstOrCreate(&suite)
	if result.Error != nil {
		log.WithError(result.Error).Errorf("failed to get or create suite %q", name)
		return nil
	}

	// Log only if we created a new record (RowsAffected > 0)
	if result.RowsAffected > 0 {
		log.WithField("suite", name).Info("created new test suite")
	}

	id := suite.ID
	return &id
}

// Runs when the DB is set up / migrated.
func populateTestSuitesInDB(db *gorm.DB) error {
	for _, suiteName := range testSuites {
		if getOrCreateSuite(db, suiteName) == nil {
			return errors.Errorf("error loading suite into db: %s", suiteName)
		}
	}
	return nil
}
