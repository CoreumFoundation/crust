package cored

import (
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cosmossecp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum/app"
	"github.com/CoreumFoundation/coreum/pkg/config"
)

// importMnemonicsToKeyring adds keys to local keystore.
func importMnemonicsToKeyring(homeDir string, mnemonics map[string]string) error {
	kr, err := keyring.New("cored", "test", homeDir, nil, config.NewEncodingConfig(app.ModuleBasics).Codec)
	if err != nil {
		return errors.WithStack(err)
	}

	for name, mnemonic := range mnemonics {
		if _, err := kr.NewAccount(name, mnemonic, "", sdk.GetConfig().GetFullBIP44Path(), hd.Secp256k1); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

// UnsafeKeyring is kering with n ability to get private key,
type UnsafeKeyring interface {
	keyring.Keyring
	ExportPrivateKeyObject(uid string) (types.PrivKey, error)
}

// PrivateKeyFromMnemonic generates private key from mnemonic.
func PrivateKeyFromMnemonic(mnemonic string) (cosmossecp256k1.PrivKey, error) {
	kr, ok := keyring.NewInMemory(config.NewEncodingConfig(app.ModuleBasics).Codec).(UnsafeKeyring)
	if !ok {
		panic("can't cast to UnsafeKeyring")
	}

	_, err := kr.NewAccount("tmp", mnemonic, "", sdk.GetConfig().GetFullBIP44Path(), hd.Secp256k1)
	if err != nil {
		return cosmossecp256k1.PrivKey{}, err
	}

	privKey, err := kr.ExportPrivateKeyObject("tmp")
	if err != nil {
		panic(err)
	}

	return cosmossecp256k1.PrivKey{
		Key: privKey.Bytes(),
	}, nil
}
