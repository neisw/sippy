// nolint
package componentreadiness

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/openshift/sippy/pkg/apis/api/componentreport/bq"
	"github.com/openshift/sippy/pkg/apis/api/componentreport/crtest"
	"github.com/openshift/sippy/pkg/apis/api/componentreport/reqopts"
	"github.com/openshift/sippy/pkg/apis/api/componentreport/testdetails"
	"github.com/stretchr/testify/assert"

	"github.com/openshift/sippy/pkg/api/componentreadiness/utils"
	v1 "github.com/openshift/sippy/pkg/apis/config/v1"

	crtype "github.com/openshift/sippy/pkg/apis/api/componentreport"
	"github.com/openshift/sippy/pkg/util/sets"
)

func fakeComponentAndCapabilityGetter(test crtest.KeyWithVariants, stats bq.TestStatus) (string, []string) {
	name := stats.TestName
	known := map[string]struct {
		component    string
		capabilities []string
	}{
		"test 1": {
			component:    "component 1",
			capabilities: []string{"cap1"},
		},
		"test 2": {
			component:    "component 2",
			capabilities: []string{"cap21", "cap22"},
		},
		"test 3": {
			component:    "component 1",
			capabilities: []string{"cap1"},
		},
	}
	if comCap, ok := known[name]; ok {
		return comCap.component, comCap.capabilities
	}
	return "other", []string{"other"}
}

var (
	defaultAdvancedOption = reqopts.Advanced{
		Confidence:     95,
		PityFactor:     5,
		MinimumFailure: 3,
	}
	defaultColumnGroupByVariants    = sets.NewString(strings.Split(DefaultColumnGroupBy, ",")...)
	defaultDBGroupByVariants        = sets.NewString(strings.Split(DefaultDBGroupBy, ",")...)
	defaultComponentReportGenerator = ComponentReportGenerator{
		ReqOptions: reqopts.RequestOptions{
			VariantOption: reqopts.Variants{
				ColumnGroupBy: defaultColumnGroupByVariants,
				DBGroupBy:     defaultDBGroupByVariants,
			},
			AdvancedOption: defaultAdvancedOption,
		},
	}
	flakeFailAdvancedOption = reqopts.Advanced{
		Confidence:     95,
		PityFactor:     5,
		MinimumFailure: 3,
		FlakeAsFailure: true,
	}
	flakeFailComponentReportGenerator = ComponentReportGenerator{
		ReqOptions: reqopts.RequestOptions{
			VariantOption: reqopts.Variants{
				ColumnGroupBy: defaultColumnGroupByVariants,
				DBGroupBy:     defaultDBGroupByVariants,
			},
			AdvancedOption: flakeFailAdvancedOption,
		},
	}
	installerColumnGroupByVariants           = sets.NewString("Platform", "Architecture", "Network", "Installer")
	groupByInstallerComponentReportGenerator = ComponentReportGenerator{
		ReqOptions: reqopts.RequestOptions{
			VariantOption: reqopts.Variants{
				ColumnGroupBy: installerColumnGroupByVariants,
				DBGroupBy:     defaultDBGroupByVariants,
			},
			AdvancedOption: defaultAdvancedOption,
		},
	}
	componentPageGenerator = ComponentReportGenerator{
		ReqOptions: reqopts.RequestOptions{
			TestIDOptions: []reqopts.TestIdentification{
				{
					Component: "component 2",
				},
			},
			VariantOption: reqopts.Variants{
				ColumnGroupBy: defaultColumnGroupByVariants,
				DBGroupBy:     defaultDBGroupByVariants,
			},
			AdvancedOption: defaultAdvancedOption,
		},
	}
	capabilityPageGenerator = ComponentReportGenerator{
		ReqOptions: reqopts.RequestOptions{
			TestIDOptions: []reqopts.TestIdentification{
				{
					Component:  "component 2",
					Capability: "cap22",
				},
			},
			VariantOption: reqopts.Variants{
				ColumnGroupBy: defaultColumnGroupByVariants,
				DBGroupBy:     defaultDBGroupByVariants,
			},
			AdvancedOption: defaultAdvancedOption,
		},
	}
	testPageGenerator = ComponentReportGenerator{
		ReqOptions: reqopts.RequestOptions{
			TestIDOptions: []reqopts.TestIdentification{
				{
					Component:  "component 2",
					Capability: "cap22",
					TestID:     "2",
				},
			},
			VariantOption: reqopts.Variants{
				ColumnGroupBy: defaultColumnGroupByVariants,
				DBGroupBy:     defaultDBGroupByVariants,
			},
			AdvancedOption: defaultAdvancedOption,
		},
	}
	testDetailsGenerator = ComponentReportGenerator{
		ReqOptions: reqopts.RequestOptions{
			TestIDOptions: []reqopts.TestIdentification{
				{
					Component:  "component 1",
					Capability: "cap11",
					TestID:     "1",
					RequestedVariants: map[string]string{
						"Platform":     "aws",
						"Architecture": "amd64",
						"Network":      "ovn",
					},
				},
			},
			VariantOption: reqopts.Variants{
				ColumnGroupBy: defaultColumnGroupByVariants,
				DBGroupBy:     defaultDBGroupByVariants,
			},
			AdvancedOption: defaultAdvancedOption,
		},
	}
)

func filterColumnIDByDefault(id crtest.ColumnIdentification) crtest.ColumnIdentification {
	ret := crtest.ColumnIdentification{Variants: map[string]string{}}
	for _, variant := range strings.Split(DefaultDBGroupBy, ",") {
		if value, ok := id.Variants[variant]; ok {
			ret.Variants[variant] = value
		}
	}
	return ret
}

