package main

import (
	"strconv"

	"github.com/spf13/cobra"
	"github.com/starclusterteam/go-starbox/config"
	"github.com/starclusterteam/go-starbox/log"
	"github.com/syndtr/goleveldb/leveldb"
)

var dbFile string

var providerAddr string
var providerMinHeight uint64

func init() {
	var err error

	providerAddr = config.String("PROVIDER_ADDR", "http://localhost:26657")
	providerMinHeightStr := config.String("PROVIDER_MIN_HEIGHT", "1")

	providerMinHeight, err = strconv.ParseUint(providerMinHeightStr, 10, 64)

	if err != nil {
		panic(err)
	}

	dbFile = config.String("DB_FILE", "database.db")
}

func newDB() (*leveldb.DB, error) {
	return leveldb.OpenFile(dbFile, nil)
}

func main() {
	mainCmd := &cobra.Command{
		Use:   "vset-detect",
		Short: "Detects validator set changes",
	}

	indexCmd := &cobra.Command{
		Use:   "index",
		Short: "Indexes blocks from the chain",
	}

	indexProviderCmd := &cobra.Command{
		Use:   "provider",
		Short: "Indexes blocks from the provider",
		RunE:  indexProvider,
	}

	indexConsumerCmd := &cobra.Command{
		Use:   "consumer <consumer name>",
		Short: "Indexes blocks from the consumer",
		RunE:  indexConsumer,
	}

	viewMissingValidatorCmd := &cobra.Command{
		Use:   "view-missing-validator",
		Short: "View missing validator",
		RunE:  viewMissingValidator,
	}
	viewMissingValidatorCmd.Flags().Uint64("toBlock", 0, "Latest block to query")

	indexCmd.AddCommand(indexProviderCmd)
	indexCmd.AddCommand(indexConsumerCmd)

	getBlockCmd := &cobra.Command{
		Use:   "get-block <chain> <block height>",
		Short: "Get block",
		RunE:  getBlock,
	}

	validatorSetCmd := &cobra.Command{
		Use:   "validator-set <chain> <block height>",
		Short: "Get validator set at specific block height",
		RunE:  validatorSet,
	}

	evidenceCmd := &cobra.Command{
		Use:   "evidence <chain>",
		Short: "Get evidence",
		RunE:  evidence,
	}

	mainCmd.AddCommand(getBlockCmd)
	mainCmd.AddCommand(indexCmd)
	mainCmd.AddCommand(validatorSetCmd)
	mainCmd.AddCommand(evidenceCmd)
	mainCmd.AddCommand(viewMissingValidatorCmd)

	if err := mainCmd.Execute(); err != nil {
		log.PanicExit(err)
	}
}
