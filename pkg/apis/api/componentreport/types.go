package componentreport

import (
	"encoding/json"
	"math/big"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"github.com/openshift/sippy/pkg/apis/cache"
	"github.com/openshift/sippy/pkg/db/models"
	"github.com/openshift/sippy/pkg/util/sets"
)

type Release struct {
	Release string
	End     *time.Time
	Start   *time.Time
}

type ReleaseTestMap struct {
	Release
	Tests map[string]TestStatus
}

type FallbackReleases struct {
	Releases map[string]ReleaseTestMap
}

// PullRequestOptions specifies a specific pull request to use as the
// basis or (more often) sample for the report.
type PullRequestOptions struct {
	Org      string
	Repo     string
	PRNumber string
}

// PayloadOptions specifies a specific payload tag to use as the
// sample for the report. This is only used for sample, not basis.
type PayloadOptions struct {
	Tag string
}

type RequestReleaseOptions struct {
	Release            string              `json:"release" yaml:"release"`
	PullRequestOptions *PullRequestOptions `json:"pull_request_options,omitempty" yaml:"pull_request_options,omitempty"`
	PayloadOptions     *PayloadOptions     `json:"payload_options,omitempty" yaml:"payload_options,omitempty"`
	Start              time.Time           `json:"start,omitempty" yaml:"start,omitempty"`
	End                time.Time           `json:"end,omitempty" yaml:"end,omitempty"`
}

// RequestRelativeReleaseOptions is an unfortunate necessity for views where we do not have
// a fixed time, rather a relative time to now/ga. It is translated to the above normal struct before use.
//
// When returned in the API, it should include the concrete start/end calculated from relative
// for the point in time when the request was made. This is used in the UI to pre-populate the
// date picks to transition from view based to custom reporting.
type RequestRelativeReleaseOptions struct {
	RequestReleaseOptions `json:",inline" yaml:",inline"` //nolint:revive
	// inline is a known option
	RelativeStart string `json:"relative_start,omitempty" yaml:"relative_start,omitempty"`
	RelativeEnd   string `json:"relative_end,omitempty" yaml:"relative_end,omitempty"`
}

// RequestTestIdentificationOptions handles options used in the test details report when we focus in
// on a specific test and variants combo, typically because it is or was regressed.
type RequestTestIdentificationOptions struct {
	Component  string `json:"component,omitempty" yaml:"component,omitempty"`
	Capability string `json:"capability,omitempty" yaml:"capability,omitempty"`
	// TestID is a unique identification for the test defined in the DB.
	// It matches the test_id in the bigquery ci_analysis_us.junit table.
	TestID string `json:"test_id,omitempty" yaml:"test_id,omitempty"`
	// RequestedVariants are used for filtering the test details view down to a specific set.
	RequestedVariants map[string]string `json:"requested_variants,omitempty" yaml:"requested_variants,omitempty"`
	// BaseOverrideRelease is used when we're requesting a test details report for both the base release, and a fallback override that had a better pass rate.
	BaseOverrideRelease string `json:"base_override_release,omitempty" yaml:"base_override_release,omitempty"`
}

type RequestVariantOptions struct {
	ColumnGroupBy       sets.String         `json:"column_group_by" yaml:"column_group_by"`
	DBGroupBy           sets.String         `json:"db_group_by" yaml:"db_group_by"`
	IncludeVariants     map[string][]string `json:"include_variants" yaml:"include_variants"`
	CompareVariants     map[string][]string `json:"compare_variants,omitempty" yaml:"compare_variants,omitempty"`
	VariantCrossCompare []string            `json:"variant_cross_compare,omitempty" yaml:"variant_cross_compare,omitempty"`
}

// RequestOptions is a struct packaging all the options for a CR request.
// BaseOverrideRelease is the counterpart to RequestAdvancedOptions.IncludeMultiReleaseAnalysis
// When multi release analysis is enabled we 'fallback' to the release that has the highest
// threshold for indicating a regression.  If a release prior to the selected BaseRelease has a
// higher standard it will be set as the BaseRelease to be included in the TestDetails analysis
type RequestOptions struct {
	BaseRelease    RequestReleaseOptions
	SampleRelease  RequestReleaseOptions
	VariantOption  RequestVariantOptions
	AdvancedOption RequestAdvancedOptions
	CacheOption    cache.RequestOptions
	// TODO: phase our once multi TestIDOptions is fully implemented
	TestIDOptions []RequestTestIdentificationOptions
}

func AnyAreBaseOverrides(opts []RequestTestIdentificationOptions) bool {
	for _, tid := range opts {
		if tid.BaseOverrideRelease != "" {
			return true
		}
	}
	return false
}

