module github.com/InjectiveLabs/peggo

go 1.16

require (
	github.com/InjectiveLabs/etherman v1.7.0
	github.com/InjectiveLabs/metrics v0.0.1
	github.com/InjectiveLabs/sdk-go v1.47.6
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/cometbft/cometbft v0.37.1
	github.com/cosmos/cosmos-sdk v0.47.2
	github.com/ethereum/go-ethereum v1.11.5
	github.com/hashicorp/go-multierror v1.1.1
	github.com/jawher/mow.cli v1.2.0
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.20.0
	github.com/pkg/errors v0.9.1
	github.com/shirou/gopsutil v3.21.6+incompatible // indirect
	github.com/shopspring/decimal v1.2.0
	github.com/stretchr/testify v1.8.3
	github.com/xlab/closer v0.0.0-20190328110542-03326addb7c2
	github.com/xlab/suplog v1.3.1
	golang.org/x/crypto v0.7.0
	google.golang.org/grpc v1.54.0
)

replace (
	github.com/CosmWasm/wasmd => github.com/InjectiveLabs/wasmd v0.40.0-inj
	github.com/bandprotocol/bandchain-packet => github.com/InjectiveLabs/bandchain-packet v0.0.4-0.20230327115226-35199d4659d5
	github.com/cometbft/cometbft => github.com/InjectiveLabs/cometbft v0.37.1-inj
	github.com/cosmos/cosmos-sdk => github.com/InjectiveLabs/cosmos-sdk v0.47.2-inj-2
	github.com/ethereum/go-ethereum => github.com/ethereum/go-ethereum v1.12.0

)
