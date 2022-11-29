package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/starclusterteam/go-starbox/log"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
	"github.com/tendermint/tendermint/types"
	"golang.org/x/sync/semaphore"
)

func indexProvider(cmd *cobra.Command, args []string) error {
	db, err := newDB()
	if err != nil {
		return errors.Wrap(err, "failed to open database")
	}
	defer db.Close()

	provider, err := NewRPCClient(providerAddr, "provider", db)
	if err != nil {
		return errors.Wrap(err, "failed to create provider client")
	}

	latestBlock, err := provider.GetLatestBlockHeight()
	if err != nil {
		return errors.Wrap(err, "failed to get latest block height")
	}

	if latestBlock < providerMinHeight {
		return errors.New("latest block is less than minimum height")
	}

	log.Infof("Min height: %d", providerMinHeight)
	log.Infof("Latest block: %d", latestBlock)

	var wg sync.WaitGroup
	lock := semaphore.NewWeighted(32)

	for i := providerMinHeight; i <= latestBlock; i++ {
		wg.Add(1)
		go func(height uint64) {
			lock.Acquire(context.Background(), 1)
			defer lock.Release(1)
			defer wg.Done()

			if err := indexBlock(db, provider, height, false); err != nil {
				log.Errorf("failed to index block %d: %s", height, err)
			}
		}(i)
	}

	wg.Wait()

	fmt.Println("Done !!!")

	return nil
}

func indexConsumer(cmd *cobra.Command, args []string) error {
	db, err := newDB()
	if err != nil {
		return errors.Wrap(err, "failed to open database")
	}
	defer db.Close()

	if len(args) != 1 {
		return errors.New("missing consumer address")
	}

	consumerName := args[0]

	consumerEnvKey := fmt.Sprintf("%s_ADDR", strings.ToUpper(consumerName))
	consumerAddr := os.Getenv(consumerEnvKey)
	if consumerAddr == "" {
		return errors.Errorf("missing %s environment variable", consumerEnvKey)
	}

	consumerMinHeightKey := fmt.Sprintf("%s_MIN_HEIGHT", strings.ToUpper(consumerName))
	consumerMinHeightStr := os.Getenv(consumerMinHeightKey)
	if consumerMinHeightStr == "" {
		return errors.Errorf("missing %s environment variable", consumerMinHeightKey)
	}
	consumerMinHeight, err := strconv.ParseUint(consumerMinHeightStr, 10, 64)
	if err != nil {
		return errors.Wrap(err, "failed to parse minimum height")
	}

	consumer, err := NewRPCClient(consumerAddr, consumerName, db)
	if err != nil {
		return errors.Wrap(err, "failed to create consumer client")
	}

	latestBlock, err := consumer.GetLatestBlockHeight()
	if err != nil {
		return errors.Wrap(err, "failed to get latest block height")
	}

	if latestBlock < consumerMinHeight {
		return errors.New("latest block is less than minimum height")
	}

	log.Infof("Min height: %d", consumerMinHeight)
	log.Infof("Latest block: %d", latestBlock)

	var wg sync.WaitGroup
	lock := semaphore.NewWeighted(32)

	for i := consumerMinHeight; i <= latestBlock; i++ {
		wg.Add(1)
		go func(height uint64) {
			lock.Acquire(context.Background(), 1)
			defer lock.Release(1)
			defer wg.Done()

			if err := indexBlock(db, consumer, height, false); err != nil {
				log.Errorf("failed to index block %d: %s", height, err)
			}
		}(i)
	}

	wg.Wait()

	fmt.Println("Done !!!")

	return nil
}

func getBlock(cmd *cobra.Command, args []string) error {
	db, err := newDB()
	if err != nil {
		return errors.Wrap(err, "failed to open database")
	}
	defer db.Close()

	if len(args) != 2 {
		return errors.New("missing consumer address")
	}

	provider := args[0]
	height, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return errors.Wrap(err, "failed to parse block height")
	}

	key := BlockKey(provider, height)

	data, err := db.Get(key, nil)

	if err != nil {
		return errors.Wrap(err, "failed to get block")
	}

	block, err := BlockFromJSON(data)
	if err != nil {
		return errors.Wrap(err, "failed to decode block")
	}

	fmt.Println(block)

	key = EvidenceKey(provider, height)
	ok, err := db.Has(key, nil)
	if err != nil {
		return errors.Wrap(err, "failed to check evidence")
	}
	if ok {
		data, err := db.Get(key, nil)
		if err != nil {
			return errors.Wrap(err, "failed to get evidence")
		}

		fmt.Println(string(data))
	}

	return nil
}