// View is a server side construct representing a predefined view over the component readiness data.
// Useful for defining the primary view of what we deem required for considering the release ready.
type View struct {
	Name            string                           `json:"name" yaml:"name"`
	BaseRelease     RequestRelativeReleaseOptions    `json:"base_release" yaml:"base_release"`
	SampleRelease   RequestRelativeReleaseOptions    `json:"sample_release" yaml:"sample_release"`
	TestIDOption    RequestTestIdentificationOptions `json:"test_id_options" yaml:"test_id_options"`
	VariantOptions  RequestVariantOptions            `json:"variant_options" yaml:"variant_options"`
	AdvancedOptions RequestAdvancedOptions           `json:"advanced_options" yaml:"advanced_options"`

	Metrics            ViewMetrics            `json:"metrics" yaml:"metrics"`
	RegressionTracking ViewRegressionTracking `json:"regression_tracking" yaml:"regression_tracking"`
	AutomateJira       AutomateJira           `json:"automate_jira" yaml:"automate_jira"`
	PrimeCache         ViewPrimeCache         `json:"prime_cache" yaml:"prime_cache"`
}

type ViewMetrics struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
}

type ViewRegressionTracking struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
}

type ViewPrimeCache struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
}
type AutomateJira struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
}

type RequestAdvancedOptions struct {
	MinimumFailure              int  `json:"minimum_failure" yaml:"minimum_failure"`
	Confidence                  int  `json:"confidence" yaml:"confidence"`
	PityFactor                  int  `json:"pity_factor" yaml:"pity_factor"`
	PassRateRequiredNewTests    int  `json:"pass_rate_required_new_tests" yaml:"pass_rate_required_new_tests"`
	PassRateRequiredAllTests    int  `json:"pass_rate_required_all_tests" yaml:"pass_rate_required_all_tests"`
	IgnoreMissing               bool `json:"ignore_missing" yaml:"ignore_missing"`
	IgnoreDisruption            bool `json:"ignore_disruption" yaml:"ignore_disruption"`
	FlakeAsFailure              bool `json:"flake_as_failure" yaml:"flake_as_failure"`
	IncludeMultiReleaseAnalysis bool `json:"include_multi_release_analysis" yaml:"include_multi_release_analysis"`
}

// TestCount is a struct representing the counts of test results in BigQuery-land.
type TestCount struct {
	TotalCount   int `json:"total_count" bigquery:"total_count"`
	SuccessCount int `json:"success_count" bigquery:"success_count"`
	FlakeCount   int `json:"flake_count" bigquery:"flake_count"`
}

//nolint:revive
func (tc TestCount) Add(add TestCount) TestCount {
	tc.TotalCount += add.TotalCount
	tc.SuccessCount += add.SuccessCount
	tc.FlakeCount += add.FlakeCount
	return tc
}
func (tc TestCount) Failures() int { // translate to sippy/stats-land
	failure := tc.TotalCount - tc.SuccessCount - tc.FlakeCount
	if failure < 0 { // this shouldn't happen but just as a failsafe...
		failure = 0
	}
	return failure
}
func (tc TestCount) ToTestStats(flakeAsFailure bool) TestDetailsTestStats { // translate to sippy/stats-land
	return NewTestStats(tc.SuccessCount, tc.Failures(), tc.FlakeCount, flakeAsFailure)
}

// TestStatus is an internal type used to pass data bigquery onwards to the actual
// report generation. It is not serialized over the API.
type TestStatus struct {
	TestName     string   `json:"test_name"`
	TestSuite    string   `json:"test_suite"`
	Component    string   `json:"component"`
	Capabilities []string `json:"capabilities"`
	Variants     []string `json:"variants"`
	TestCount
	LastFailure time.Time `json:"last_failure"`
}

func (ts TestStatus) GetTotalSuccessFailFlakeCounts() (int, int, int, int) {
	failures := ts.Failures()
	return ts.TotalCount, ts.SuccessCount, failures, ts.FlakeCount
}

// ReportTestStatus contains the mapping of all test keys (serialized with TestWithVariantsKey, variants + testID)
// It is also an internal type used to pass data from bigquery onwards to report generation, and does not get serialized
// as an API response.
type ReportTestStatus struct {
	// BaseStatus represents the stable basis for the comparison. Maps TestWithVariantsKey serialized as a string, to test status.
	BaseStatus map[string]TestStatus `json:"base_status"`

	// SampleSatus represents the sample for the comparison. Maps TestWithVariantsKey serialized as a string, to test status.
	SampleStatus map[string]TestStatus `json:"sample_status"`
	GeneratedAt  *time.Time            `json:"generated_at"`
}

