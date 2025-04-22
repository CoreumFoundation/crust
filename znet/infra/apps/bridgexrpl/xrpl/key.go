package xrpl

import (
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/crypto"
	rippledata "github.com/rubblelabs/ripple/data"
	"github.com/samber/lo"

	coreumconfig "github.com/CoreumFoundation/coreum/v6/pkg/config"
)

// HDPath is the hd path of XRPL key.
const HDPath = "m/44'/144'/0'/0/0"

// KeyFromSeed generates private key from seed.
func KeyFromSeed(seedPhrase string) (crypto.Key, error) {
	seed, err := rippledata.NewSeedFromAddress(seedPhrase)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create rippledata seed from seed phrase")
	}
	return seed.Key(rippledata.ECDSA), nil
}

// KeyFromMnemonic generates private key from mnemonic.
func KeyFromMnemonic(mnemonic string) (crypto.Key, error) {
	const keyName = "key"

	encodingConfig := coreumconfig.NewEncodingConfig()
	kr := keyring.NewInMemory(encodingConfig.Codec)

	_, err := kr.NewAccount(
		keyName,
		mnemonic,
		"",
		HDPath,
		hd.Secp256k1,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	key, err := kr.Key(keyName)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	rl := key.GetLocal()
	if rl.PrivKey == nil {
		return nil, errors.Errorf("private key is not available, key name:%s", keyName)
	}
	privKey, ok := rl.PrivKey.GetCachedValue().(cryptotypes.PrivKey)
	if !ok {
		return nil, errors.New("unable to cast any to cryptotypes.PrivKey")
	}

	return newXRPLPrivKey(privKey), nil
}

// AccountFromKey creates account from private key.
func AccountFromKey(key crypto.Key) rippledata.Account {
	var account rippledata.Account
	copy(account[:], key.Id(lo.ToPtr[uint32](0)))
	return account
}

// AccountFromMnemonic creates account from mnemonic.
func AccountFromMnemonic(mnemonic string) (rippledata.Account, error) {
	privKey, err := KeyFromMnemonic(mnemonic)
	if err != nil {
		return rippledata.Account{}, err
	}

	var account rippledata.Account
	copy(account[:], privKey.Id(lo.ToPtr[uint32](0)))

	return account, nil
}

// xrplPrivKey is `ripplecrypto.Key` implementation.
type xrplPrivKey struct {
	privKey cryptotypes.PrivKey
}

func newXRPLPrivKey(privKey cryptotypes.PrivKey) xrplPrivKey {
	return xrplPrivKey{
		privKey: privKey,
	}
}

//nolint:revive,stylecheck //interface method
func (k xrplPrivKey) Id(sequence *uint32) []byte {
	return crypto.Sha256RipeMD160(k.Public(sequence))
}

func (k xrplPrivKey) Private(_ *uint32) []byte {
	return k.privKey.Bytes()
}

func (k xrplPrivKey) Public(_ *uint32) []byte {
	return k.privKey.PubKey().Bytes()
}
