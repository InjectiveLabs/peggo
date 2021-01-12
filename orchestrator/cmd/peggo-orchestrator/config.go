package main

import (
	"strings"
	"time"

	cli "github.com/jawher/mow.cli"
	log "github.com/xlab/suplog"
)

var (
	envName              *string
	appLogLevel          *string
	svcWaitTimeout       *string
	evmNodeHTTP          *string
	injectiveProtoAddr   *string
	fetchIntervalSeconds *string
	priceFeederFromPK    *string
	yfiVaultAddress      *string
	statsdPrefix         *string
	statsdAddr           *string
	statsdStuckDur       *string
	statsdMocking        *string
	statsdDisabled       *string
)

func initFlags() {
	envName = app.String(cli.StringOpt{
		Name:   "env",
		Desc:   "Application environment",
		EnvVar: "APP_ENV",
		Value:  "local",
	})

	appLogLevel = app.String(cli.StringOpt{
		Name:   "l log-level",
		Desc:   "Available levels: error, warn, info, debug.",
		EnvVar: "APP_LOG_LEVEL",
		Value:  "info",
	})

	svcWaitTimeout = app.String(cli.StringOpt{
		Name:   "svc-wait-timeout",
		Desc:   "Standard wait timeout for all service dependencies (e.g. injectived).",
		EnvVar: "SERVICE_WAIT_TIMEOUT",
		Value:  "1m",
	})

	evmNodeHTTP = app.String(cli.StringOpt{
		Name:   "evm-node-http",
		Desc:   "Specify HTTP endpoint for an EVM node.",
		EnvVar: "EVM_RPC_HTTP",
		Value:  "http://localhost:1317",
	})

	injectiveProtoAddr = app.String(cli.StringOpt{
		Name:   "injectived-grpc-addr",
		Desc:   "Specify GRPC address of the injectived service.",
		EnvVar: "INJECTIVED_GRPC_ADDR",
		Value:  "tcp://localhost:9900",
	})

	fetchIntervalSeconds = app.String(cli.StringOpt{
		Name:   "F fetch-interval",
		Desc:   "Specify price update interval (example: 60s)",
		EnvVar: "PRICE_FETCH_INTERVAL",
		Value:  "60s",
	})

	yfiVaultAddress = app.String(cli.StringOpt{
		Name:   "yfi-vault-address",
		Desc:   "Address for Yfi Vault index amount",
		EnvVar: "YFI_VAULT_ADDRESS",
		Value:  "0x07A8fA2531aab1eA8D6a50E8a81069b370ed24BE",
	})

	priceFeederFromPK = app.String(cli.StringOpt{
		Name:   "from-pk",
		Desc:   "Sender private key (Ex: 5D862464FE95...)",
		EnvVar: "PRICE_FEEDER_PRIVATE_KEY",
		Value:  "",
	})

	statsdPrefix = app.String(cli.StringOpt{
		Name:   "statsd-prefix",
		Desc:   "Specify StatsD compatible metrics prefix.",
		EnvVar: "STATSD_PREFIX",
		Value:  "relayer_api",
	})
	statsdAddr = app.String(cli.StringOpt{
		Name:   "statsd-addr",
		Desc:   "UDP address of a StatsD compatible metrics aggregator.",
		EnvVar: "STATSD_ADDR",
		Value:  "localhost:8125",
	})
	statsdStuckDur = app.String(cli.StringOpt{
		Name:   "statsd-stuck-func",
		Desc:   "Sets a duration to consider a function to be stuck (e.g. in deadlock).",
		EnvVar: "STATSD_STUCK_DUR",
		Value:  "5m",
	})
	statsdMocking = app.String(cli.StringOpt{
		Name:   "statsd-mocking",
		Desc:   "If enabled replaces statsd client with a mock one that simply logs values.",
		EnvVar: "STATSD_MOCKING",
		Value:  "false",
	})
	statsdDisabled = app.String(cli.StringOpt{
		Name:   "statsd-disabled",
		Desc:   "Force disabling statsd reporting completely.",
		EnvVar: "STATSD_DISABLED",
		Value:  "false",
	})
}

func Level(s string) log.Level {
	switch s {
	case "1", "error":
		return log.ErrorLevel
	case "2", "warn":
		return log.WarnLevel
	case "3", "info":
		return log.InfoLevel
	case "4", "debug":
		return log.DebugLevel
	default:
		return log.FatalLevel
	}
}

func toBool(s string) bool {
	switch strings.ToLower(s) {
	case "true", "1", "t", "yes":
		return true
	default:
		return false
	}
}

func duration(s string, defaults time.Duration) time.Duration {
	dur, err := time.ParseDuration(s)
	if err != nil {
		dur = defaults
	}
	return dur
}

func checkStatsdPrefix(s string) string {
	if !strings.HasSuffix(s, ".") {
		return s + "."
	}
	return s
}
