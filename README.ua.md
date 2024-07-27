# Injective's Peggo [![Peggy.sol MythX](https://badgen.net/https/api.mythx.io/v1/projects/82ca9468-f86d-4550-a0ae-bc120eeb055f/badge/data?cache=300&icon=https://raw.githubusercontent.com/ConsenSys/mythx-github-badge/main/logo_white.svg)](https://docs.mythx.io/dashboard/github-badges)

Peggo - це впровадження Peggy Orchestrator на мові програмування Go для Injective.

Важливі команди:

* `peggo orchestrator` запускає основний цикл оркестратора.
* `peggo tx register-eth-key` це спеціальна команда для надсилання ключа Ethereum, який буде використовуватися для підпису повідомлень від вашого Валідатора


## Installation

Спочатку завантажте собі `Go 1.15+` на https://golang.org/dl/ а потім:

```
$ go get github.com/InjectiveLabs/peggo/orchestrator/cmd/...
```

## peggo

Peggo - це допоміжний виконуваний файл для оркестрації валідатора Peggy.

### Конфігурація

Використовуйте аргументи CLI, прапорці або створіть `.env` зі змінними середовища.

### Використання

```
$ peggo --help

Використання: peggo [Опції] КОМАНДА [arg...]

Peggo - це допоміжний виконуваний файл для оркестрації валідатора Peggy.

Опції:
  -e, --env                Назва середовища, в якому працює цей додаток. Використовується для метрик та повідомлень про помилки. (env $PEGGO_ENV) (default "local")
  -l, --log-level          Доступні рівні: error, warn, info, debug. (env $PEGGO_LOG_LEVEL) (default "info")
      --svc-wait-timeout   Стандартний тайм-аут очікування для зовнішніх служб (наприклад, з'єднання Cosmos daemon GRPC connection) (env $PEGGO_SERVICE_WAIT_TIMEOUT) (default "1m")

Команди:
  orchestrator             Запускає головний цикл оркестратора.
  q, query                 Команди запитів, які можуть отримати інформацію про стан від Peggy.
  tx                       Транзакції для управління Peggy та обслуговування.
  version                  Друкує інформацію про версію та виходить.

Запустіть 'peggo COMMAND --help' для отримання додаткової інформації про команду.  
```

## Команди

### peggo orchestrator

```
$ peggo orchestrator -h

Використання: peggo orchestrator [Опції]

Запускає головний цикл оркестратора.

Опції:
      --cosmos-chain-id                  Вкажіть ID мережі Cosmos. (env $PEGGO_COSMOS_CHAIN_ID) (default "888")
      --cosmos-grpc                      Кінцева точка запиту Cosmos GRPC (env $PEGGO_COSMOS_GRPC) (default "tcp://localhost:9900")
      --tendermint-rpc                   Кінцева точка Tendermint RPC (env $PEGGO_TENDERMINT_RPC) (default "http://localhost:26657")
      --cosmos-gas-prices                Вкажіть комісію за транзакції в мережі Cosmos у вигляді цін на газ DecCoins (env $PEGGO_COSMOS_GAS_PRICES)
      --cosmos-keyring                   Вкажіть бекенд Cosmos keyring (os|file|kwallet|pass|test) (env $PEGGO_COSMOS_KEYRING) (default "file")
      --cosmos-keyring-dir               Вкажіть директорію Cosmos keyring, якщо використовуєте file keyring. (env $PEGGO_COSMOS_KEYRING_DIR)
      --cosmos-keyring-app               Вкажіть ім'я додатку Cosmos keyring. (env $PEGGO_COSMOS_KEYRING_APP) (default "peggo")
      --cosmos-from                      Вкажіть ім'я або адресу ключа валідатора Cosmos. Якщо вказано, має існувати в keyring, ledger або співпадати з privkey. (env $PEGGO_COSMOS_FROM)
      --cosmos-from-passphrase           Вкажіть пароль keyring, інакше буде використано Stdin. (env $PEGGO_COSMOS_FROM_PASSPHRASE) (default "peggo")
      --cosmos-pk                        Вкажіть приватний ключ рахунку валідатора Cosmos у форматі hex. ВИКОРИСТОВУЙТЕ ТІЛЬКИ ДЛЯ ТЕСТУВАННЯ! (env $PEGGO_COSMOS_PK)
      --cosmos-use-ledger                Використовуйте додаток Cosmos на апаратному ledger для підпису транзакцій. (env $PEGGO_COSMOS_USE_LEDGER)
      --eth-chain-id                     Вкажіть ID мережі Ethereum. (env $PEGGO_ETH_CHAIN_ID) (default 42)
      --eth-node-http                    Вкажіть HTTP кінцеву точку для вузла Ethereum. (env $PEGGO_ETH_RPC) (default "http://localhost:1317")
      --eth-node-alchemy-ws              Вкажіть url веб-сокета для вузла Ethereum Alchemy. (env $PEGGO_ETH_ALCHEMY_WS)
      --eth_gas_price_adjustment         Корекція ціни газу для транзакцій на Ethereum (env $PEGGO_ETH_GAS_PRICE_ADJUSTMENT) (default 1.3)
      --eth-keystore-dir                 Вкажіть директорію Ethereum keystore (формат Geth) префікс. (env $PEGGO_ETH_KEYSTORE_DIR)
      --eth-from                         Вкажіть адресу відправника. Якщо вказано, має існувати в keystore, ledger або відповідати privkey. (env $PEGGO_ETH_FROM)
      --eth-passphrase                   Пароль для розблокування приватного ключа з armor, якщо порожній, то використовується stdin. (env $PEGGO_ETH_PASSPHRASE)
      --eth-pk                           Надайте необроблений приватний ключ валідатора Ethereum у форматі hex. ВИКОРИСТОВУЙТЕ ТІЛЬКИ ДЛЯ ТЕСТУВАННЯ! (env $PEGGO_ETH_PK)
      --eth-use-ledger                   Використовуйте додаток Ethereum на апаратному ledger для підпису транзакцій. (env $PEGGO_ETH_USE_LEDGER)
      --relay_valsets                    Якщо включено, relayer буде перенаправляти valsets до ethereum (env $PEGGO_RELAY_VALSETS)
      --relay_valset_offset_dur          Якщо встановлено, relayer буде транслювати valsetUpdate лише після того, як пройде relayValsetOffsetDur від часу створення valsetUpdate (env $PEGGO_RELAY_VALSET_OFFSET_DUR) (default "5m")
      --relay_batches                    Якщо включено, relayer пересилатиме пакети до Ethereum. (env $PEGGO_RELAY_BATCHES)
      --relay_batch_offset_dur           Якщо встановлено, релейер буде транслювати пакети лише після того, як пройде relayBatchOffsetDur від моменту створення пакету (env $PEGGO_RELAY_BATCH_OFFSET_DUR) (default "5m")
      --relay_pending_tx_wait_duration   Якщо встановлено, релейер буде транслювати очікуючі пакети/оновлення valsetupdate лише після того, як пройде час pendingTxWaitDuration (env $PEGGO_RELAY_PENDING_TX_WAIT_DURATION) (default "20m")
      --min_batch_fee_usd                Якщо встановлено, запит на пакет створить пакети тільки у випадку, якщо поріг комісії перевищено (env $PEGGO_MIN_BATCH_FEE_USD) (default 23.3)
      --coingecko_api                    Вкажіть кінцеву точку HTTP для coingecko api. (env $PEGGO_COINGECKO_API) (default "https://api.coingecko.com/api/v3")

```

