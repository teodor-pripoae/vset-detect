package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"github.com/starclusterteam/go-starbox/log"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/tendermint/tendermint/rpc/client/http"
	"github.com/tendermint/tendermint/types"
)

type RPCClient struct {
	addr   string
	name   string
	client *http.HTTP
	db     *leveldb.DB
}

func NewRPCClient(addr string, name string, db *leveldb.DB) (*RPCClient, error) {
	cl, err := http.New(addr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create tendermint client")
	}

	return &RPCClient{addr: addr, name: name, client: cl, db: db}, nil
}

func (c *RPCClient) Name() string {
	return c.name
}

func (c *RPCClient) GetLatestBlockHeight() (uint64, error) {
	response, err := c.client.Status(context.Background())

	if err != nil {
		return 0, errors.Wrap(err, "failed to get latest block height")
	}

	return uint64(response.SyncInfo.LatestBlockHeight), nil
}

func (c *RPCClient) GetBlockByHeight(height uint64) (*types.Block, []byte, error) {
	h := int64(height)
	response, err := c.client.Block(context.Background(), &h)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get block")
	}

	block := response.Block

	evidence := response.Block.Evidence

	if len(evidence.Evidence) == 0 {
		return block, nil, nil
	}

	evidenceBytes, err := json.Marshal(evidence)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to marshal evidence")
	}

	return block, evidenceBytes, nil
}

func (c *RPCClient) GetValidatorsAtHeight(height int64) ([]*types.Validator, error) {
	key := fmt.Sprintf("%s:validatorz:%d", c.name, height)
	data, err := c.db.Get([]byte(key), nil)
	if err != nil {
		page := 1
		perPage := 300
		response, err := c.client.Validators(context.Background(), &height, &page, &perPage)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get validators for block %d", height)
		}

		for _, v := range response.Validators {
			v.PubKey = nil
		}

		data, _ = json.Marshal(response.Validators)

		err = c.db.Put([]byte(key), data, nil)
		if err != nil {
			log.Errorf("failed to save validators for block %d to db: %s", height, err)
		}
	}

	var validators []*types.Validator
	err = json.Unmarshal(data, &validators)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal validators for block %d", height)
	}

	return validators, nil
}

func BlockFromJSONResponse(data []byte) (*types.Block, []byte, error) {
	data, evidence, err := fixInvalidBlock(data)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to fix invalid block")
	}
	// log.Infof("Response: %s", data)

	var result struct {
		Result struct {
			Block *types.Block `json:"block"`
		} `json:"result"`
	}

	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to unmarshal response body")
	}

	return result.Result.Block, evidence, nil
}

func BlockFromJSON(j []byte) (*types.Block, error) {
	block := &types.Block{}
	err := json.Unmarshal(j, block)

	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal block")
	}
	return block, nil
}

func BlockToJSON(block *types.Block) ([]byte, error) {
	return json.Marshal(block)
}

// Go struct field Consensus.result.block.header.version.block of type uint64
func fixInvalidBlock(dataBytes []byte) ([]byte, []byte, error) {
	data := map[string]interface{}{}
	err := json.Unmarshal(dataBytes, &data)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to unmarshal response body")
	}

	result, ok := data["result"].(map[string]interface{})
	if !ok {
		return nil, nil, errors.New("failed to parse result")
	}

	block, ok := result["block"].(map[string]interface{})
	if !ok {
		return nil, nil, errors.New("failed to parse block")
	}

	header, ok := block["header"].(map[string]interface{})
	if !ok {
		return nil, nil, errors.New("failed to parse header")
	}

	version, ok := header["version"].(map[string]interface{})
	if !ok {
		return nil, nil, errors.New("failed to parse version")
	}

	lastCommit, ok := block["last_commit"].(map[string]interface{})
	if !ok {
		return nil, nil, errors.New("failed to parse last_commit")
	}

	if err := fixStringToInt(version, "block"); err != nil {
		return nil, nil, errors.Wrap(err, "failed to fix version.block")
	}

	evidenceO, ok := block["evidence"].(map[string]interface{})
	if !ok {
		return nil, nil, errors.New("failed to parse evidence")
	}
	evidenceList, ok := evidenceO["evidence"].([]interface{})
	if !ok {
		return nil, nil, errors.New("failed to parse evidence.evidence")
	}
	evidence := []byte{}
	if len(evidenceList) > 0 {
		evidence, err = json.Marshal(evidenceList)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to marshal evidence")
		}

		evidenceO["evidence"] = []interface{}{}
	}

	if err := fixStringToInt(header, "height"); err != nil {
		return nil, nil, errors.Wrap(err, "failed to fix header.height")
	}

	if err := fixStringToInt(lastCommit, "height"); err != nil {
		return nil, nil, errors.Wrap(err, "failed to fix last_commit.height")
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to marshal data")
	}

	if len(evidence) > 0 {
		return jsonData, evidence, nil
	}

	return jsonData, nil, nil
}

func fixStringToInt(m map[string]interface{}, field string) error {
	fieldStr, ok := m[field].(string)
	if !ok {
		return errors.Errorf("failed to parse field %s", field)
	}

	fieldU, err := strconv.ParseUint(fieldStr, 10, 64)
	if err != nil {
		return errors.Wrapf(err, "failed to parse field %s, value: %s", field, fieldStr)
	}

	m[field] = fieldU

	return nil
}
