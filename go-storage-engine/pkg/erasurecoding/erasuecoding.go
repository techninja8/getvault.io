package erasurecoding

import (
	"github.com/klauspost/reedsolomon"
)

const (
	dataShards   = 4
	parityShards = 2
)

// Encode splits and encodes the data into shards.
func Encode(data []byte) ([][]byte, error) {
	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return nil, err
	}
	shards, err := enc.Split(data)
	if err != nil {
		return nil, err
	}
	if err = enc.Encode(shards); err != nil {
		return nil, err
	}
	return shards, nil
}

// Decode reconstructs the original data from shards.
func Decode(shards [][]byte) ([]byte, error) {
	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return nil, err
	}
	if err = enc.Reconstruct(shards); err != nil {
		return nil, err
	}
	// Join shards back into a single byte slice.
	data, err := enc.Join(nil, shards, -1)
	if err != nil {
		return nil, err
	}
	return data, nil
}
