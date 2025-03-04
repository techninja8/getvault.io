package proofofinclusion

import (
	"crypto/sha256"
	"fmt"

	"github.com/cbergoon/merkletree"
)

// Content implements the merkletree.Content interface.
type Content struct {
	data []byte
}

func (c Content) CalculateHash() ([]byte, error) {
	h := sha256.Sum256(c.data)
	return h[:], nil
}

func (c Content) Equals(other merkletree.Content) (bool, error) {
	return string(c.data) == string(other.(Content).data), nil
}

// BuildMerkleTree constructs a Merkle tree from the provided data slices.
func BuildMerkleTree(dataSlices [][]byte) (*merkletree.MerkleTree, error) {
	var list []merkletree.Content
	for _, d := range dataSlices {
		list = append(list, Content{data: d})
	}
	tree, err := merkletree.NewTree(list)
	if err != nil {
		return nil, err
	}
	return tree, nil
}

// GetProof returns a textual representation of the Merkle proof for a given content.
func GetProof(tree *merkletree.MerkleTree, content []byte) (string, error) {
	proof, indices, err := tree.GetMerklePath(Content{data: content})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("proof: %v, indices: %v", proof, indices), nil
}
