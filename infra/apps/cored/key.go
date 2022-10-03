package cored

import (
	"encoding/hex"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum/pkg/types"
)

// addMnemonicsToKeyring adds keys to local keystore
func addMnemonicsToKeyring(homeDir string, mnemonics map[string]string) {
	keyringDB, err := keyring.New("cored", "test", homeDir, nil)
	must.OK(err)

	for name, mnemonic := range mnemonics {
		_, err := keyringDB.NewAccount(name, mnemonic, "", sdk.GetConfig().GetFullBIP44Path(), hd.Secp256k1)
		must.OK(err)
	}
}

// PrivateKeyFromMnemonic generates private key from mnemonic
func PrivateKeyFromMnemonic(mnemonic string) (types.Secp256k1PrivateKey, error) {
	kr := keyring.NewUnsafe(keyring.NewInMemory())

	_, err := kr.NewAccount("tmp", mnemonic, "", sdk.GetConfig().GetFullBIP44Path(), hd.Secp256k1)
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