type ComponentReport struct {
	Rows        []ReportRow `json:"rows,omitempty"`
	GeneratedAt *time.Time  `json:"generated_at"`
}

type ReportRow struct {
	RowIdentification
	Columns []ReportColumn `json:"columns,omitempty"`
}

type RowIdentification struct {
	Component  string `json:"component"`
	Capability string `json:"capability,omitempty"`
	TestName   string `json:"test_name,omitempty"`
	TestSuite  string `json:"test_suite,omitempty"`
	TestID     string `json:"test_id,omitempty"`
}

type ReportColumn struct {
	ColumnIdentification
	Status         Status              `json:"status"`
	RegressedTests []ReportTestSummary `json:"regressed_tests,omitempty"`
}

type ColumnID string

type ColumnIdentification struct {
	Variants map[string]string `json:"variants"`
}

type Status int

type ReportTestIdentification struct {
	RowIdentification
	ColumnIdentification
}

type ReportTestSummary struct {
	// TODO: really feels like this could just be moved  ReportTestStats, eliminating the need for ReportTestSummary
	ReportTestIdentification
	ReportTestStats
}

// Comparison is the type of comparison done for a test that has been marked red.
type Comparison string

const (
	PassRate    Comparison = "pass_rate"
	FisherExact Comparison = "fisher_exact"
)

// ReportTestStats is an overview struct for a particular regressed test's stats.
// (basis passes and pass rate, sample passes and pass rate, and fishers exact confidence)
// Important type returned by the API.
// TODO: compare with TestStatus we use internally, see if we can converge?
type ReportTestStats struct {
	// ReportStatus is an integer representing the severity of the regression.
	ReportStatus Status `json:"status"`

	// Comparison indicates what mode was used to check this tests results in the sample.
	Comparison Comparison `json:"comparison"`

	// Explanations are human-readable details of why this test was marked regressed.
	Explanations []string `json:"explanations"`

	SampleStats TestDetailsReleaseStats `json:"sample_stats"`

	// RequiredConfidence is the confidence required from Fishers to consider a regression.
	// Typically, it is as defined in the request options, but middleware may choose to adjust.
	// 95 = 95% confidence of a regression required.
	RequiredConfidence int `json:"-"`

	// PityAdjustment can be used to adjust the tolerance for failures for this particular test.
	PityAdjustment float64 `json:"-"`

	// RequiredPassRateAdjustment can be used to adjust the tolerance for failures for a new test.
	RequiredPassRateAdjustment float64 `json:"-"`

	// Optional fields depending on the Comparison mode

	// FisherExact indicates the confidence of a regression after applying Fisher's Exact Test.
	FisherExact *float64 `json:"fisher_exact,omitempty"`

	// BaseStats may not be present in the response, i.e. new tests regressed because of their pass rate.
	BaseStats *TestDetailsReleaseStats `json:"base_stats,omitempty"`

	// LastFailure is the last time the regressed test failed.
	LastFailure *time.Time `json:"last_failure"`

	// Regression is populated with data on when we first detected this regression. If unset it implies
	// the regression tracker has not yet run to find it, or you're using report params/a view without regression tracking.
	Regression *models.TestRegression `json:"regression,omitempty"`
}

// TestDetailsAnalysis is a collection of stats for the report which could potentially carry
// multiple different analyses run.
type TestDetailsAnalysis struct {
	ReportTestStats
	JobStats []TestDetailsJobStats `json:"job_stats,omitempty"`
}

// ReportTestDetails is the top level API response for test details reports.
type ReportTestDetails struct {
	ReportTestIdentification
	JiraComponent   string     `json:"jira_component"`
	JiraComponentID *big.Rat   `json:"jira_component_id"`
	TestName        string     `json:"test_name"`
	GeneratedAt     *time.Time `json:"generated_at"`

	// Analyses is a list of potentially multiple analysis run for this test.
	// Callers can assume that the first in the list is somewhat authoritative, and should
	// be displayed by default, but each analysis offers details and explanations on it's outcome
	// and can be used in some capacity.
	Analyses []TestDetailsAnalysis `json:"analyses"`
}

type TestDetailsReleaseStats struct {
	Release string `json:"release"`
	Start   *time.Time
	End     *time.Time
	TestDetailsTestStats
}

