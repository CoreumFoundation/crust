package cored

import (
	"crypto/ed25519"
	"encoding/hex"

	tmed25519 "github.com/tendermint/tendermint/crypto/ed25519"
)

// NodeID computes node ID from node public key.
func NodeID(pubKey ed25519.PublicKey) string {
	return hex.EncodeToString(tmed25519.PubKey(pubKey).Address())
}
