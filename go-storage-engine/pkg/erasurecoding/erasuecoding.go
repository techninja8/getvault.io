package erasurecoding

import (
	"bytes"

	"github.com/klauspost/reedsolomon"
)

var (
	DataShards   = 8
	ParityShards = 6
)

// Encode splits and encodes the data into shards.
func Encode(data []byte) ([][]byte, error) {
	enc, err := reedsolomon.New(DataShards, ParityShards)
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
	enc, err := reedsolomon.New(DataShards, ParityShards)
	if err != nil {
		return nil, err
	}
	if err = enc.Reconstruct(shards); err != nil {
		return nil, err
	}
	// Join shards back into a single byte slice.
	var buf bytes.Buffer
	if err = enc.Join(&buf, shards, len(shards[0])*DataShards); err != nil {
		return nil, err
	}
	//return bytes.Trim(buf.Bytes(), "\x00"), nil

	return buf.Bytes(), nil
}