type TestDetailsTestStats struct {
	SuccessCount int `json:"success_count"`
	FailureCount int `json:"failure_count"`
	FlakeCount   int `json:"flake_count"`
	// calculate from the above with PassRate method:
	SuccessRate float64 `json:"success_rate"`
}

func (tdts TestDetailsTestStats) Total() int {
	return tdts.SuccessCount + tdts.FailureCount + tdts.FlakeCount
}

func (tdts TestDetailsTestStats) Passes(flakesAsFailure bool) int {
	if flakesAsFailure {
		return tdts.SuccessCount
	}
	return tdts.SuccessCount + tdts.FlakeCount
}

func (tdts TestDetailsTestStats) PassRate(flakesAsFailure bool) float64 {
	return CalculatePassRate(tdts.SuccessCount, tdts.FailureCount, tdts.FlakeCount, flakesAsFailure)
}

func (tdts TestDetailsTestStats) Add(add TestDetailsTestStats, flakesAsFailure bool) TestDetailsTestStats {
	return NewTestStats(
		tdts.SuccessCount+add.SuccessCount,
		tdts.FailureCount+add.FailureCount,
		tdts.FlakeCount+add.FlakeCount,
		flakesAsFailure,
	)
}

func (tdts TestDetailsTestStats) AddTestCount(add TestCount, flakesAsFailure bool) TestDetailsTestStats {
	return NewTestStats(
		tdts.SuccessCount+add.SuccessCount,
		tdts.FailureCount+add.Failures(),
		tdts.FlakeCount+add.FlakeCount,
		flakesAsFailure,
	)
}

func (tdts TestDetailsTestStats) FailPassWithFlakes(flakesAsFailure bool) (int, int) {
	if flakesAsFailure {
		return tdts.FailureCount + tdts.FlakeCount, tdts.SuccessCount
	}
	return tdts.FailureCount, tdts.SuccessCount + tdts.FlakeCount
}

func NewTestStats(successCount, failureCount, flakeCount int, flakesAsFailure bool) TestDetailsTestStats {
	return TestDetailsTestStats{
		SuccessCount: successCount,
		FailureCount: failureCount,
		FlakeCount:   flakeCount,
		SuccessRate:  CalculatePassRate(successCount, failureCount, flakeCount, flakesAsFailure),
	}
}

func CalculatePassRate(success, failure, flake int, treatFlakeAsFailure bool) float64 {
	total := success + failure + flake
	if total == 0 {
		return 0.0
	}
	if treatFlakeAsFailure {
		return float64(success) / float64(total)
	}
	return float64(success+flake) / float64(total)
}

type TestDetailsJobStats struct {
	// one of sample/base job name could be missing if jobs change between releases
	SampleJobName     string                   `json:"sample_job_name,omitempty"`
	BaseJobName       string                   `json:"base_job_name,omitempty"`
	SampleStats       TestDetailsTestStats     `json:"sample_stats"`
	BaseStats         TestDetailsTestStats     `json:"base_stats"`
	SampleJobRunStats []TestDetailsJobRunStats `json:"sample_job_run_stats,omitempty"`
	BaseJobRunStats   []TestDetailsJobRunStats `json:"base_job_run_stats,omitempty"`
	Significant       bool                     `json:"significant"`
}

type TestDetailsJobRunStats struct {
	JobURL    string         `json:"job_url"`
	JobRunID  string         `json:"job_run_id"`
	StartTime civil.DateTime `json:"start_time"`
	// TestStats is the test stats from one particular job run.
	// For the majority of the tests, there is only one junit. But
	// there are cases multiple junits are generated for the same test.
	TestStats TestDetailsTestStats `json:"test_stats"`
}

// TestJobRunRows are the per job run rows that come back from bigquery for a test details report
// indicating if the test passed or failed.
// Fields are named count somewhat misleadingly as technically they're always 0 or 1 today.
type TestJobRunRows struct {
	TestKey      TestWithVariantsKey `json:"test_key"`
	TestKeyStr   string              `json:"-"` // transient field so we dont have to keep recalculating
	TestName     string              `bigquery:"test_name"`
	ProwJob      string              `bigquery:"prowjob_name"`
	ProwJobRunID string              `bigquery:"prowjob_run_id"`
	ProwJobURL   string              `bigquery:"prowjob_url"`
	StartTime    civil.DateTime      `bigquery:"prowjob_start"`
	TestCount
	JiraComponent   string   `bigquery:"jira_component"`
	JiraComponentID *big.Rat `bigquery:"jira_component_id"`
}

