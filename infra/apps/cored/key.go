package cored

import (
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum/pkg/types"
	"github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cosmossecp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
)

// addKeysToStore adds keys to local keystore
func addKeysToStore(homeDir string, keys map[string]types.Secp256k1PrivateKey) {
	keyringDB, err := keyring.New("cored", "test", homeDir, nil)
	must.OK(err)
	signatureAlgos, _ := keyringDB.SupportedAlgorithms()
	signatureAlgo, err := keyring.NewSigningAlgoFromString("secp256k1", signatureAlgos)
	must.OK(err)

	signatureAlgo.Generate()

	for name, key := range keys {
		privKey := &cosmossecp256k1.PrivKey{Key: key}
		must.OK(keyringDB.ImportPrivKey(name, crypto.EncryptArmorPrivKey(privKey, "dummy", privKey.Type()), "dummy"))
	}
}