func TestGenerateComponentReport(t *testing.T) {
	awsAMD64OVNTest := crtest.KeyWithVariants{
		TestID: "1",
		Variants: map[string]string{
			"Platform":     "aws",
			"Architecture": "amd64",
			"Network":      "ovn",
			"Upgrade":      "upgrade-micro",
			"Topology":     "ha",
			"FeatureSet":   "techpreview",
			"Suite":        "serial",
			"Installer":    "ipi",
		},
	}
	awsAMD64OVNTestBytes, err := json.Marshal(awsAMD64OVNTest)
	if err != nil {
		assert.NoError(t, err, "error marshalling awsAMD64OVNTest")
	}
	awsAMD64SDNTest := crtest.KeyWithVariants{
		TestID: "2",
		Variants: map[string]string{
			"Platform":     "aws",
			"Architecture": "amd64",
			"Network":      "sdn",
			"Upgrade":      "upgrade-micro",
			"Topology":     "ha",
			"FeatureSet":   "techpreview",
			"Suite":        "serial",
			"Installer":    "ipi",
		},
	}
	awsAMD64SDNTestBytes, err := json.Marshal(awsAMD64SDNTest)
	if err != nil {
		assert.NoError(t, err, "error marshalling awsAMD64SDNTest")
	}
	awsAMD64SDNInstallerUPITest := crtest.KeyWithVariants{
		TestID: "2",
		Variants: map[string]string{
			"Platform":     "aws",
			"Architecture": "amd64",
			"Network":      "sdn",
			"Upgrade":      "upgrade-micro",
			"Topology":     "ha",
			"FeatureSet":   "techpreview",
			"Suite":        "serial",
			"Installer":    "upi",
		},
	}
	awsAMD64SDNInstallerUPITestBytes, err := json.Marshal(awsAMD64SDNInstallerUPITest)
	if err != nil {
		assert.NoError(t, err, "error marshalling awsAMD64SDNInstallerUPITest")
	}
	awsAMD64OVN2Test := crtest.KeyWithVariants{
		TestID: "3",
		Variants: map[string]string{
			"Platform":     "aws",
			"Architecture": "amd64",
			"Network":      "ovn",
			"Upgrade":      "upgrade-micro",
		},
	}
	awsAMD64OVN2TestBytes, err := json.Marshal(awsAMD64OVN2Test)
	if err != nil {
		assert.NoError(t, err, "error marshalling awsAMD64OVN2Test")
	}
	awsAMD64OVNInstallerIPITest := crtest.KeyWithVariants{
		TestID: "1",
		Variants: map[string]string{
			"Platform":     "aws",
			"Architecture": "amd64",
			"Network":      "ovn",
			"Upgrade":      "upgrade-micro",
			"Topology":     "ha",
			"FeatureSet":   "techpreview",
			"Suite":        "serial",
			"Installer":    "ipi",
		},
	}
	awsAMD64OVNVariantsTestBytes, err := json.Marshal(awsAMD64OVNInstallerIPITest)
	if err != nil {
		assert.NoError(t, err, "error marshalling awsAMD64OVNInstallerIPITest")
	}
	awsAMD64OVNBaseTestStats90Percent := bq.TestStatus{
		TestName: "test 1",
		Variants: []string{"standard"},
		Count: crtest.Count{
			TotalCount:   1000,
			FlakeCount:   10,
			SuccessCount: 900,
		},
	}
	awsAMD64OVNBaseTestStats50Percent := bq.TestStatus{
		TestName: "test 1",
		Variants: []string{"standard"},
		Count: crtest.Count{
			TotalCount:   1000,
			FlakeCount:   10,
			SuccessCount: 500,
		},
	}
	awsAMD64OVNBaseTestStatsVariants90Percent := bq.TestStatus{
		TestName: "test 1",
		Variants: []string{"standard", "fips"},
		Count: crtest.Count{
			TotalCount:   1000,
			FlakeCount:   10,
			SuccessCount: 900,
		},
	}
	awsAMD64OVNSampleTestStats90Percent := bq.TestStatus{
		TestName: "test 1",
		Variants: []string{"standard"},
		Count: crtest.Count{
			TotalCount:   100,
			FlakeCount:   1,
			SuccessCount: 90,
		},
	}
	awsAMD64OVNSampleTestStats85Percent := bq.TestStatus{
		TestName: "test 1",
		Variants: []string{"standard"},
		Count: crtest.Count{
			TotalCount:   100,
			FlakeCount:   1,
			SuccessCount: 85,
		},
	}
	awsAMD64OVNSampleTestStats50Percent := bq.TestStatus{
		TestName: "test 1",
		Variants: []string{"standard"},
		Count: crtest.Count{
			TotalCount:   100,
			FlakeCount:   1,
			SuccessCount: 50,
		},
	}
	awsAMD64OVNSampleTestStatsTiny := bq.TestStatus{
		TestName: "test 1",
		Variants: []string{"standard"},
		Count: crtest.Count{
			TotalCount:   3,
			FlakeCount:   0,
			SuccessCount: 1,
		},
	}
	awsAMD64OVNSampleTestStatsVariants90Percent := bq.TestStatus{
		TestName: "test 1",
		Variants: []string{"standard", "fips"},
		Count: crtest.Count{
			TotalCount:   100,
			FlakeCount:   1,
			SuccessCount: 90,
		},
	}
	awsAMD64SDNBaseTestStats90Percent := bq.TestStatus{
		TestName: "test 2",
		Variants: []string{"standard"},
		Count: crtest.Count{
			TotalCount:   1000,
			FlakeCount:   10,
			SuccessCount: 900,
		},
	}
	awsAMD64SDNBaseTestStats50Percent := bq.TestStatus{
		TestName: "test 2",
		Variants: []string{"standard"},
		Count: crtest.Count{
			TotalCount:   1000,
			FlakeCount:   10,
			SuccessCount: 500,
		},
	}
	awsAMD64SDNSampleTestStats90Percent := bq.TestStatus{
		TestName: "test 2",
		Variants: []string{"standard"},
		Count: crtest.Count{
			TotalCount:   100,
			FlakeCount:   1,
			SuccessCount: 90,
		},
	}
	awsAMD64OVN2BaseTestStats90Percent := bq.TestStatus{
		TestName: "test 3",
		Variants: []string{"standard"},
		Count: crtest.Count{
			TotalCount:   1000,
			FlakeCount:   10,
			SuccessCount: 900,
		},
	}
	awsAMD64OVN2SampleTestStats80Percent := bq.TestStatus{
		TestName: "test 3",
		Variants: []string{"standard"},
		Count: crtest.Count{
			TotalCount:   100,
			FlakeCount:   1,
			SuccessCount: 80,
		},
	}
	columnAWSAMD64OVN := crtest.ColumnIdentification{
		Variants: map[string]string{
			"Platform":     "aws",
			"Architecture": "amd64",
			"Network":      "ovn",
		},
	}
	columnAWSAMD64OVNInstallerIPI := crtest.ColumnIdentification{
		Variants: map[string]string{
			"Platform":     "aws",
			"Architecture": "amd64",
			"Network":      "ovn",
			"Installer":    "ipi",
		},
	}
	columnAWSAMD64SDN := crtest.ColumnIdentification{
		Variants: map[string]string{
			"Platform":     "aws",
			"Architecture": "amd64",
			"Network":      "sdn",
		},
	}
	columnAWSAMD64SDNInstallerUPI := crtest.ColumnIdentification{
		Variants: map[string]string{
			"Platform":     "aws",
			"Architecture": "amd64",
			"Network":      "sdn",
			"Installer":    "upi",
		},
	}
	columnAWSAMD64OVNFull := crtest.ColumnIdentification{
		Variants: map[string]string{
			"Platform":     "aws",
			"Architecture": "amd64",
			"Network":      "ovn",
			"Upgrade":      "upgrade-micro",
			"Topology":     "ha",
			"FeatureSet":   "techpreview",
			"Suite":        "serial",
			"Installer":    "ipi",
		},
	}
	columnAWSAMD64SDNFull := crtest.ColumnIdentification{
		Variants: map[string]string{
			"Platform":     "aws",
			"Architecture": "amd64",
			"Network":      "sdn",
			"Upgrade":      "upgrade-micro",
			"Topology":     "ha",
			"FeatureSet":   "techpreview",
			"Suite":        "serial",
			"Installer":    "ipi",
		},
	}
	rowComponent1 := crtest.RowIdentification{
		Component: "component 1",
	}
	rowComponent2 := crtest.RowIdentification{
		Component: "component 2",
	}
	rowComponent2Cap21 := crtest.RowIdentification{
		Component:  "component 2",
		Capability: "cap21",
	}
	rowComponent2Cap22 := crtest.RowIdentification{
		Component:  "component 2",
		Capability: "cap22",
	}
	rowComponent2Cap22Test2 := crtest.RowIdentification{
		Component:  "component 2",
		Capability: "cap22",
		TestName:   "test 2",
		TestID:     "2",
	}

	tests := []struct {
		name           string
		generator      ComponentReportGenerator
		baseStatus     map[string]bq.TestStatus
		sampleStatus   map[string]bq.TestStatus
		expectedReport crtype.ComponentReport
	}{
		{
			name:      "top page test no significant and missing data",
			generator: defaultComponentReportGenerator,
			baseStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNBaseTestStats90Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNBaseTestStats90Percent,
			},
			sampleStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNSampleTestStats85Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNSampleTestStats90Percent,
			},
			expectedReport: crtype.ComponentReport{
				Rows: []crtype.ReportRow{
					{
						RowIdentification: crtest.RowIdentification{
							Component: "component 1",
						},
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVN,
								Status:               crtest.NotSignificant,
							},
							{
								ColumnIdentification: columnAWSAMD64SDN,
								Status:               crtest.MissingBasisAndSample,
							},
						},
					},
					{
						RowIdentification: crtest.RowIdentification{
							Component: "component 2",
						},
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVN,
								Status:               crtest.MissingBasisAndSample,
							},
							{
								ColumnIdentification: columnAWSAMD64SDN,
								Status:               crtest.NotSignificant,
							},
						},
					},
				},
			},
		},
		{
			name:      "top page test with both improvement and regression",
			generator: defaultComponentReportGenerator,
			baseStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes):  awsAMD64OVNBaseTestStats90Percent,
				string(awsAMD64OVN2TestBytes): awsAMD64OVN2BaseTestStats90Percent,
				string(awsAMD64SDNTestBytes):  awsAMD64SDNBaseTestStats50Percent,
			},
			sampleStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes):  awsAMD64OVNSampleTestStats50Percent,
				string(awsAMD64OVN2TestBytes): awsAMD64OVN2SampleTestStats80Percent,
				string(awsAMD64SDNTestBytes):  awsAMD64SDNSampleTestStats90Percent,
			},
			expectedReport: crtype.ComponentReport{
				Rows: []crtype.ReportRow{
					{
						RowIdentification: rowComponent1,
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVN,
								Status:               crtest.ExtremeRegression,
								RegressedTests: []crtype.ReportTestSummary{
									{
										Identification: crtest.Identification{
											RowIdentification: crtest.RowIdentification{
												TestName: awsAMD64OVNBaseTestStats90Percent.TestName,
												TestID:   awsAMD64OVNTest.TestID,
											},
											ColumnIdentification: crtest.ColumnIdentification{
												Variants: awsAMD64OVNTest.Variants,
											},
										},
										TestComparison: testdetails.TestComparison{
											RequiredConfidence: 95,
											Comparison:         crtest.FisherExact,
											Explanations: []string{
												"Extreme regression detected.",
												"Fishers Exact probability of a regression: 100.00%.",
												"Test pass rate dropped from 91.00% to 51.00%.",
											},
											ReportStatus: crtest.ExtremeRegression,
											FisherExact:  thrift.Float64Ptr(1.8251046156331867e-21),
											SampleStats: testdetails.ReleaseStats{
												Stats: crtest.Stats{
													SuccessRate:  0.51,
													SuccessCount: 50,
													FailureCount: 49,
													FlakeCount:   1,
												},
												Start: &time.Time{},
												End:   &time.Time{},
											},
											BaseStats: &testdetails.ReleaseStats{
												Stats: crtest.Stats{
													SuccessRate:  0.91,
													SuccessCount: 900,
													FailureCount: 90,
													FlakeCount:   10,
												},
												Start: &time.Time{},
												End:   &time.Time{},
											},
										},
									},
									{
										Identification: crtest.Identification{
											RowIdentification: crtest.RowIdentification{
												TestName: awsAMD64OVN2BaseTestStats90Percent.TestName,
												TestID:   awsAMD64OVN2Test.TestID,
											},
											ColumnIdentification: crtest.ColumnIdentification{
												Variants: awsAMD64OVN2Test.Variants,
											},
										},
										TestComparison: testdetails.TestComparison{
											RequiredConfidence: 95,
											Comparison:         crtest.FisherExact,
											Explanations: []string{
												"Significant regression detected.",
												"Fishers Exact probability of a regression: 100.00%.",
												"Test pass rate dropped from 91.00% to 81.00%.",
											},
											ReportStatus: crtest.SignificantRegression,
											FisherExact:  thrift.Float64Ptr(0.002621948654892275),
											SampleStats: testdetails.ReleaseStats{
												Stats: crtest.Stats{
													SuccessRate:  0.81,
													SuccessCount: 80,
													FailureCount: 19,
													FlakeCount:   1,
												},
												Start: &time.Time{},
												End:   &time.Time{},
											},
											BaseStats: &testdetails.ReleaseStats{
												Stats: crtest.Stats{
													SuccessRate:  0.91,
													SuccessCount: 900,
													FailureCount: 90,
													FlakeCount:   10,
												},
												Start: &time.Time{},
												End:   &time.Time{},
											},
										},
									},
								},
							},
							{
								ColumnIdentification: columnAWSAMD64SDN,
								Status:               crtest.MissingBasisAndSample,
							},
						},
					},
					{
						RowIdentification: rowComponent2,
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVN,
								Status:               crtest.MissingBasisAndSample,
							},
							{
								ColumnIdentification: columnAWSAMD64SDN,
								Status:               crtest.SignificantImprovement,
							},
						},
					},
				},
			},
		},
		{
			name:      "component page test no significant and missing data",
			generator: componentPageGenerator,
			baseStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNBaseTestStats90Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNBaseTestStats90Percent,
			},
			sampleStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNSampleTestStats90Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNSampleTestStats90Percent,
			},
			expectedReport: crtype.ComponentReport{
				Rows: []crtype.ReportRow{
					{
						RowIdentification: rowComponent2Cap21,
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVN,
								Status:               crtest.MissingBasisAndSample,
							},
							{
								ColumnIdentification: columnAWSAMD64SDN,
								Status:               crtest.NotSignificant,
							},
						},
					},
					{
						RowIdentification: rowComponent2Cap22,
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVN,
								Status:               crtest.MissingBasisAndSample,
							},
							{
								ColumnIdentification: columnAWSAMD64SDN,
								Status:               crtest.NotSignificant,
							},
						},
					},
				},
			},
		},
		{
			name:      "component page test with both improvement and regression",
			generator: componentPageGenerator,
			baseStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNBaseTestStats90Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNBaseTestStats50Percent,
			},
			sampleStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNBaseTestStats50Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNBaseTestStats90Percent,
			},
			expectedReport: crtype.ComponentReport{
				Rows: []crtype.ReportRow{
					{
						RowIdentification: rowComponent2Cap21,
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVN,
								Status:               crtest.MissingBasisAndSample,
							},
							{
								ColumnIdentification: columnAWSAMD64SDN,
								Status:               crtest.SignificantImprovement,
							},
						},
					},
					{
						RowIdentification: rowComponent2Cap22,
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVN,
								Status:               crtest.MissingBasisAndSample,
							},
							{
								ColumnIdentification: columnAWSAMD64SDN,
								Status:               crtest.SignificantImprovement,
							},
						},
					},
				},
			},
		},
		{
			name:      "capability page test no significant and missing data",
			generator: capabilityPageGenerator,
			baseStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNBaseTestStats90Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNBaseTestStats90Percent,
			},
			sampleStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNSampleTestStats90Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNSampleTestStats90Percent,
			},
			expectedReport: crtype.ComponentReport{
				Rows: []crtype.ReportRow{
					{
						RowIdentification: rowComponent2Cap22Test2,
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVN,
								Status:               crtest.MissingBasisAndSample,
							},
							{
								ColumnIdentification: columnAWSAMD64SDN,
								Status:               crtest.NotSignificant,
							},
						},
					},
				},
			},
		},
		{
			name:      "capability page test with both improvement and regression",
			generator: capabilityPageGenerator,
			baseStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNBaseTestStats90Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNBaseTestStats50Percent,
			},
			sampleStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNSampleTestStats50Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNSampleTestStats90Percent,
			},
			expectedReport: crtype.ComponentReport{
				Rows: []crtype.ReportRow{
					{
						RowIdentification: rowComponent2Cap22Test2,
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVN,
								Status:               crtest.MissingBasisAndSample,
							},
							{
								ColumnIdentification: columnAWSAMD64SDN,
								Status:               crtest.SignificantImprovement,
							},
						},
					},
				},
			},
		},
		{
			name:      "test page test no significant and missing data",
			generator: testPageGenerator,
			baseStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNBaseTestStats90Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNBaseTestStats90Percent,
			},
			sampleStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNSampleTestStats90Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNSampleTestStats90Percent,
			},
			expectedReport: crtype.ComponentReport{
				Rows: []crtype.ReportRow{
					{
						RowIdentification: rowComponent2Cap22Test2,
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: filterColumnIDByDefault(columnAWSAMD64OVNFull),
								Status:               crtest.MissingBasisAndSample,
							},
							{
								ColumnIdentification: filterColumnIDByDefault(columnAWSAMD64SDNFull),
								Status:               crtest.NotSignificant,
							},
						},
					},
				},
			},
		},
		{
			name:      "test page test with both improvement and regression",
			generator: testPageGenerator,
			baseStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNBaseTestStats90Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNBaseTestStats50Percent,
			},
			sampleStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNSampleTestStats50Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNSampleTestStats90Percent,
			},
			expectedReport: crtype.ComponentReport{
				Rows: []crtype.ReportRow{
					{
						RowIdentification: rowComponent2Cap22Test2,
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: filterColumnIDByDefault(columnAWSAMD64OVNFull),
								Status:               crtest.MissingBasisAndSample,
							},
							{
								ColumnIdentification: filterColumnIDByDefault(columnAWSAMD64SDNFull),
								Status:               crtest.SignificantImprovement,
							},
						},
					},
				},
			},
		},
		{
			name: "top page test confidence 90 result in regression",
			generator: ComponentReportGenerator{
				ReqOptions: reqopts.RequestOptions{
					VariantOption: reqopts.Variants{
						ColumnGroupBy: defaultColumnGroupByVariants,
					},
					AdvancedOption: reqopts.Advanced{
						Confidence:     90,
						PityFactor:     5,
						MinimumFailure: 3,
					},
				},
			},
			baseStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNBaseTestStats90Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNBaseTestStats90Percent,
			},
			sampleStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNSampleTestStats85Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNSampleTestStats90Percent,
			},
			expectedReport: crtype.ComponentReport{
				Rows: []crtype.ReportRow{
					{
						RowIdentification: rowComponent1,
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVN,
								Status:               crtest.SignificantRegression,
								RegressedTests: []crtype.ReportTestSummary{
									{
										Identification: crtest.Identification{
											RowIdentification: crtest.RowIdentification{
												TestName: awsAMD64OVNBaseTestStats90Percent.TestName,
												TestID:   awsAMD64OVNTest.TestID,
											},
											ColumnIdentification: crtest.ColumnIdentification{
												Variants: awsAMD64OVNTest.Variants,
											},
										},
										TestComparison: testdetails.TestComparison{
											RequiredConfidence: 90,
											Comparison:         crtest.FisherExact,
											Explanations: []string{
												"Significant regression detected.",
												"Fishers Exact probability of a regression: 99.92%.",
												"Test pass rate dropped from 91.00% to 86.00%.",
											},
											ReportStatus: crtest.SignificantRegression,
											FisherExact:  thrift.Float64Ptr(0.07837082801914011),
											SampleStats: testdetails.ReleaseStats{
												Stats: crtest.Stats{
													SuccessRate:  0.86,
													SuccessCount: 85,
													FailureCount: 14,
													FlakeCount:   1,
												},
												Start: &time.Time{},
												End:   &time.Time{},
											},
											BaseStats: &testdetails.ReleaseStats{
												Stats: crtest.Stats{
													SuccessRate:  0.91,
													SuccessCount: 900,
													FailureCount: 90,
													FlakeCount:   10,
												},
												Start: &time.Time{},
												End:   &time.Time{},
											},
										},
									},
								},
							},
							{
								ColumnIdentification: columnAWSAMD64SDN,
								Status:               crtest.MissingBasisAndSample,
							},
						},
					},
					{
						RowIdentification: rowComponent2,
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVN,
								Status:               crtest.MissingBasisAndSample,
							},
							{
								ColumnIdentification: columnAWSAMD64SDN,
								Status:               crtest.NotSignificant,
							},
						},
					},
				},
			},
		},
		{
			name: "top page test confidence 90 pity 10 result in no regression",
			generator: ComponentReportGenerator{
				ReqOptions: reqopts.RequestOptions{
					VariantOption: reqopts.Variants{
						ColumnGroupBy: defaultColumnGroupByVariants,
					},
					AdvancedOption: reqopts.Advanced{
						Confidence:     90,
						PityFactor:     10,
						MinimumFailure: 3,
					},
				},
			},
			baseStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNBaseTestStats90Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNBaseTestStats90Percent,
			},
			sampleStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNSampleTestStats85Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNSampleTestStats90Percent,
			},
			expectedReport: crtype.ComponentReport{
				Rows: []crtype.ReportRow{
					{
						RowIdentification: rowComponent1,
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVN,
								Status:               crtest.NotSignificant,
							},
							{
								ColumnIdentification: columnAWSAMD64SDN,
								Status:               crtest.MissingBasisAndSample,
							},
						},
					},
					{
						RowIdentification: rowComponent2,
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVN,
								Status:               crtest.MissingBasisAndSample,
							},
							{
								ColumnIdentification: columnAWSAMD64SDN,
								Status:               crtest.NotSignificant,
							},
						},
					},
				},
			},
		},
		{
			name:      "top page test minimum failure no regression",
			generator: defaultComponentReportGenerator,
			baseStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNBaseTestStats90Percent,
				string(awsAMD64SDNTestBytes): awsAMD64SDNBaseTestStats90Percent,
			},
			sampleStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes): awsAMD64OVNSampleTestStatsTiny,
				string(awsAMD64SDNTestBytes): awsAMD64SDNSampleTestStats90Percent,
			},
			expectedReport: crtype.ComponentReport{
				Rows: []crtype.ReportRow{
					{
						RowIdentification: rowComponent1,
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVN,
								Status:               crtest.NotSignificant,
							},
							{
								ColumnIdentification: columnAWSAMD64SDN,
								Status:               crtest.MissingBasisAndSample,
							},
						},
					},
					{
						RowIdentification: rowComponent2,
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVN,
								Status:               crtest.MissingBasisAndSample,
							},
							{
								ColumnIdentification: columnAWSAMD64SDN,
								Status:               crtest.NotSignificant,
							},
						},
					},
				},
			},
		},
		{
			name:      "top page test group by installer",
			generator: groupByInstallerComponentReportGenerator,
			baseStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNVariantsTestBytes):     awsAMD64OVNBaseTestStatsVariants90Percent,
				string(awsAMD64SDNInstallerUPITestBytes): awsAMD64SDNBaseTestStats90Percent,
			},
			sampleStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNVariantsTestBytes):     awsAMD64OVNSampleTestStatsVariants90Percent,
				string(awsAMD64SDNInstallerUPITestBytes): awsAMD64SDNSampleTestStats90Percent,
			},
			expectedReport: crtype.ComponentReport{
				Rows: []crtype.ReportRow{
					{
						RowIdentification: crtest.RowIdentification{
							Component: "component 1",
						},
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVNInstallerIPI,
								Status:               crtest.NotSignificant,
							},
							{
								ColumnIdentification: columnAWSAMD64SDNInstallerUPI,
								Status:               crtest.MissingBasisAndSample,
							},
						},
					},
					{
						RowIdentification: crtest.RowIdentification{
							Component: "component 2",
						},
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVNInstallerIPI,
								Status:               crtest.MissingBasisAndSample,
							},
							{
								ColumnIdentification: columnAWSAMD64SDNInstallerUPI,
								Status:               crtest.NotSignificant,
							},
						},
					},
				},
			},
		},
		{
			name:      "top page test with both improvement and regression flake as failure",
			generator: flakeFailComponentReportGenerator,
			baseStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes):  awsAMD64OVNBaseTestStats90Percent,
				string(awsAMD64OVN2TestBytes): awsAMD64OVN2BaseTestStats90Percent,
				string(awsAMD64SDNTestBytes):  awsAMD64SDNBaseTestStats50Percent,
			},
			sampleStatus: map[string]bq.TestStatus{
				string(awsAMD64OVNTestBytes):  awsAMD64OVNSampleTestStats50Percent,
				string(awsAMD64OVN2TestBytes): awsAMD64OVN2SampleTestStats80Percent,
				string(awsAMD64SDNTestBytes):  awsAMD64SDNSampleTestStats90Percent,
			},
			expectedReport: crtype.ComponentReport{
				Rows: []crtype.ReportRow{
					{
						RowIdentification: rowComponent1,
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVN,
								Status:               crtest.ExtremeRegression,
								RegressedTests: []crtype.ReportTestSummary{
									{
										Identification: crtest.Identification{
											RowIdentification: crtest.RowIdentification{
												TestName: awsAMD64OVNBaseTestStats90Percent.TestName,
												TestID:   awsAMD64OVNTest.TestID,
											},
											ColumnIdentification: crtest.ColumnIdentification{
												Variants: awsAMD64OVNTest.Variants,
											},
										},
										TestComparison: testdetails.TestComparison{
											RequiredConfidence: 95,
											Comparison:         crtest.FisherExact,
											Explanations: []string{
												"Extreme regression detected.",
												"Fishers Exact probability of a regression: 100.00%.",
												"Test pass rate dropped from 90.00% to 50.00%.",
											},
											ReportStatus: crtest.ExtremeRegression,
											FisherExact:  thrift.Float64Ptr(1.0800451094957381e-20),
											SampleStats: testdetails.ReleaseStats{
												Stats: crtest.Stats{
													SuccessRate:  0.5,
													SuccessCount: 50,
													FailureCount: 49,
													FlakeCount:   1,
												},
												Start: &time.Time{},
												End:   &time.Time{},
											},
											BaseStats: &testdetails.ReleaseStats{
												Stats: crtest.Stats{
													SuccessRate:  0.9,
													SuccessCount: 900,
													FailureCount: 90,
													FlakeCount:   10,
												},
												Start: &time.Time{},
												End:   &time.Time{},
											},
										},
									},
									{
										Identification: crtest.Identification{
											RowIdentification: crtest.RowIdentification{
												TestName: awsAMD64OVN2BaseTestStats90Percent.TestName,
												TestID:   awsAMD64OVN2Test.TestID,
											},
											ColumnIdentification: crtest.ColumnIdentification{
												Variants: awsAMD64OVN2Test.Variants,
											},
										},
										TestComparison: testdetails.TestComparison{
											RequiredConfidence: 95,
											Comparison:         crtest.FisherExact,
											Explanations: []string{
												"Significant regression detected.",
												"Fishers Exact probability of a regression: 100.00%.",
												"Test pass rate dropped from 90.00% to 80.00%.",
											},
											ReportStatus: crtest.SignificantRegression,
											FisherExact:  thrift.Float64Ptr(0.0035097810890055117),
											SampleStats: testdetails.ReleaseStats{
												Stats: crtest.Stats{
													SuccessRate:  0.8,
													SuccessCount: 80,
													FailureCount: 19,
													FlakeCount:   1,
												},
												Start: &time.Time{},
												End:   &time.Time{},
											},
											BaseStats: &testdetails.ReleaseStats{
												Stats: crtest.Stats{
													SuccessRate:  0.9,
													SuccessCount: 900,
													FailureCount: 90,
													FlakeCount:   10,
												},
												Start: &time.Time{},
												End:   &time.Time{},
											},
										},
									},
								},
							},
							{
								ColumnIdentification: columnAWSAMD64SDN,
								Status:               crtest.MissingBasisAndSample,
							},
						},
					},
					{
						RowIdentification: rowComponent2,
						Columns: []crtype.ReportColumn{
							{
								ColumnIdentification: columnAWSAMD64OVN,
								Status:               crtest.MissingBasisAndSample,
							},
							{
								ColumnIdentification: columnAWSAMD64SDN,
								Status:               crtest.SignificantImprovement,
							},
						},
					},
				},
			},
		},
	}
	componentAndCapabilityGetter = fakeComponentAndCapabilityGetter
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report, err := tc.generator.generateComponentTestReport(tc.baseStatus, tc.sampleStatus)
			assert.NoError(t, err, "error generating component report")

			// WARNING: PC and Mac differ on floating point comparisons when you get far enough into the precision.
			// We need to do fuzzy floating point comparisons which poses a problem for the way these tests are
			// written to compare an entire report object. To avoid having to surgically compare everything, we
			// will first iterate all rows cols and regressed tests to compare any floating point vals we need to.
			// Then we nil them out and deep compare the rest of the object. This prevents any missed bugs where we
			// add new data to the report, but forget to explicitly compare it.
			assert.Equal(t, len(tc.expectedReport.Rows), len(report.Rows))
			for ir := range report.Rows {
				assert.Equal(t, tc.expectedReport.Rows[ir].RowIdentification, report.Rows[ir].RowIdentification)
				assert.Equal(t, len(tc.expectedReport.Rows[ir].Columns), len(report.Rows[ir].Columns))
				for ic := range report.Rows[ir].Columns {
					assert.Equal(t, len(tc.expectedReport.Rows[ir].Columns[ic].RegressedTests), len(report.Rows[ir].Columns[ic].RegressedTests))
					for it, regTest := range report.Rows[ir].Columns[ic].RegressedTests {
						assert.InDelta(t, *tc.expectedReport.Rows[ir].Columns[ic].RegressedTests[it].FisherExact, *regTest.FisherExact, 0.000001, regTest.TestName)
						tc.expectedReport.Rows[ir].Columns[ic].RegressedTests[it].FisherExact = nil
						report.Rows[ir].Columns[ic].RegressedTests[it].FisherExact = nil

					}
				}
			}
			assert.Equal(t, tc.expectedReport, report, "expected report %+v, got %+v", tc.expectedReport, report)
		})
	}
}