// TestJobRunStatuses contains the rows returned from a test details query organized by base and sample,
// essentially the actual job runs and their status that was used to calculate this
// report.
// Status fields map prowjob name to each row result we received for that job.
type TestJobRunStatuses struct {
	BaseStatus map[string][]TestJobRunRows `json:"base_status"`
	// TODO: This could be a little cleaner if we did status.BaseStatuses plural and tied them to a release,
	// allowing the release fallback mechanism to stay a little cleaner. That would more clearly
	// keep middleware details out of the main codebase.
	BaseOverrideStatus map[string][]TestJobRunRows `json:"base_override_status"`
	SampleStatus       map[string][]TestJobRunRows `json:"sample_status"`
	GeneratedAt        *time.Time                  `json:"generated_at"`
}

const (
	// FailedFixedRegression indicates someone has claimed the bug is fix, but we see failures past the resolution time
	FailedFixedRegression Status = -1000
	// ExtremeRegression shows regression with >15% pass rate change
	ExtremeRegression Status = -500
	// SignificantRegression shows significant regression
	SignificantRegression Status = -400
	// ExtremeTriagedRegression shows an ExtremeRegression that clears when Triaged incidents are factored in
	ExtremeTriagedRegression Status = -300
	// SignificantTriagedRegression shows a SignificantRegression that clears when Triaged incidents are factored in
	SignificantTriagedRegression Status = -200
	// FixedRegression indicates someone has claimed the bug is now fixed, but has not yet rolled off the sample window
	FixedRegression Status = -150
	// MissingSample indicates sample data missing
	MissingSample Status = -100
	// NotSignificant indicates no significant difference
	NotSignificant Status = 0
	// MissingBasis indicates basis data missing
	MissingBasis Status = 100
	// MissingBasisAndSample indicates basis and sample data missing
	MissingBasisAndSample Status = 200
	// SignificantImprovement indicates improved sample rate
	SignificantImprovement Status = 300
)

func StringForStatus(s Status) string {
	switch s {
	case ExtremeRegression:
		return "Extreme"
	case SignificantRegression:
		return "Significant"
	case ExtremeTriagedRegression:
		return "ExtremeTriaged"
	case SignificantTriagedRegression:
		return "SignificantTriaged"
	case MissingSample:
		return "MissingSample"
	case FixedRegression:
		return "Fixed"
	case FailedFixedRegression:
		return "FailedFixed"
	}
	return "Unknown"
}

type ReportResponse []ReportRow

type TestVariants struct {
	Network  []string `json:"network,omitempty"`
	Upgrade  []string `json:"upgrade,omitempty"`
	Arch     []string `json:"arch,omitempty"`
	Platform []string `json:"platform,omitempty"`
	Variant  []string `json:"variant,omitempty"`
}

// JobVariant defines a variant and the possible values
type JobVariant struct {
	VariantName   string   `bigquery:"variant_name"`
	VariantValues []string `bigquery:"variant_values"`
}

// JobVariants contains all variants supported in the system.
type JobVariants struct {
	Variants map[string][]string `json:"variants,omitempty"`
}

type Variant struct {
	Key   string `bigquery:"key" json:"key"`
	Value string `bigquery:"value" json:"value"`
}

// TODO: temporary for migration
type TestRegressionBigQuery struct {
	// Snapshot is the time at which the full set of regressions for all releases was inserted into the db.
	// When querying we use only those with the latest snapshot time.
	Snapshot     time.Time              `bigquery:"snapshot" json:"snapshot"`
	View         string                 `bigquery:"view" json:"view"`
	Release      string                 `bigquery:"release" json:"release"`
	TestID       string                 `bigquery:"test_id" json:"test_id"`
	TestName     string                 `bigquery:"test_name" json:"test_name"`
	RegressionID string                 `bigquery:"regression_id" json:"regression_id"`
	Opened       time.Time              `bigquery:"opened" json:"opened"`
	Closed       bigquery.NullTimestamp `bigquery:"closed" json:"closed"`
	Variants     []Variant              `bigquery:"variants" json:"variants"`
}

// TestWithVariantsKey connects the core unique db testID string to a set of variants.
// Used to serialize/deserialize as a map key when we pass test status around.
type TestWithVariantsKey struct {
	TestID string `json:"test_id"`

	// Proposed, need to serialize to use as map key
	Variants map[string]string `json:"variants"`
}

// KeyOrDie serializes this test key into a json string suitable for use in maps.
// JSON serialization uses sorted map keys, so the output is stable.
func (t TestWithVariantsKey) KeyOrDie() string {
	testIDBytes, err := json.Marshal(t)
	if err != nil {
		panic(err)
	}
	return string(testIDBytes)
}
