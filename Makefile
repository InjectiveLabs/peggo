BUILD_DIR   ?= $(CURDIR)/build
SDK_VERSION := $(shell go list -m github.com/cosmos/cosmos-sdk | sed 's:.* ::')
COMMIT      := $(shell git log -1 --format='%H')

###############################################################################
##                                  Version                                  ##
###############################################################################

ifeq (,$(VERSION))
  VERSION := $(shell git describe --exact-match 2>/dev/null)
  # if VERSION is empty, then populate it with branch's name and raw commit hash
  ifeq (,$(VERSION))
    VERSION := $(BRANCH)-$(COMMIT)
  endif
endif

###############################################################################
##                              Build / Install                              ##
###############################################################################

ldflags = -X github.com/umee-network/peggo/cmd/peggo.Version=$(VERSION) \
		  -X github.com/umee-network/peggo/cmd/peggo.Commit=$(COMMIT) \
		  -X github.com/umee-network/peggo/cmd/peggo.SDKVersion=$(SDK_VERSION)

BUILD_FLAGS := -ldflags '$(ldflags)'

build: go.sum
	@echo "--> Building..."
	CGO_ENABLED=0 go build -mod=readonly -o $(BUILD_DIR)/ $(BUILD_FLAGS) ./...

install: go.sum
	@echo "--> Installing..."
	CGO_ENABLED=0 go install -mod=readonly $(BUILD_FLAGS) ./...

.PHONY: build install

###############################################################################
##                              Tests & Linting                              ##
###############################################################################

PACKAGES_UNIT=$(shell go list ./... | grep -v '/e2e' | grep -v '/solidity' | grep -v '/test' )
PACKAGES_E2E=$(shell go list ./... | grep '/e2e')
TEST_PACKAGES=./...
TEST_TARGETS := test-unit test-unit-cover test-race test-e2e

test-unit: ARGS=-timeout=5m -tags='norace'
test-unit: TEST_PACKAGES=$(PACKAGES_UNIT)
test-unit-cover: ARGS=-timeout=5m -tags='norace' -coverprofile=coverage.txt -covermode=atomic
test-unit-cover: TEST_PACKAGES=$(PACKAGES_UNIT)
test-e2e: ARGS=-timeout=25m -v
test-e2e: TEST_PACKAGES=$(PACKAGES_E2E)
$(TEST_TARGETS): run-tests

run-tests:
ifneq (,$(shell which tparse 2>/dev/null))
	@echo "--> Running tests"
	@go test -mod=readonly -json $(ARGS) $(TEST_PACKAGES) | tparse
else
	@echo "--> Running tests"
	@go test -mod=readonly $(ARGS) $(TEST_PACKAGES)
endif

test-integration:
	@echo "--> Running tests"
	@go test -mod=readonly -race ./test/... -v

lint:
	@echo "--> Running linter"
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint run --timeout=10m

mocks:
	@echo "--> Generating mocks"
	@go run github.com/golang/mock/mockgen -destination=mocks/cosmos.go \
			-package=mocks github.com/umee-network/peggo/cmd/peggo/client \
			CosmosClient
	@go run github.com/golang/mock/mockgen -destination=mocks/evm_provider.go \
			-package=mocks github.com/umee-network/peggo/orchestrator/ethereum/provider \
			EVMProviderWithRet
	@go run github.com/golang/mock/mockgen -destination=mocks/gravity_queryclient.go \
			-package=mocks github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types \
			QueryClient
	@go run github.com/golang/mock/mockgen -destination=mocks/gravity/gravity_contract.go \
			-package=gravity github.com/umee-network/peggo/orchestrator/ethereum/gravity \
			Contract

.PHONY: test-integration lint mocks

###############################################################################
##                                 Solidity                                  ##
###############################################################################

gen: solidity-wrappers

SOLIDITY_DIR = ../Gravity-Bridge/solidity
solidity-wrappers: $(SOLIDITY_DIR)/contracts/*.sol
	cd $(SOLIDITY_DIR)/contracts/ ; \
	for file in $(^F) ; do \
			mkdir -p ../wrappers/$${file} ; \
			echo abigen --type=peggy --pkg wrappers --out=../wrappers/$${file}/wrapper.go --sol $${file} ; \
			abigen --type=peggy --pkg wrappers --out=../wrappers/$${file}/wrapper.go --sol $${file} ; \
	done


###############################################################################
##                                  Docker                                   ##
###############################################################################

docker-build:
	@docker build -t umeenet/peggo .

docker-build-debug:
	@docker build -t umeenet/peggo --build-arg IMG_TAG=debug .

.PHONY: docker-build docker-build-debug