func TestGenerateComponentTestDetailsReport(t *testing.T) {
	prowJob1 := "ProwJob1"
	prowJob2 := "ProwJob2"
	type testStats struct {
		Success int
		Failure int
		Flake   int
	}
	type requiredJobStats struct {
		job string
		testStats
	}
	baseHighSuccessStats := testStats{
		Success: 1000,
		Failure: 100,
		Flake:   50,
	}
	baseLowSuccessStats := testStats{
		Success: 500,
		Failure: 600,
		Flake:   50,
	}
	sampleHighSuccessStats := testStats{
		Success: 100,
		Failure: 9,
		Flake:   4,
	}
	sampleLowSuccessStats := testStats{
		Success: 50,
		Failure: 59,
		Flake:   4,
	}
	testDetailsRowIdentification := crtest.RowIdentification{
		TestID:     testDetailsGenerator.ReqOptions.TestIDOptions[0].TestID,
		Component:  testDetailsGenerator.ReqOptions.TestIDOptions[0].Component,
		Capability: testDetailsGenerator.ReqOptions.TestIDOptions[0].Capability,
	}
	testDetailsColumnIdentification := crtest.ColumnIdentification{
		Variants: testDetailsGenerator.ReqOptions.TestIDOptions[0].RequestedVariants,
	}
	sampleReleaseStatsTwoHigh := testdetails.ReleaseStats{
		Release: testDetailsGenerator.ReqOptions.SampleRelease.Name,
		Stats: crtest.Stats{
			SuccessRate:  0.9203539823008849,
			SuccessCount: 200,
			FailureCount: 18,
			FlakeCount:   8,
		},
		Start: &time.Time{},
		End:   &time.Time{},
	}
	baseReleaseStatsTwoHigh := testdetails.ReleaseStats{
		Release: testDetailsGenerator.ReqOptions.BaseRelease.Name,
		Stats: crtest.Stats{
			SuccessRate:  0.9130434782608695,
			SuccessCount: 2000,
			FailureCount: 200,
			FlakeCount:   100,
		},
	}
	sampleTestStatsHigh := crtest.Stats{
		SuccessRate:  0.9203539823008849,
		SuccessCount: 100,
		FailureCount: 9,
		FlakeCount:   4,
	}
	baseTestStatsHigh := crtest.Stats{
		SuccessRate:  0.9130434782608695,
		SuccessCount: 1000,
		FailureCount: 100,
		FlakeCount:   50,
	}
	sampleTestStatsLow := crtest.Stats{
		SuccessRate:  0.4778761061946903,
		SuccessCount: 50,
		FailureCount: 59,
		FlakeCount:   4,
	}
	baseTestStatsLow := crtest.Stats{
		SuccessRate:  0.4782608695652174,
		SuccessCount: 500,
		FailureCount: 600,
		FlakeCount:   50,
	}
	sampleReleaseStatsOneHigh := testdetails.ReleaseStats{
		Release: testDetailsGenerator.ReqOptions.SampleRelease.Name,
		Stats: crtest.Stats{
			SuccessRate:  0.9203539823008849,
			SuccessCount: 100,
			FailureCount: 9,
			FlakeCount:   4,
		},
		Start: &time.Time{},
		End:   &time.Time{},
	}
	baseReleaseStatsOneHigh := testdetails.ReleaseStats{
		Release: testDetailsGenerator.ReqOptions.BaseRelease.Name,
		Stats: crtest.Stats{
			SuccessRate:  0.9130434782608695,
			SuccessCount: 1000,
			FailureCount: 100,
			FlakeCount:   50,
		},
	}
	sampleReleaseStatsOneLow := testdetails.ReleaseStats{
		Release: testDetailsGenerator.ReqOptions.SampleRelease.Name,
		Stats: crtest.Stats{
			SuccessRate:  0.4778761061946903,
			SuccessCount: 50,
			FailureCount: 59,
			FlakeCount:   4,
		},
		Start: &time.Time{},
		End:   &time.Time{},
	}
	baseReleaseStatsOneLow := testdetails.ReleaseStats{
		Release: testDetailsGenerator.ReqOptions.BaseRelease.Name,
		Stats: crtest.Stats{
			SuccessRate:  0.4782608695652174,
			SuccessCount: 500,
			FailureCount: 600,
			FlakeCount:   50,
		},
	}
	tests := []struct {
		name                    string
		generator               ComponentReportGenerator
		baseRequiredJobStats    []requiredJobStats
		sampleRequiredJobStats  []requiredJobStats
		expectedReport          testdetails.Report
		expectedSampleJobRunLen map[string]int
		expectedBaseJobRunLen   map[string]int
	}{
		{
			name:      "one job with high pass base and sample",
			generator: testDetailsGenerator,
			baseRequiredJobStats: []requiredJobStats{
				{
					job:       prowJob1,
					testStats: baseHighSuccessStats,
				},
			},
			sampleRequiredJobStats: []requiredJobStats{
				{
					job:       prowJob1,
					testStats: sampleHighSuccessStats,
				},
			},
			expectedReport: testdetails.Report{
				Identification: crtest.Identification{
					RowIdentification:    testDetailsRowIdentification,
					ColumnIdentification: testDetailsColumnIdentification,
				},
				Analyses: []testdetails.Analysis{
					{
						TestComparison: testdetails.TestComparison{
							Comparison:   crtest.FisherExact,
							SampleStats:  sampleReleaseStatsOneHigh,
							BaseStats:    &baseReleaseStatsOneHigh,
							FisherExact:  thrift.Float64Ptr(.4807457902463764),
							ReportStatus: crtest.NotSignificant,
						},
						JobStats: []testdetails.JobStats{
							{
								SampleJobName: prowJob1,
								SampleStats:   sampleTestStatsHigh,
								BaseStats:     baseTestStatsHigh,
								Significant:   false,
							},
						},
					},
				},
			},
			expectedSampleJobRunLen: map[string]int{
				prowJob1: 113,
			},
			expectedBaseJobRunLen: map[string]int{
				prowJob1: 1150,
			},
		},
		{
			name:      "one job with high base and low sample pass rate",
			generator: testDetailsGenerator,
			baseRequiredJobStats: []requiredJobStats{
				{
					job:       prowJob1,
					testStats: baseHighSuccessStats,
				},
			},
			sampleRequiredJobStats: []requiredJobStats{
				{
					job:       prowJob1,
					testStats: sampleLowSuccessStats,
				},
			},
			expectedReport: testdetails.Report{
				Identification: crtest.Identification{
					RowIdentification:    testDetailsRowIdentification,
					ColumnIdentification: testDetailsColumnIdentification,
				},
				Analyses: []testdetails.Analysis{
					{
						TestComparison: testdetails.TestComparison{
							Comparison:   crtest.FisherExact,
							SampleStats:  sampleReleaseStatsOneLow,
							BaseStats:    &baseReleaseStatsOneHigh,
							FisherExact:  thrift.Float64Ptr(8.209711662216515e-28),
							ReportStatus: crtest.ExtremeRegression,
						},
						JobStats: []testdetails.JobStats{
							{
								SampleJobName: prowJob1,
								BaseJobName:   prowJob2,
								SampleStats:   sampleTestStatsLow,
								BaseStats:     baseTestStatsHigh,
								Significant:   true,
							},
						},
					},
				},
			},
			expectedSampleJobRunLen: map[string]int{
				prowJob1: 113,
			},
			expectedBaseJobRunLen: map[string]int{
				prowJob1: 1150,
			},
		},
		{
			name:      "one job with low base and high sample pass rate",
			generator: testDetailsGenerator,
			baseRequiredJobStats: []requiredJobStats{
				{
					job:       prowJob1,
					testStats: baseLowSuccessStats,
				},
			},
			sampleRequiredJobStats: []requiredJobStats{
				{
					job:       prowJob1,
					testStats: sampleHighSuccessStats,
				},
			},
			expectedReport: testdetails.Report{
				Identification: crtest.Identification{
					RowIdentification:    testDetailsRowIdentification,
					ColumnIdentification: testDetailsColumnIdentification,
				},
				Analyses: []testdetails.Analysis{
					{
						TestComparison: testdetails.TestComparison{
							Comparison:   crtest.FisherExact,
							SampleStats:  sampleReleaseStatsOneHigh,
							BaseStats:    &baseReleaseStatsOneLow,
							FisherExact:  thrift.Float64Ptr(4.911246201592593e-22),
							ReportStatus: crtest.SignificantImprovement,
						},
						JobStats: []testdetails.JobStats{
							{
								SampleJobName: prowJob1,
								BaseJobName:   prowJob2,
								SampleStats:   sampleTestStatsHigh,
								BaseStats:     baseTestStatsLow,
								Significant:   false,
							},
						},
					},
				},
			},
			expectedSampleJobRunLen: map[string]int{
				prowJob1: 113,
			},
			expectedBaseJobRunLen: map[string]int{
				prowJob1: 1150,
			},
		},
		{
			name:      "two jobs with high pass rate",
			generator: testDetailsGenerator,
			baseRequiredJobStats: []requiredJobStats{
				{
					job:       prowJob1,
					testStats: baseHighSuccessStats,
				},
				{
					job:       prowJob2,
					testStats: baseHighSuccessStats,
				},
			},
			sampleRequiredJobStats: []requiredJobStats{
				{
					job:       prowJob1,
					testStats: sampleHighSuccessStats,
				},
				{
					job:       prowJob2,
					testStats: sampleHighSuccessStats,
				},
			},
			expectedReport: testdetails.Report{
				Identification: crtest.Identification{
					RowIdentification:    testDetailsRowIdentification,
					ColumnIdentification: testDetailsColumnIdentification,
				},
				Analyses: []testdetails.Analysis{
					{
						TestComparison: testdetails.TestComparison{
							Comparison:   crtest.FisherExact,
							SampleStats:  sampleReleaseStatsTwoHigh,
							BaseStats:    &baseReleaseStatsTwoHigh,
							FisherExact:  thrift.Float64Ptr(0.4119831376606586),
							ReportStatus: crtest.NotSignificant,
						},
						JobStats: []testdetails.JobStats{
							{
								SampleJobName: prowJob1,
								SampleStats:   sampleTestStatsHigh,
								BaseStats:     baseTestStatsHigh,
								Significant:   false,
							},
							{
								SampleJobName: prowJob2,
								SampleStats:   sampleTestStatsHigh,
								BaseStats:     baseTestStatsHigh,
								Significant:   false,
							},
						},
					},
				},
			},
			expectedSampleJobRunLen: map[string]int{
				prowJob1: 113,
				prowJob2: 113,
			},
			expectedBaseJobRunLen: map[string]int{
				prowJob1: 1150,
				prowJob2: 1150,
			},
		},
	}
	componentAndCapabilityGetter = fakeComponentAndCapabilityGetter
	for _, tc := range tests {
		baseStats := map[string][]bq.TestJobRunRows{}
		sampleStats := map[string][]bq.TestJobRunRows{}
		for _, testStats := range tc.baseRequiredJobStats {
			for i := 0; i < testStats.Success; i++ {
				baseStats[testStats.job] = append(baseStats[testStats.job], bq.TestJobRunRows{
					ProwJob: testStats.job,
					Count: crtest.Count{
						TotalCount:   1,
						SuccessCount: 1,
					},
				})
			}
			for i := 0; i < testStats.Failure; i++ {
				baseStats[testStats.job] = append(baseStats[testStats.job], bq.TestJobRunRows{
					ProwJob: testStats.job,
					Count:   crtest.Count{TotalCount: 1},
				})
			}
			for i := 0; i < testStats.Flake; i++ {
				baseStats[testStats.job] = append(baseStats[testStats.job], bq.TestJobRunRows{
					ProwJob: testStats.job,
					Count: crtest.Count{
						TotalCount: 1,
						FlakeCount: 1,
					},
				})
			}
		}
		for _, testStats := range tc.sampleRequiredJobStats {
			for i := 0; i < testStats.Success; i++ {
				sampleStats[testStats.job] = append(sampleStats[testStats.job], bq.TestJobRunRows{
					ProwJob: testStats.job,
					Count: crtest.Count{
						TotalCount:   1,
						SuccessCount: 1,
					},
				})
			}
			for i := 0; i < testStats.Failure; i++ {
				sampleStats[testStats.job] = append(sampleStats[testStats.job], bq.TestJobRunRows{
					ProwJob: testStats.job,
					Count:   crtest.Count{TotalCount: 1},
				})
			}
			for i := 0; i < testStats.Flake; i++ {
				sampleStats[testStats.job] = append(sampleStats[testStats.job], bq.TestJobRunRows{
					ProwJob: testStats.job,
					Count: crtest.Count{
						TotalCount: 1,
						FlakeCount: 1,
					},
				})
			}
		}

		t.Run(tc.name, func(t *testing.T) {
			report := tc.generator.internalGenerateTestDetailsReport("", nil, nil, baseStats, sampleStats, tc.generator.ReqOptions.TestIDOptions[0])
			assert.Equal(t, tc.expectedReport.RowIdentification, report.RowIdentification, "expected report row identification %+v, got %+v", tc.expectedReport.RowIdentification, report.RowIdentification)
			assert.Equal(t, tc.expectedReport.ColumnIdentification, report.ColumnIdentification, "expected report column identification %+v, got %+v", tc.expectedReport.ColumnIdentification, report.ColumnIdentification)
			assert.Equal(t, tc.expectedReport.Analyses[0].BaseStats, report.Analyses[0].BaseStats, "expected report base stats %+v, got %+v", tc.expectedReport.Analyses[0].BaseStats, report.Analyses[0].BaseStats)
			assert.Equal(t, tc.expectedReport.Analyses[0].SampleStats, report.Analyses[0].SampleStats, "expected report sample stats %+v, got %+v", tc.expectedReport.Analyses[0].SampleStats, report.Analyses[0].SampleStats)
			assert.Equal(t, fmt.Sprintf("%.4f", *tc.expectedReport.Analyses[0].FisherExact), fmt.Sprintf("%.4f", *report.Analyses[0].FisherExact), "expected fisher exact number %+v, got %+v", tc.expectedReport.Analyses[0].FisherExact, report.Analyses[0].FisherExact)
			assert.Equal(t, tc.expectedReport.Analyses[0].ReportStatus, report.Analyses[0].ReportStatus, "expected report status %+v, got %+v", tc.expectedReport.Analyses[0].ReportStatus, report.Analyses[0].ReportStatus)
			assert.Equal(t, len(tc.expectedReport.Analyses[0].JobStats), len(report.Analyses[0].JobStats), "expected len of job stats %+v, got %+v", len(tc.expectedReport.Analyses[0].JobStats), report.Analyses[0].JobStats)
			for i := range tc.expectedReport.Analyses[0].JobStats {
				jobName := report.Analyses[0].JobStats[i].SampleJobName
				assert.Equal(t, tc.expectedReport.Analyses[0].JobStats[i].SampleJobName, jobName, "expected job name %+v, got %+v", tc.expectedReport.Analyses[0].JobStats[i].SampleJobName, jobName)
				assert.Equal(t, tc.expectedReport.Analyses[0].JobStats[i].Significant, report.Analyses[0].JobStats[i].Significant, "expected per job significant %+v, got %+v", tc.expectedReport.Analyses[0].JobStats[i].Significant, report.Analyses[0].JobStats[i].Significant)
				assert.Equal(t, tc.expectedReport.Analyses[0].JobStats[i].BaseStats, report.Analyses[0].JobStats[i].BaseStats, "expected per job base stats for %s to be %+v, got %+v", tc.expectedReport.Analyses[0].JobStats[i].SampleJobName, tc.expectedReport.Analyses[0].JobStats[i].BaseStats, report.Analyses[0].JobStats[i].BaseStats)
				assert.Equal(t, tc.expectedReport.Analyses[0].JobStats[i].SampleStats, report.Analyses[0].JobStats[i].SampleStats, "expected per job sample stats for %s to be %+v, got %+v", tc.expectedReport.Analyses[0].JobStats[i].SampleJobName, tc.expectedReport.Analyses[0].JobStats[i].SampleStats, report.Analyses[0].JobStats[i].SampleStats)
				assert.Equal(t, tc.expectedSampleJobRunLen[jobName], len(report.Analyses[0].JobStats[i].SampleJobRunStats), "expected sample job run counts %+v, got %+v", tc.expectedSampleJobRunLen[jobName], len(report.Analyses[0].JobStats[i].SampleJobRunStats))
				assert.Equal(t, tc.expectedBaseJobRunLen[jobName], len(report.Analyses[0].JobStats[i].BaseJobRunStats), "expected base job run counts %+v, got %+v", tc.expectedBaseJobRunLen[jobName], len(report.Analyses[0].JobStats[i].BaseJobRunStats))
			}
			// assert.Equal(t, tc.expectedReport.ReportStatus, report.ReportStatus, "expected report %+v, got %+v", tc.expectedReport, report)
			// output, _ := json.MarshalIndent(report, "", "    ")
			// fmt.Printf("-----report \n%+v\n", string(output))
		})
	}
}

