package main

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/starclusterteam/go-starbox/log"
	"github.com/syndtr/goleveldb/leveldb"
)

func BlockKey(providerName string, height uint64) []byte {
	key := fmt.Sprintf("%s:block:%d", providerName, height)
	return []byte(key)
}

func EvidenceKey(providerName string, height uint64) []byte {
	key := fmt.Sprintf("%s:evidence:%d", providerName, height)
	return []byte(key)
}

func indexBlock(db *leveldb.DB, rpc *RPCClient, height uint64, force bool) error {
	key := BlockKey(rpc.Name(), height)
	hasKey, err := db.Has(key, nil)
	if err != nil {
		log.Errorf("Failed to check if key exists: %s", err)
		return errors.Wrap(err, "failed to check if key exists")
	}

	if hasKey && !force {
		log.Debugf("Skipping block %d, key already exists: %s", height, key)
		return nil
	}

	block, evidence, err := rpc.GetBlockByHeight(height)
	if err != nil {
		log.Errorf("Failed to get block %d: %s", height, err)
		return errors.Wrap(err, "failed to get block")
	}

	data, err := BlockToJSON(block)
	if err != nil {
		log.Errorf("Failed to encode block %d: %s", height, err)
		return errors.Wrap(err, "failed to encode block")
	}

	err = db.Put([]byte(key), data, nil)
	if err != nil {
		log.Errorf("Failed to save block %d: %s", height, err)
		return errors.Wrap(err, "failed to save block")
	}

	if evidence != nil {
		key := fmt.Sprintf("%s:evidence:%d", rpc.Name(), height)

		err = db.Put([]byte(key), evidence, nil)
		if err != nil {
			log.Errorf("Failed to save evidence %d: %s", height, err)
			return errors.Wrap(err, "failed to save evidence")
		}
	}

	log.Infof("Indexed block %d", height)
	return nil
}

func validatorSetForBlock(db *leveldb.DB, providerName string, height uint64) ([]byte, error) {
	key := BlockKey(providerName, height)
	data, err := db.Get(key, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get block")
	}

	block, err := BlockFromJSON(data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode block")
	}

	return block.ValidatorsHash, nil
}