### peggo tx register-eth-key

```
 peggo tx register-eth-key --help

Використання: peggo tx register-eth-key [ОПЦІЇ]

Надсилає ключ Ethereum, який буде використовуватися для підпису повідомлень від імені вашого валідатора

ОПЦІЇ:
      --cosmos-chain-id          Вказує ID ланцюга мережі Cosmos. (env $PEGGO_COSMOS_CHAIN_ID) (default "888")
      --cosmos-grpc              Кінцева точка запиту Cosmos GRPC (env $PEGGO_COSMOS_GRPC) (default "tcp://localhost:9900")
      --tendermint-rpc           Кінцева точка Tendermint RPC (env $PEGGO_TENDERMINT_RPC) (default "http://localhost:26657")
      --cosmos-gas-prices        Вказує вартість транзакцій ланцюга Cosmos у вигляді газових цін DecCoins (env $PEGGO_COSMOS_GAS_PRICES)
      --cosmos-keyring           Вкажіть бекенд Cosmos keyring (os|file|kwallet|pass|test) (env $PEGGO_COSMOS_KEYRING) (default "file")
      --cosmos-keyring-dir       Вкажіть директорію Cosmos keyring, якщо використовуєте file keyring. (env $PEGGO_COSMOS_KEYRING_DIR)
      --cosmos-keyring-app       Вкажіть ім'я додатку Cosmos keyring. (env $PEGGO_COSMOS_KEYRING_APP) (default "peggo")
      --cosmos-from              Вкажіть ім'я або адресу ключа валідатора Cosmos. Якщо вказано, має існувати в keyring, ledger або співпадати з privkey. (env $PEGGO_COSMOS_FROM)
      --cosmos-from-passphrase   Вкажіть пароль keyring, інакше буде використано Stdin. (env $PEGGO_COSMOS_FROM_PASSPHRASE) (default "peggo")
      --cosmos-pk                Вкажіть приватний ключ рахунку валідатора Cosmos у форматі hex. ВИКОРИСТОВУЙТЕ ТІЛЬКИ ДЛЯ ТЕСТУВАННЯ! (env $PEGGO_COSMOS_PK)
      --cosmos-use-ledger        Використовуйте додаток Cosmos на апаратному ledger для підпису транзакцій. (env $PEGGO_COSMOS_USE_LEDGER)
      --eth-keystore-dir         Вказує директорію Ethereum keystore (формат Geth) префікс. (env $PEGGO_ETH_KEYSTORE_DIR)
      --eth-from                 Вкажіть адресу відправника. Якщо вказано, має існувати в keystore, ledger або відповідати privkey. (env $PEGGO_ETH_FROM)
      --eth-passphrase           Пароль для розблокування приватного ключа з armor, якщо порожній, то використовується stdin. (env $PEGGO_ETH_PASSPHRASE)
      --eth-pk                   Надайте необроблений приватний ключ валідатора Ethereum у форматі hex. ВИКОРИСТОВУЙТЕ ТІЛЬКИ ДЛЯ ТЕСТУВАННЯ! (env $PEGGO_ETH_PK)
      --eth-use-ledger           Використовуйте додаток Ethereum на апаратному ledger для підпису транзакцій. (env $PEGGO_ETH_USE_LEDGER)
  -y, --yes                      Завжди автоматично підтверджує дії, такі як відправка транзакцій. (env $PEGGO_ALWAYS_AUTO_CONFIRM)
```

## Ліцензія

Apache 2.0
