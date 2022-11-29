package main

import "time"

type ValsetUpdate struct {
	ValidatorsHash    string    `json:"validators_hash"`
	OldValidatorsHash string    `json:"old_validators_hash"`
	Md5Hash           string    `json:"md5_hash"`
	OldMd5Hash        string    `json:"old_md5_hash"`
	Height            int64     `json:"height"`
	Timestamp         time.Time `json:"timestamp"`
}
