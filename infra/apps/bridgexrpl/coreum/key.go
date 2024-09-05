package coreum

import (
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pkg/errors"

	coreumconfig "github.com/CoreumFoundation/coreum/v4/pkg/config"
	"github.com/CoreumFoundation/coreum/v4/pkg/config/constant"
)

// AccountFromMnemonic creates account from mnemonic.
func AccountFromMnemonic(mnemonic string) (sdk.AccAddress, error) {
	const keyName = "key"

	encodingConfig := coreumconfig.NewEncodingConfig()
	kr := keyring.NewInMemory(encodingConfig.Codec)

	keyInfo, err := kr.NewAccount(
		keyName,
		mnemonic,
		"",
		hd.CreateHDPath(constant.CoinType, 0, 0).String(),
		hd.Secp256k1,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	addr, err := keyInfo.GetAddress()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return addr, nil
}