func Test_componentReportGenerator_normalizeProwJobName(t *testing.T) {
	tests := []struct {
		name          string
		sampleRelease string
		baseRelease   string
		jobName       string
		want          string
	}{
		{
			name:        "base release is removed",
			baseRelease: "4.16",
			jobName:     "periodic-ci-openshift-release-master-ci-4.16-e2e-azure-ovn-upgrade",
			want:        "periodic-ci-openshift-release-master-ci-X.X-e2e-azure-ovn-upgrade",
		},
		{
			name:          "sample release is removed",
			sampleRelease: "4.16",
			jobName:       "periodic-ci-openshift-release-master-ci-4.16-e2e-azure-ovn-upgrade",
			want:          "periodic-ci-openshift-release-master-ci-X.X-e2e-azure-ovn-upgrade",
		},
		{
			name:    "frequency is removed",
			jobName: "periodic-ci-openshift-release-master-ci-test-job-f27",
			want:    "periodic-ci-openshift-release-master-ci-test-job-fXX",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ComponentReportGenerator{}
			if tt.baseRelease != "" {
				c.ReqOptions.BaseRelease = reqopts.Release{Name: tt.baseRelease}
			}
			if tt.sampleRelease != "" {
				c.ReqOptions.SampleRelease = reqopts.Release{Name: tt.sampleRelease}
			}

			assert.Equalf(t, tt.want, utils.NormalizeProwJobName(tt.jobName, c.ReqOptions), "normalizeProwJobName(%v)", tt.jobName)
		})
	}
}

