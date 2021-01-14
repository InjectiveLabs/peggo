package orchestrator

import (
	"context"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/peggo/orchestrator/relayer"
	"github.com/InjectiveLabs/peggo/orchestrator/sidechain"
)

const defaultLoopDur = 10 * time.Second

// RunLoop combines the four major roles required to make
// up the 'Orchestrator', all four of these are async loops.
func (s *peggyOrchestrator) RunLoop(ctx context.Context) {
	wg := new(sync.WaitGroup)
	defer wg.Wait()

	go s.ethOracleMainLoop(wg)
	go s.ethSignerMainLoop(wg)
	go s.relayerMainLoop(wg)
	go s.valsetRequesterLoop(wg)
}

// ethOracleMainLoop is responsible for making sure that Ethereum events are retrieved from the Ethereum blockchain
// and ferried over to Cosmos where they will be used to issue tokens or process batches.
//
// TODO this loop requires a method to bootstrap back to the correct event nonce when restarted
func (s *peggyOrchestrator) ethOracleMainLoop(wg *sync.WaitGroup) {
	defer wg.Done()
	ctx := context.Background()

	var err error
	var lastCheckedBlock uint64
	for {
		lastCheckedBlock, err = s.getLastCheckedBlock(ctx)
		if err != nil {
			log.WithError(err).Errorln("failed to get last checked block, retry in", defaultRetryDur)
			time.Sleep(defaultRetryDur)
			continue
		}

		log.Infoln("Oracle resync complete, Oracle now operational")
		break
	}

	t := time.NewTimer(0)
	for range t.C {
		latestHeader, err := s.ethProvider.HeaderByNumber(ctx, nil)
		if err != nil {
			log.WithError(err).Errorln("failed to get latest header, retry in", defaultRetryDur)
			t.Reset(defaultRetryDur)
			continue
		}

		latestEthBlock := latestHeader.Number.Uint64()
		latestCosmosBlock, err := s.tmClient.GetLatestBlockHeight()
		if err != nil {
			log.WithError(err).Errorln("failed to get latest cosmos block height, retry in", defaultRetryDur)
			t.Reset(defaultRetryDur)
			continue
		}

		log.Debugf("[ethOracleMainLoop] Latest Eth block %d, latest Cosmos block %d",
			latestEthBlock, latestCosmosBlock,
		)

		// Relays events from Ethereum -> Cosmos
		currentBlock, err := s.checkForEvents(ctx, lastCheckedBlock)
		if err != nil {
			log.WithError(err).Errorln("error during eht event checking, retry in", defaultRetryDur)
			t.Reset(defaultRetryDur)
			continue
		} else {
			lastCheckedBlock = currentBlock
		}

		t.Reset(defaultLoopDur)
	}

}

