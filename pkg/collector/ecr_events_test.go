package collector

//
//import (
//	"errors"
//	"testing"
//
//	"github.com/aws/aws-sdk-go/aws"
//	"github.com/aws/aws-sdk-go/service/ecr"
//	"github.com/stretchr/testify/assert"
//	"github.com/stretchr/testify/suite"
//)
//
//var testPackageDetails = map[string]*ecr.ImageScanFinding{
//	"SEVERITY_UNSPECIFIED": {
//		Name:     aws.String("CVE-2020-8177"),
//		Severity: aws.String("SEVERITY_UNSPECIFIED"),
//		Uri:      aws.String("https://security-tracker.debian.org/tracker/CVE-2020-8177"),
//		Attributes: []*ecr.Attribute{
//			{
//				Key:   aws.String("package_version"),
//				Value: aws.String("7.64.0-4+deb10u1"),
//			},
//			{
//				Key:   aws.String("package_name"),
//				Value: aws.String("curl"),
//			},
//		},
//	},
//	"MINIMAL": {
//		Name:     aws.String("CVE-2017-9117"),
//		Severity: aws.String("INFORMATIONAL"),
//		Uri:      aws.String("https://security-tracker.debian.org/tracker/CVE-2017-9117"),
//		Attributes: []*ecr.Attribute{
//			{
//				Key:   aws.String("package_version"),
//				Value: aws.String("4.1.0+git191117-2~deb10u1"),
//			},
//			{
//				Key:   aws.String("package_name"),
//				Value: aws.String("tiff"),
//			},
//		},
//	},
//	"LOW": {
//		Name:     aws.String("CVE-2020-10029"),
//		Severity: aws.String("LOW"),
//		Uri:      aws.String("https://security-tracker.debian.org/tracker/CVE-2020-10878"),
//		Attributes: []*ecr.Attribute{
//			{
//				Key:   aws.String("package_version"),
//				Value: aws.String("2.28-10"),
//			},
//			{
//				Key:   aws.String("package_name"),
//				Value: aws.String("glibc"),
//			},
//		},
//	},
//	"MEDIUM": {
//		Name:     aws.String("CVE-2019-3844"),
//		Severity: aws.String("MEDIUM"),
//		Uri:      aws.String("https://security-tracker.debian.org/tracker/CVE-2019-3844"),
//		Attributes: []*ecr.Attribute{
//			{
//				Key:   aws.String("package_version"),
//				Value: aws.String("241-7~deb10u4"),
//			},
//			{
//				Key:   aws.String("package_name"),
//				Value: aws.String("systemd"),
//			},
//		},
//	},
//	"HIGH": {
//		Name:     aws.String("CVE-2020-10878"),
//		Severity: aws.String("HIGH"),
//		Uri:      aws.String("https://security-tracker.debian.org/tracker/CVE-2020-10878"),
//		Attributes: []*ecr.Attribute{
//			{
//				Key:   aws.String("package_version"),
//				Value: aws.String("5.28.1-6"),
//			},
//			{
//				Key:   aws.String("package_name"),
//				Value: aws.String("perl"),
//			},
//		},
//	},
//	"CRITICAL": {
//		Name:     aws.String("CVE-2019-12400"),
//		Severity: aws.String("CRITICAL"),
//		Uri:      aws.String("https://security-tracker.debian.org/tracker/CVE-2019-12400"),
//		Attributes: []*ecr.Attribute{
//			{
//				Key:   aws.String("package_version"),
//				Value: aws.String("1.5.8-2"),
//			},
//			{
//				Key:   aws.String("package_name"),
//				Value: aws.String("libxml-security-java"),
//			},
//		},
//	},
//}
//
//var testFindingsOutput = map[string]*ecr.DescribeImageScanFindingsOutput{
//	"zeroVulnerabilities": {
//		ImageScanStatus: &ecr.ImageScanStatus{
//			Status: aws.String(ecr.ScanStatusComplete),
//		},
//		ImageScanFindings: &ecr.ImageScanFindings{
//			FindingSeverityCounts: map[string]*int64{
//				ecr.FindingSeverityUndefined:     aws.Int64(0),
//				ecr.FindingSeverityInformational: aws.Int64(0),
//				ecr.FindingSeverityLow:           aws.Int64(0),
//				ecr.FindingSeverityMedium:        aws.Int64(0),
//				ecr.FindingSeverityHigh:          aws.Int64(0),
//				ecr.FindingSeverityCritical:      aws.Int64(0),
//			},
//		},
//	},
//	"singleVulnerability": {
//		ImageScanStatus: &ecr.ImageScanStatus{
//			Status: aws.String(ecr.ScanStatusComplete),
//		},
//		ImageScanFindings: &ecr.ImageScanFindings{
//			FindingSeverityCounts: map[string]*int64{
//				ecr.FindingSeverityUndefined:     aws.Int64(0),
//				ecr.FindingSeverityInformational: aws.Int64(0),
//				ecr.FindingSeverityLow:           aws.Int64(0),
//				ecr.FindingSeverityMedium:        aws.Int64(0),
//				ecr.FindingSeverityHigh:          aws.Int64(0),
//				ecr.FindingSeverityCritical:      aws.Int64(1),
//			},
//			Findings: []*ecr.ImageScanFinding{
//				testPackageDetails["CRITICAL"],
//			},
//		},
//	},
//	"manyVulnerabilities": {
//		ImageScanStatus: &ecr.ImageScanStatus{
//			Status: aws.String(ecr.ScanStatusComplete),
//		},
//		ImageScanFindings: &ecr.ImageScanFindings{
//			FindingSeverityCounts: map[string]*int64{
//				ecr.FindingSeverityUndefined:     aws.Int64(1),
//				ecr.FindingSeverityInformational: aws.Int64(1),
//				ecr.FindingSeverityLow:           aws.Int64(1),
//				ecr.FindingSeverityMedium:        aws.Int64(1),
//				ecr.FindingSeverityHigh:          aws.Int64(1),
//				ecr.FindingSeverityCritical:      aws.Int64(1),
//			},
//			Findings: []*ecr.ImageScanFinding{
//				testPackageDetails["SEVERITY_UNSPECIFIED"],
//				testPackageDetails["MINIMAL"],
//				testPackageDetails["LOW"],
//				testPackageDetails["MEDIUM"],
//				testPackageDetails["HIGH"],
//				testPackageDetails["CRITICAL"],
//			},
//		},
//	},
//}
//
//type ECRTestSuite struct {
//	suite.Suite
//}
//
//type mockECRClient struct{}
//
//func (m *mockECRClient) DescribeImageScanFindings(input *ecr.DescribeImageScanFindingsInput) (*ecr.DescribeImageScanFindingsOutput, error) {
//	if _, ok := testFindingsOutput[*input.ImageId.ImageDigest]; ok {
//		return testFindingsOutput[*input.ImageId.ImageDigest], nil
//	}
//
//	return nil, errors.New("error")
//}
//
//func (suite *ECRTestSuite) TestGetNoVulnerabilityDetail() {
//	client := &mockECRClient{}
//
//	testECRDetail := &ECRImageScanDetail{
//		ScanStatus:     ecr.ScanStatusComplete,
//		RepositoryName: "testRepo",
//		ImageDigest:    "zeroVulnerabilities",
//	}
//
//	vulnerabilityOccurences, _ := getVulnerabilityDetails(client, testECRDetail)
//
//	assert.Len(suite.T(), vulnerabilityOccurences, 0)
//}
//
//func (suite *ECRTestSuite) TestGetOneVulnerabilityDetail() {
//	client := &mockECRClient{}
//
//	testECRDetail := &ECRImageScanDetail{
//		ScanStatus:     ecr.ScanStatusComplete,
//		RepositoryName: "testRepo",
//		ImageDigest:    "singleVulnerability",
//	}
//
//	result, _ := getVulnerabilityDetails(client, testECRDetail)
//
//	assert.Len(suite.T(), result, 1)
//	assert.Equal(suite.T(), result[0].Vulnerability.Severity.String(), *testPackageDetails["CRITICAL"].Severity)
//	assert.Equal(suite.T(), result[0].Vulnerability.PackageIssue[0].AffectedLocation.CpeUri, *testPackageDetails["CRITICAL"].Uri)
//	assert.Equal(suite.T(), result[0].Vulnerability.PackageIssue[0].AffectedLocation.Version.Name, *testPackageDetails["CRITICAL"].Attributes[0].Value)
//	assert.Equal(suite.T(), result[0].Vulnerability.PackageIssue[0].AffectedLocation.Package, *testPackageDetails["CRITICAL"].Attributes[1].Value)
//}
//
//func (suite *ECRTestSuite) TestGetMultipleVulnerabilityDetail() {
//	client := &mockECRClient{}
//
//	testECRDetail := &ECRImageScanDetail{
//		ScanStatus:     ecr.ScanStatusComplete,
//		RepositoryName: "testRepo",
//		ImageDigest:    "manyVulnerabilities",
//	}
//
//	result, _ := getVulnerabilityDetails(client, testECRDetail)
//
//	assert.Len(suite.T(), result, 6)
//	for i := range result {
//		s := result[i].Vulnerability.Severity.String()
//		assert.Equal(suite.T(), result[i].Vulnerability.PackageIssue[0].AffectedLocation.CpeUri, *testPackageDetails[s].Uri)
//		assert.Equal(suite.T(), result[i].Vulnerability.PackageIssue[0].AffectedLocation.Version.Name, *testPackageDetails[s].Attributes[0].Value)
//	}
//}
//
//func (suite *ECRTestSuite) TestBadInput() {
//	client := &mockECRClient{}
//
//	testECRDetail := &ECRImageScanDetail{
//		ScanStatus:     ecr.ScanStatusComplete,
//		RepositoryName: "testRepo",
//		ImageDigest:    "nonExistentImage",
//	}
//
//	_, err := getVulnerabilityDetails(client, testECRDetail)
//
//	assert.Error(suite.T(), err)
//}
//func TestECRTestSuite(t *testing.T) {
//	suite.Run(t, new(ECRTestSuite))
//}