func Test_componentReportGenerator_assessComponentStatus(t *testing.T) {
	tests := []struct {
		name          string
		sampleTotal   int
		sampleSuccess int
		sampleFlake   int
		baseTotal     int
		baseSuccess   int
		baseFlake     int

		requiredPassRateForNewTests int
		requiredPassRateForAllTests int
		minFail                     int

		expectedStatus   crtest.Status
		expectedFischers *float64
	}{
		{
			name:             "regular regression",
			sampleTotal:      15,
			sampleSuccess:    13,
			sampleFlake:      0,
			baseTotal:        15,
			baseSuccess:      14,
			baseFlake:        1,
			expectedStatus:   -400,
			expectedFischers: thrift.Float64Ptr(0.2413793103448262),
		},
		{
			name:             "zero success",
			sampleTotal:      15,
			sampleSuccess:    0,
			sampleFlake:      0,
			baseTotal:        15,
			baseSuccess:      14,
			baseFlake:        1,
			expectedStatus:   -500,
			expectedFischers: thrift.Float64Ptr(6.446725037893782e-09),
		},
		{
			name:                        "new test no regression",
			sampleTotal:                 1000,
			sampleSuccess:               999,
			requiredPassRateForNewTests: 99,
			expectedStatus:              crtest.MissingBasis,
			expectedFischers:            nil,
		},
		{
			name:                        "new test extreme regression",
			sampleTotal:                 15,
			sampleSuccess:               13,
			requiredPassRateForNewTests: 99,
			expectedStatus:              crtest.ExtremeRegression,
			expectedFischers:            nil,
		},
		{
			name:                        "new test significant regression",
			sampleTotal:                 1000,
			sampleSuccess:               985,
			requiredPassRateForNewTests: 99,
			expectedStatus:              crtest.SignificantRegression,
		},
		{
			name:                        "pass rate mode significant regression",
			sampleTotal:                 100,
			sampleSuccess:               94,
			sampleFlake:                 0,
			baseTotal:                   100,
			baseSuccess:                 94,
			baseFlake:                   0,
			requiredPassRateForAllTests: 95,
			expectedStatus:              crtest.SignificantRegression,
		},
		{
			name:                        "pass rate mode extreme regression",
			sampleTotal:                 100,
			sampleSuccess:               89,
			sampleFlake:                 0,
			baseTotal:                   100,
			baseSuccess:                 89,
			baseFlake:                   0,
			requiredPassRateForAllTests: 95,
			expectedStatus:              crtest.ExtremeRegression,
		},
		{
			name:                        "pass rate mode no regression",
			sampleTotal:                 100,
			sampleSuccess:               97,
			sampleFlake:                 0,
			baseTotal:                   100,
			baseSuccess:                 97,
			baseFlake:                   0,
			requiredPassRateForAllTests: 95,
			expectedStatus:              crtest.NotSignificant,
		},
		{
			name:                        "pass rate mode significant regression under minimum failures",
			sampleTotal:                 20,
			sampleSuccess:               18,
			sampleFlake:                 0,
			baseTotal:                   20,
			baseSuccess:                 18,
			baseFlake:                   0,
			requiredPassRateForAllTests: 95,
			minFail:                     5,
			expectedStatus:              crtest.NotSignificant,
		},
		{
			name:                        "pass rate mode significant regression over minimum failures",
			sampleTotal:                 20,
			sampleSuccess:               18,
			sampleFlake:                 0,
			baseTotal:                   20,
			baseSuccess:                 18,
			baseFlake:                   0,
			requiredPassRateForAllTests: 95,
			minFail:                     1,
			expectedStatus:              crtest.SignificantRegression,
		},
		{
			name:                        "pass rate mode insufficient runs to trigger",
			sampleTotal:                 6,
			sampleSuccess:               0,
			sampleFlake:                 0,
			requiredPassRateForAllTests: 95,
			expectedStatus:              crtest.NotSignificant,
		},
		{
			name:                        "pass rate mode barely sufficient runs to trigger",
			sampleTotal:                 7,
			sampleSuccess:               6,
			sampleFlake:                 0,
			requiredPassRateForAllTests: 95,
			expectedStatus:              crtest.ExtremeRegression,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ComponentReportGenerator{}
			c.ReqOptions.AdvancedOption.PassRateRequiredNewTests = tt.requiredPassRateForNewTests
			c.ReqOptions.AdvancedOption.PassRateRequiredAllTests = tt.requiredPassRateForAllTests
			c.ReqOptions.AdvancedOption.MinimumFailure = tt.minFail

			testAnalysis := &testdetails.TestComparison{
				SampleStats: testdetails.ReleaseStats{
					Stats: crtest.Stats{
						SuccessCount: tt.sampleSuccess,
						FlakeCount:   tt.sampleFlake,
						FailureCount: tt.sampleTotal - tt.sampleSuccess - tt.sampleFlake,
					},
				},
				BaseStats: &testdetails.ReleaseStats{
					Stats: crtest.Stats{
						SuccessCount: tt.baseSuccess,
						FlakeCount:   tt.baseFlake,
						FailureCount: tt.baseTotal - tt.baseSuccess - tt.baseFlake,
					},
				},
			}

			c.assessComponentStatus(testAnalysis)
			assert.Equalf(t, tt.expectedStatus, testAnalysis.ReportStatus, "assessComponentStatus expected status not equal")
			if tt.expectedFischers != nil {
				// Mac and Linux do not matchup on floating point precision, so lets approximate the comparison:
				assert.Equalf(t,
					fmt.Sprintf("%.4f", *tt.expectedFischers),
					fmt.Sprintf("%.4f", *testAnalysis.FisherExact),
					"assessComponentStatus expected fischers value not equal")
			} else {
				assert.Nil(t, testAnalysis.FisherExact)
			}

		})
	}
}

