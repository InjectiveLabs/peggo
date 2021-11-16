package solidity

import (
	"os"
	"strings"
	"testing"

	"github.com/InjectiveLabs/etherman/deployer"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	ContractDeployer deployer.Deployer
	CoverageEnabled  bool
	CoverageAgent    deployer.CoverageDataCollector
)

func TestPeggo(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeSuite(func() {
		CoverageEnabled = toBool(os.Getenv("PEGGO_TEST_COVERAGE")) || toBool(os.Getenv("COVERAGE"))

		if CoverageEnabled {
			CoverageAgent = deployer.NewCoverageDataCollector(
				deployer.CoverageMode(os.Getenv("PEGGO_TEST_COVERAGE_MODE")),
			)
		}

		d, err := deployer.New(
			deployer.OptionEVMRPCEndpoint(os.Getenv("PEGGO_TEST_EVM_RPC")),
			deployer.OptionGasLimit(10000000),
			deployer.OptionEnableCoverage(CoverageEnabled),
		)
		orFail(err)

		ContractDeployer = d
	})

	AfterSuite(func() {
		if CoverageEnabled {
			var outFile *os.File = nil
			var err error

			contractNames := []string{"Peggy", "HashingTest"}
			coverageOut := os.Getenv("PEGGO_TEST_COVERAGE_OUT")

			if len(coverageOut) > 0 && strings.HasSuffix(coverageOut, ".html") {
				outFile, err = os.Create(coverageOut)
				orFail(err)
				defer outFile.Close()

				err = CoverageAgent.ReportHTML(outFile, contractNames...)
				orFail(err)

				return
			} else if len(coverageOut) > 0 {
				outFile, err = os.Create(coverageOut)
				orFail(err)
				defer outFile.Close()

				err = CoverageAgent.ReportTextCoverfile(outFile, contractNames...)
				orFail(err)

				return
			} else {
				_ = CoverageAgent.ReportHTML(nil, contractNames...)
			}
		}
	})

	RunSpecs(t, "Peggo Test Suite")
}
