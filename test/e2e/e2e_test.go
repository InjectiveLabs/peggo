package e2e

import (
	"context"
	"fmt"
	"time"
)

func (s *IntegrationTestSuite) TestPhotonTokenTransfers() {
	// deploy photon ERC20 token contact
	var photonERC20Addr string
	s.Run("deploy_photon_erc20", func() {
		photonERC20Addr = s.deployERC20Token("photon")
	})

	// send 100 photon tokens from Umee to Ethereum
	s.Run("send_photon_tokens_to_eth", func() {
		ethRecipient := s.chain.validators[1].ethereumKey.address
		s.sendFromUmeeToEth(0, ethRecipient, "100photon", "10photon", "3photon")

		umeeEndpoint := fmt.Sprintf("http://%s", s.valResources[0].GetHostPort("1317/tcp"))
		fromAddr := s.chain.validators[0].keyInfo.GetAddress()

		// require the sender's (validator) balance decreased
		balance, err := queryUmeeDenomBalance(umeeEndpoint, fromAddr.String(), "photon")
		s.Require().NoError(err)
		s.Require().GreaterOrEqual(balance.Amount.Int64(), int64(99999998237))

		// require the Ethereum recipient balance increased
		var latestBalance int
		s.Require().Eventuallyf(
			func() bool {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()

				b, err := queryEthTokenBalance(ctx, s.ethClient, photonERC20Addr, ethRecipient)
				if err != nil {
					return false
				}

				latestBalance = b

				// The balance could differ if the receiving address was the orchestrator
				// that sent the batch tx and got the gravity fee.
				return b >= 100 && b <= 103
			},
			2*time.Minute,
			5*time.Second,
			"unexpected balance: %d", latestBalance,
		)
	})

	// send 100 photon tokens from Ethereum back to Umee
	s.Run("send_photon_tokens_from_eth", func() {
		toAddr := s.chain.validators[0].keyInfo.GetAddress()
		s.sendFromEthToUmee(1, photonERC20Addr, toAddr.String(), "100")

		umeeEndpoint := fmt.Sprintf("http://%s", s.valResources[0].GetHostPort("1317/tcp"))
		expBalance := int64(99999998334)

		// require the original sender's (validator) balance increased
		var latestBalance int64
		s.Require().Eventuallyf(
			func() bool {
				b, err := queryUmeeDenomBalance(umeeEndpoint, toAddr.String(), "photon")
				if err != nil {
					return false
				}

				latestBalance = b.Amount.Int64()

				return latestBalance >= expBalance
			},
			2*time.Minute,
			5*time.Second,
			"unexpected balance: %d", latestBalance,
		)
	})
}

func (s *IntegrationTestSuite) TestUmeeTokenTransfers() {
	// deploy umee ERC20 token contract
	var umeeERC20Addr string
	s.Run("deploy_umee_erc20", func() {
		umeeERC20Addr = s.deployERC20Token("uumee")
	})

	// send 300 umee tokens from Umee to Ethereum
	s.Run("send_uumee_tokens_to_eth", func() {
		ethRecipient := s.chain.validators[1].ethereumKey.address
		s.sendFromUmeeToEth(0, ethRecipient, "300uumee", "10photon", "7uumee")

		endpoint := fmt.Sprintf("http://%s", s.valResources[0].GetHostPort("1317/tcp"))
		fromAddr := s.chain.validators[0].keyInfo.GetAddress()

		balance, err := queryUmeeDenomBalance(endpoint, fromAddr.String(), "uumee")
		s.Require().NoError(err)
		s.Require().Equal(int64(9999999693), balance.Amount.Int64())

		// require the Ethereum recipient balance increased
		var latestBalance int
		s.Require().Eventuallyf(
			func() bool {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()

				b, err := queryEthTokenBalance(ctx, s.ethClient, umeeERC20Addr, ethRecipient)
				if err != nil {
					return false
				}

				latestBalance = b

				// The balance could differ if the receiving address was the orchestrator
				// that sent the batch tx and got the gravity fee.
				return b >= 300 && b <= 307
			},
			5*time.Minute,
			5*time.Second,
			"unexpected balance: %d", latestBalance,
		)
	})

	// send 300 umee tokens from Ethereum back to Umee
	s.Run("send_uumee_tokens_from_eth", func() {
		toAddr := s.chain.validators[0].keyInfo.GetAddress()
		s.sendFromEthToUmee(1, umeeERC20Addr, toAddr.String(), "300")

		umeeEndpoint := fmt.Sprintf("http://%s", s.valResources[0].GetHostPort("1317/tcp"))
		expBalance := int64(9999999993)

		// require the original sender's (validator) balance increased
		var latestBalance int64
		s.Require().Eventuallyf(
			func() bool {
				b, err := queryUmeeDenomBalance(umeeEndpoint, toAddr.String(), "uumee")
				if err != nil {
					return false
				}

				latestBalance = b.Amount.Int64()

				return latestBalance == expBalance
			},
			2*time.Minute,
			5*time.Second,
			"unexpected balance: %d", latestBalance,
		)
	})
}
