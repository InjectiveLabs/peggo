module github.com/InjectiveLabs/peggo/orchestrator

go 1.15

require (
	github.com/InjectiveLabs/sdk-go v1.12.10
	github.com/alexcesaro/statsd v2.0.0+incompatible
	github.com/bugsnag/panicwrap v1.3.0 // indirect
	github.com/cosmos/cosmos-sdk v0.40.0
	github.com/ethereum/go-ethereum v1.9.25
	github.com/gofrs/uuid v4.0.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.1
	github.com/jawher/mow.cli v1.2.0
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.14.0
	github.com/shopspring/decimal v1.2.0
	github.com/tendermint/tendermint v0.34.0-rc6
	github.com/xlab/closer v0.0.0-20190328110542-03326addb7c2
	github.com/xlab/suplog v1.1.0
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
	google.golang.org/genproto v0.0.0-20201214200347-8c77b98c765d
	google.golang.org/grpc v1.34.0
)

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.2-alpha.regen.4

replace github.com/cosmos/cosmos-sdk => github.com/InjectiveLabs/cosmos-sdk v0.40.0-fix8

replace github.com/ethereum/go-ethereum => github.com/InjectiveLabs/go-ethereum v1.9.22
