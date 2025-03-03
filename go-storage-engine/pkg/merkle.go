package merkle

import (
	"bytes"
	"crypto/sha256"
	"errors"
)

// ProofElement represents one node in the Merkle proof.
type ProofElement struct {
	Hash   []byte
	IsLeft bool // If true, the sibling hash is on the left of the current node.
}

// MerkleTree holds the complete tree structure.
type MerkleTree struct {
	Leaves [][]byte   // Original leaf hashes.
	Levels [][][]byte // Levels[0] = leaves, Levels[1] = intermediate nodes, etc.
}

// NewMerkleTree builds a Merkle tree from the provided data slices.
// Each data slice is assumed to be already hashed or is hashed internally.
func NewMerkleTree(leaves [][]byte) *MerkleTree {
	mt := &MerkleTree{Leaves: leaves}
	mt.buildTree()
	return mt
}

// buildTree constructs the tree levels from the leaves up.
func (mt *MerkleTree) buildTree() {
	level := mt.Leaves
	mt.Levels = append(mt.Levels, level)

	// Continue until we have a single hash (the root).
	for len(level) > 1 {
		var nextLevel [][]byte
		for i := 0; i < len(level); i += 2 {
			left := level[i]
			var right []byte
			if i+1 < len(level) {
				right = level[i+1]
			} else {
				// Duplicate last element if odd number of nodes.
				right = left
			}
			combined := append(left, right...)
			h := sha256.Sum256(combined)
			nextLevel = append(nextLevel, h[:])
		}
		mt.Levels = append(mt.Levels, nextLevel)
		level = nextLevel
	}
}

// Root returns the Merkle root.
func (mt *MerkleTree) Root() []byte {
	if len(mt.Levels) == 0 {
		return nil
	}
	lastLevel := mt.Levels[len(mt.Levels)-1]
	if len(lastLevel) == 0 {
		return nil
	}
	return lastLevel[0]
}

// GetProof generates the proof of inclusion for a leaf at a given index.
func (mt *MerkleTree) GetProof(index int) ([]ProofElement, error) {
	if index < 0 || index >= len(mt.Leaves) {
		return nil, errors.New("index out of range")
	}
	var proof []ProofElement
	// Walk up the tree level by level.
	for level := 0; level < len(mt.Levels)-1; level++ {
		levelSize := len(mt.Levels[level])
		var siblingIndex int
		var isLeft bool
		if index%2 == 0 {
			// Even index: sibling is to the right.
			siblingIndex = index + 1
			isLeft = false
			if siblingIndex >= levelSize {
				// In case of an odd number of nodes, skip if no sibling.
				continue
			}
		} else {
			// Odd index: sibling is to the left.
			siblingIndex = index - 1
			isLeft = true
		}
		proof = append(proof, ProofElement{
			Hash:   mt.Levels[level][siblingIndex],
			IsLeft: isLeft,
		})
		// Move to the parent index.
		index /= 2
	}
	return proof, nil
}

// VerifyProof verifies that a given leaf and its proof produce the expected root.
func VerifyProof(leaf []byte, proof []ProofElement, root []byte) bool {
	computedHash := leaf
	for _, pe := range proof {
		var combined []byte
		if pe.IsLeft {
			combined = append(pe.Hash, computedHash...)
		} else {
			combined = append(computedHash, pe.Hash...)
		}
		h := sha256.Sum256(combined)
		computedHash = h[:]
	}
	return bytes.Equal(computedHash, root)
}