// The eth_signer simply signs off on any batches or validator sets provided by the validator
// since these are provided directly by a trusted Cosmsos node they can simply be assumed to be
// valid and signed off on.
func (s *peggyOrchestrator) ethSignerMainLoop(wg *sync.WaitGroup) {
	defer wg.Done()

	ctx := context.Background()

	var err error
	var peggyID common.Hash
	for {
		peggyID, err = s.peggyContract.GetPeggyID(ctx, s.peggyContract.FromAddress())
		if err != nil {

			log.WithError(err).Errorln("failed to get PeggyID from Ethereum contract, retry in", defaultRetryDur)
			time.Sleep(defaultRetryDur)
			continue
		}
		log.Debugf("[ethSignerMainLoop] peggyID %s", peggyID.Hex())
		break
	}

	t := time.NewTimer(0)
	for range t.C {
		latestHeader, err := s.ethProvider.HeaderByNumber(ctx, nil)
		if err != nil {
			log.WithError(err).Errorln("failed to get latest header, retry in", defaultRetryDur)
			t.Reset(defaultRetryDur)
			continue
		}

		latestEthBlock := latestHeader.Number.Uint64()
		latestCosmosBlock, err := s.tmClient.GetLatestBlockHeight()
		if err != nil {
			log.WithError(err).Errorln("failed to get latest cosmos block height, retry in", defaultRetryDur)
			t.Reset(defaultRetryDur)
			continue
		}

		log.Debugf("[ethSignerMainLoop] Latest Eth block %d, latest Cosmos block %d",
			latestEthBlock, latestCosmosBlock,
		)

		valset, err := s.cosmosQueryClient.OldestUnsignedValset(ctx, s.peggyBroadcastClient.FromAddress())
		if err == sidechain.ErrNotFound {
			log.Debugln("no Valset waiting to be signed")
		} else if err != nil {
			log.WithError(err).Errorln("failed to get unsigned Valset for signing, retry in", defaultRetryDur)
			t.Reset(defaultRetryDur)
			continue
		} else {
			log.Infoln("sending Valset confirm for %d", valset.Nonce)

			if err := s.peggyBroadcastClient.SendValsetConfirm(ctx, s.ethPrivateKey, peggyID, valset); err != nil {
				log.WithError(err).Errorln("failed to sign and send Valset confirmation to Cosmos, retry in", defaultRetryDur)
				t.Reset(defaultRetryDur)
				continue
			}
		}

		// sign the last unsigned batch, TODO check if we already have signed this
		txBatch, err := s.cosmosQueryClient.OldestUnsignedTransactionBatch(ctx, s.peggyBroadcastClient.FromAddress())
		if err == sidechain.ErrNotFound {
			log.Debugln("no TransactionBatch waiting to be signed")
		} else if err != nil {
			log.WithError(err).Errorln("failed to get unsigned TransactionBatch for signing, retry in", defaultRetryDur)
			t.Reset(defaultRetryDur)
			continue
		} else {
			log.Infoln("sending TransactionBatch confirm for %d", txBatch.BatchNonce)

			if err := s.peggyBroadcastClient.SendBatchConfirm(ctx, s.ethPrivateKey, peggyID, txBatch); err != nil {
				log.WithError(err).Errorln("failed to sign and send TransactionBatch confirmation to Cosmos, retry in", defaultRetryDur)
				t.Reset(defaultRetryDur)
				continue
			}
		}

		t.Reset(defaultLoopDur)
	}
}

// This loop doesn't have a formal role per say, anyone can request a valset
// but there does need to be some strategy to ensure requests are made. Having it
// be a function of the orchestrator makes a lot of sense as they are already online
// and have all the required funds, keys, and rpc servers setup
//
// Exactly how to balance optimizing this versus testing is an interesting discussion
// in testing we want to make sure requests are made without any powers changing on the chain
// just to simplify the test environment. But in production that's somewhat wasteful. What this
// routine does it check the current valset versus the last requested valset, if power has changed
// significantly we send in a request.
func (s *peggyOrchestrator) valsetRequesterLoop(wg *sync.WaitGroup) {
	defer wg.Done()

	ctx := context.Background()

	t := time.NewTimer(0)
	for range t.C {
		if err := s.peggyBroadcastClient.SendValsetRequest(ctx); err != nil {
			log.WithError(err).Warningln("valset request failed")
		}

		// TODO: this needed for gas saving optimizations:
		//
		//
		// latestValsets, err := s.cosmosQueryClient.LatestValsets(ctx)
		// if err != nil {
		// 	log.WithError(err).Errorln("unable to get latest valsets from Cosmos chain, retry in", defaultRetryDur)
		// 	t.Reset(defaultRetryDur)
		// 	continue
		// }
		//
		// currentValset, err := s.cosmosQueryClient.CurrentValset(ctx)
		// if err != nil {
		// 	log.WithError(err).Errorln("unable to get current valset from Cosmos chain, retry in", defaultRetryDur)
		// 	t.Reset(defaultRetryDur)
		// 	continue
		// }
		//
		// if len(latestValsets) == 0 {
		// 	if err := s.peggyBroadcastClient.SendValsetRequest(ctx); err != nil {
		// 		log.WithError(err).Warningln("valset request failed")
		// 	}
		// } else {
		// 	// TODO(xlab): gas saving?
		// 	//
		// 	// let power_diff = current_valset.power_diff(&latest_valsets[0]);
		// 	// if the power difference is more than 1% different than the last valset
		// 	// if power_diff > 0.01f32 {
		// 	//     let _ = send_valset_request(&contact, cosmos_key, fee.clone()).await;
		// 	// }
		// }

		t.Reset(defaultLoopDur)
	}
}

func (s *peggyOrchestrator) relayerMainLoop(wg *sync.WaitGroup) {
	defer wg.Done()

	r := relayer.NewPeggyRelayer(s.cosmosQueryClient, s.peggyContract)
	r.RunLoop()
}