func TestCopyIncludeVariantsAndRemoveOverrides(t *testing.T) {
	tests := []struct {
		name              string
		overrides         []v1.VariantJunitTableOverride
		currOverride      int
		includeVariants   map[string][]string
		expected          map[string][]string
		expectedSkipQuery bool
	}{
		{
			name:         "No overrides, no variants removed",
			overrides:    []v1.VariantJunitTableOverride{},
			currOverride: -1,
			includeVariants: map[string][]string{
				"key1": {"value1", "value2"},
				"key2": {"value3"},
			},
			expected: map[string][]string{
				"key1": {"value1", "value2"},
				"key2": {"value3"},
			},
		},
		{
			name: "Single override removes matching variant",
			overrides: []v1.VariantJunitTableOverride{
				{VariantName: "key1", VariantValue: "value1"},
			},
			currOverride: -1,
			includeVariants: map[string][]string{
				"key1": {"value1", "value2"},
				"key2": {"value3"},
			},
			expected: map[string][]string{
				"key1": {"value2"},
				"key2": {"value3"},
			},
		},
		{
			name: "Override does not remove its own variant",
			overrides: []v1.VariantJunitTableOverride{
				{VariantName: "key1", VariantValue: "value1"},
			},
			currOverride: 0,
			includeVariants: map[string][]string{
				"key1": {"value1", "value2"},
				"key2": {"value3"},
			},
			expected: map[string][]string{
				"key1": {"value1", "value2"},
				"key2": {"value3"},
			},
		},
		{
			name: "Multiple overrides remove multiple variants",
			overrides: []v1.VariantJunitTableOverride{
				{VariantName: "key1", VariantValue: "value1"},
				{VariantName: "key2", VariantValue: "value3"},
			},
			currOverride: -1,
			includeVariants: map[string][]string{
				"key1": {"value1", "value2"},
				"key2": {"value3", "value4"},
			},
			expected: map[string][]string{
				"key1": {"value2"},
				"key2": {"value4"},
			},
		},
		{
			name: "All variants removed",
			overrides: []v1.VariantJunitTableOverride{
				{VariantName: "key1", VariantValue: "value1"},
				{VariantName: "key1", VariantValue: "value2"},
				{VariantName: "key2", VariantValue: "value3"},
			},
			currOverride: -1,
			includeVariants: map[string][]string{
				"key1": {"value1", "value2"},
				"key2": {"value3"},
			},
			expected:          map[string][]string{},
			expectedSkipQuery: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, skipQuery := copyIncludeVariantsAndRemoveOverrides(tt.overrides, tt.currOverride, tt.includeVariants)
			assert.Equal(t, tt.expectedSkipQuery, skipQuery)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
