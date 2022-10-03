package cored

import (
	"encoding/hex"

	"github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cosmossecp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum/pkg/types"
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

// PrivateKeyFromMnemonic generates private key from mnemonic
func PrivateKeyFromMnemonic(mnemonic string) (types.Secp256k1PrivateKey, error) {
	kr := keyring.NewUnsafe(keyring.NewInMemory())

	hdPath := hd.CreateHDPath(sdk.GetConfig().GetCoinType(), 0, 0).String()
	_, err := kr.NewAccount("tmp", mnemonic, "", hdPath, hd.Secp256k1)
	if err != nil {
		return nil, err
	}

	privKeyHex, err := kr.UnsafeExportPrivKeyHex("tmp")
	if err != nil {
		panic(err)
	}

	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		panic(err)
	}
	return privKeyBytes, nil
}