func evidence(cmd *cobra.Command, args []string) error {
	db, err := newDB()
	if err != nil {
		return errors.Wrap(err, "failed to open database")
	}
	defer db.Close()

	if len(args) != 1 {
		return errors.New("missing provider address")
	}

	provider := args[0]

	iter := db.NewIterator(util.BytesPrefix([]byte(provider+":evidence:")), nil)
	defer iter.Release()

	// consumerName := args[0]

	// consumerEnvKey := fmt.Sprintf("%s_ADDR", strings.ToUpper(consumerName))
	// consumerAddr := os.Getenv(consumerEnvKey)
	// if consumerAddr == "" {
	// 	return errors.Errorf("missing %s environment variable", consumerEnvKey)
	// }

	// consumer := NewRPCClient(consumerAddr, consumerName)

	for iter.Next() {
		key := iter.Key()
		// value := iter.Value()

		heightStr := strings.Split(string(key), ":")[2]
		height, err := strconv.ParseUint(heightStr, 10, 64)
		if err != nil {
			return errors.Wrap(err, "failed to parse block height")
		}

		// if err := indexBlock(db, consumer, height, true); err != nil {
		// log.Errorf("failed to index block %d: %s", height, err)
		// }

		fmt.Println(height)
		fmt.Println(string(iter.Value()))
	}

	return nil
}

func validatorSet(cmd *cobra.Command, args []string) error {
	db, err := newDB()
	if err != nil {
		return errors.Wrap(err, "failed to open database")
	}
	defer db.Close()

	if len(args) != 2 {
		return errors.New("missing consumer address")
	}

	provider := args[0]
	height, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return errors.Wrap(err, "failed to parse block height")
	}

	key := BlockKey(provider, height)

	data, err := db.Get(key, nil)

	if err != nil {
		return errors.Wrap(err, "failed to get block")
	}

	block, err := BlockFromJSON(data)
	if err != nil {
		return errors.Wrap(err, "failed to decode block")
	}

	if block.LastCommit == nil {
		return errors.New("block has no last commit")
	}

	validators := []string{}

	for _, sig := range block.LastCommit.Signatures {
		address := sig.ValidatorAddress.String()
		if address != "" {
			validators = append(validators, address)
		}
	}

	sort.Strings(validators)
	// str := strings.Join(validators, ",")
	// hash := fmt.Sprintf("%x", md5.Sum([]byte(str)))
	// log.Infof("Md5 hash: %s", hash)

	log.Infof("Validators hash hash: %s", block.ValidatorsHash.String())
	log.Infof("Validators: %s", validators)

	return nil
}

func viewMissingValidator(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.New("missing consumer chain name")
	}

	lb, _ := cmd.Flags().GetUint64("toBlock")
	var latestBlock *uint64
	if lb == 0 {
		latestBlock = nil
	} else {
		latestBlock = &lb
	}

	consumerName := args[0]

	var validatorSetProvider []ValsetUpdate
	var validatorSetConsumer []ValsetUpdate
	var err error

	db, err := newDB()
	if err != nil {
		return errors.Wrap(err, "failed to open database")
	}
	defer db.Close()

	var wg sync.WaitGroup

	wg.Add(2)

	go func(db *leveldb.DB) {
		defer wg.Done()
		validatorSetProvider, err = validatorSetAll(db, "provider", latestBlock)
		if err != nil {
			log.Errorf("failed to get validator set from provider: %s", err)
		}
	}(db)

	go func(db *leveldb.DB) {
		defer wg.Done()
		validatorSetConsumer, err = validatorSetAll(db, consumerName, latestBlock)
		if err != nil {
			log.Errorf("failed to get validator set from consumer: %s", err)
		}
	}(db)

	wg.Wait()

	log.Infof("Found %d validator hashes in consumer %s", len(validatorSetConsumer), consumerName)
	log.Infof("Found %d validator hashes in provider", len(validatorSetProvider))

	firstProviderHash := validatorSetProvider[0]

	valset := map[string]ValsetUpdate{}

	for _, vs := range validatorSetProvider {
		valset[vs.Md5Hash] = vs
	}

	missingCount := 0
	notExistedOnProvider := 0
	outOfOrderCount := 0

	for _, vs := range validatorSetConsumer {
		if vs.Timestamp.Before(firstProviderHash.Timestamp) {
			// skip validator set updates before provider started
			continue
		}

		_, ok := valset[vs.Md5Hash]
		if !ok {
			missingCount++
			log.Infof("[missing] Found consumer validator hash %s at block %d, missing from provider", vs.ValidatorsHash, vs.Height)
			continue
		}

		// check if validator set update exists on provider before timestamp
		if !existsBeforeTimestamp(validatorSetProvider, vs.Md5Hash, vs.Timestamp) {
			notExistedOnProvider++
			log.Infof("[not existed] Found consumer validator hash %s at block %d, not existed on provider at that time", vs.ValidatorsHash, vs.Height)
			continue
		}

		if !isInOrder(validatorSetProvider, vs) {
			outOfOrderCount++
			log.Infof("[out of order] Found consumer validator hash %s, old validator hash %s at block %d", vs.ValidatorsHash, vs.OldValidatorsHash, vs.Height)
		}

		// // lastpvs, ok := valset[vs.OldValidatorsHash]
		// // if !ok {
		// // 	log.Infof("[old valhash missing on provider] Found consumer validator hash %s at block %d, old validator hash %s missing from provider", vs.ValidatorsHash, vs.Height, vs.OldValidatorsHash)
		// // 	continue
		// // }

		// // if lastpvs.Height > pvs.Height {
		// 	continue
		// }
	}

	log.Infof("Found %d missing validator hashes", missingCount)
	log.Infof("Found %d not existed on provider chain at that time", notExistedOnProvider)
	log.Infof("Found %d out of order validator hashes", outOfOrderCount)

	return nil
}

