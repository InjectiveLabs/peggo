<!-- markdownlint-disable MD024 -->
<!--
Changelog Guiding Principles:

Changelogs are for humans, not machines.
There should be an entry for every single version.
The same types of changes should be grouped.
Versions and sections should be linkable.
The latest version comes first.
The release date of each version is displayed.
Mention whether you follow Semantic Versioning.

Usage:

Change log entries are to be added to the Unreleased section under the
appropriate stanza (see below). Each entry should ideally include a tag and
the Github PR referenced in the following format:

* (<tag>) [#<PR-number>](https://github.com/umee-network/peggo/pull/<PR-number>) <changelog entry>

Types of changes (Stanzas):

Features: for new features.
Improvements: for changes in existing functionality.
Deprecated: for soon-to-be removed features.
Bug Fixes: for any bug fixes.
API Breaking: for breaking exported Go APIs used by developers.

To release a new version, ensure an appropriate release branch exists. Add a
release version and date to the existing Unreleased section which takes the form
of:

## [<version>](https://github.com/umee-network/peggo/releases/tag/<version>) - YYYY-MM-DD

Once the version is tagged and released, a PR should be made against the main
branch to incorporate the new changelog updates.

Ref: https://keepachangelog.com/en/1.0.0/
-->

# Changelog

## [Unreleased]

## [v0.2.5](https://github.com/umee-network/peggo/releases/tag/v0.2.5) - 2022-02-21

### Features

- [#189] Add the flag `--valset-relay-mode` which allows a finer control over
  how valsets will be relayed.

### Improvements

- [#201] Add ERC20 mappings for Umee's new tokens.

### Deprecated

- [#189] Deprecate the `--relay-valsets` flag.

### Bug Fixes

- [#189] Order validator set before deploying (`peggo bridge deploy-gravity`)

## [v0.2.4](https://github.com/umee-network/peggo/releases/tag/v0.2.4) - 2022-02-16

### Improvements

- [#172] Add fallback token addresses (to aid price lookup)
- [#185] Add fallback token addresses (to aid price lookup) for Umee

### Deprecated

- [#174] Deprecate `--eth-pk` in favor of an env var (`$PEGGO_ETH_PK`)

## [v0.2.3](https://github.com/umee-network/peggo/releases/tag/v0.2.3) - 2022-02-07

### Improvements

- [#158] Bump Gravity Bridge module to v1.4.1
- [#161] Bump Cosmos SDK to v0.45.1
- [#163] Bump Umee to v0.7.4

## [v0.2.2](https://github.com/umee-network/peggo/releases/tag/v0.2.2) - 2022-02-01

### Improvements

- [#144] Bump Umee version to 0.7.1 and fix Gravity Bridge to v1.3.5

### Bug Fixes

- [#139] Fix issue reported by ToB's audit (TOB-001)

## [v0.2.1](https://github.com/umee-network/peggo/releases/tag/v0.2.1) - 2022-01-31

### Improvements

- [#132] Add `cosmos-msgs-per-tx` flag to set how many messages (Ethereum claims)
  will be sent in each Cosmos transaction.
- [#134] Improve valset relaying by changing how we search for the last valid
  valset update.

### Bug Fixes

- [#134] Fix logs, CLI help and a panic when a non-function call transaction was
 received during the TX pending check.

## [v0.2.0](https://github.com/umee-network/peggo/releases/tag/v0.2.0) - 2022-01-17

### Features

- [#118] Target the [Gravity Bridge](https://github.com/Gravity-Bridge/Gravity-Bridge) module.

### Improvements

- [#123] Cleanup after GB implementation. Updates and fixes to match Gravity.sol
- [#125] Enable running tests with Ganache. Use gentx for gravity keys.

### Bug fixes

- [#128] Fix "nonce too low" error and other issues related to relaying.

## [v0.1.1](https://github.com/umee-network/peggo/releases/tag/v0.1.1) - 2021-12-22

### Bug Fixes

- [#104] Claims are split into chunks of 10 to avoid hitting request limits.

### Improvements

- [#104] Changed timeout for broadcasting TXs to Umee to 60s to match that of the
  official Gravity Bridge.
- [#105] Added a gas limit adjustment flag for Ethereum transactions.

## [v0.1.0](https://github.com/umee-network/peggo/releases/tag/v0.1.0) - 2021-12-18

### Features

- Initial release!!!