func isInOrder(validatorSet []ValsetUpdate, vs ValsetUpdate) bool {
	if vs.OldMd5Hash == "" {
		return true
	}

	i := 0
	foundBlockCurrent := 0
	foundBlockPrevious := 0

	for i < len(validatorSet) {
		if validatorSet[i].Timestamp.After(vs.Timestamp) {
			break
		}

		if foundBlockCurrent == 0 && validatorSet[i].Md5Hash == vs.Md5Hash {
			foundBlockCurrent = i
		}

		if foundBlockPrevious == 0 && validatorSet[i].Md5Hash == vs.OldMd5Hash {
			foundBlockPrevious = i
		}

		i++
	}

	return foundBlockPrevious < foundBlockCurrent
}

func existsBeforeTimestamp(validatorSet []ValsetUpdate, hash string, timestamp time.Time) bool {
	for _, vs := range validatorSet {
		if vs.Md5Hash == hash && vs.Timestamp.Before(timestamp) {
			return true
		}
	}

	return false
}

func validatorSetAll(db *leveldb.DB, chain string, latestBlock *uint64) ([]ValsetUpdate, error) {

	consumerEnvKey := fmt.Sprintf("%s_ADDR", strings.ToUpper(chain))
	consumerAddr := os.Getenv(consumerEnvKey)
	if consumerAddr == "" {
		return nil, errors.Errorf("missing %s environment variable", consumerEnvKey)
	}

	consumerMinHeightKey := fmt.Sprintf("%s_MIN_HEIGHT", strings.ToUpper(chain))
	consumerMinHeightStr := os.Getenv(consumerMinHeightKey)
	if consumerMinHeightStr == "" {
		return nil, errors.Errorf("missing %s environment variable", consumerMinHeightKey)
	}
	consumerMinHeight, err := strconv.ParseUint(consumerMinHeightStr, 10, 64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse minimum height")
	}

	consumer, err := NewRPCClient(consumerAddr, chain, db)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create RPC client")
	}

	var lb uint64

	if latestBlock == nil {
		lb, err = consumer.GetLatestBlockHeight()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get latest block height")
		}
	} else {
		lb = *latestBlock
	}

	if lb < consumerMinHeight {
		return nil, errors.New("latest block is less than minimum height")
	}

	validatorSet := []ValsetUpdate{}
	lastValidatorHash := ""
	lastMd5Hash := ""

	fileName := fmt.Sprintf("validatorset-%s.csv", chain)
	file, err := os.Create(fileName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create file")
	}
	defer file.Close()

	for i := consumerMinHeight; i <= lb; i++ {
		if i%10000 == 0 {
			log.Infof("Processing block %d on chain %s", i, chain)
		}

		key := BlockKey(chain, i)
		exists, err := db.Has(key, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to check if block exists")
		}
		if !exists {
			break
		}

		data, err := db.Get(key, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get block")
		}

		block, err := BlockFromJSON(data)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse block: %s", key)
		}

		validatorsHash := block.ValidatorsHash.String()

		if validatorsHash != lastValidatorHash {
			log.Debugf("Found new validator set: %s at height %d", validatorsHash, block.Height)

			valsetlist, err := consumer.GetValidatorsAtHeight(block.Height)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get validator set at height %d", block.Height)
			}
			md5hash := getValsetHash(valsetlist)

			bh := ValsetUpdate{
				Height:            block.Height,
				Timestamp:         block.Time,
				ValidatorsHash:    validatorsHash,
				OldValidatorsHash: lastValidatorHash,
				Md5Hash:           md5hash,
				OldMd5Hash:        lastMd5Hash,
			}

			fmt.Fprintf(file, "%d,%s,%s,%s,%s,%s\n", bh.Height, bh.Timestamp, bh.ValidatorsHash, bh.OldValidatorsHash, bh.Md5Hash, bh.OldMd5Hash)

			validatorSet = append(validatorSet, bh)

			lastValidatorHash = validatorsHash
			lastMd5Hash = md5hash
		}
	}

	return validatorSet, nil
}

func getValsetHash(valsetlist []*types.Validator) string {
	valset := []string{}
	for _, val := range valsetlist {
		str := fmt.Sprintf("%s:%d", val.Address, val.VotingPower)
		valset = append(valset, str)
	}
	sort.Strings(valset)
	valsetstr := strings.Join(valset, ";")
	return fmt.Sprintf("%x", md5.Sum([]byte(valsetstr)))
}
