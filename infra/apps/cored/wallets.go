package cored

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum/v4/pkg/config"
)

const (
	desiredTotalSupply int64 = 500_000_000_000_000 // 500m core
	stakerBalance      int64 = 10_000_000_000_000  // 10m core
)

// mnemonics generating well-known keys to create predictable wallets so manual operation is easier.
//
//nolint:lll // we don't care about mnemonic strings
const (
	AliceMnemonic   = "mandate canyon major bargain bamboo soft fetch aisle extra confirm monster jazz atom ball summer solar tell glimpse square uniform situate body ginger protect"
	BobMnemonic     = "move equip digital assault wrong speed border multiply knife steel trash donor isolate remember lucky moon cupboard achieve canyon smooth pulp chief hold symptom"
	CharlieMnemonic = "announce already cherry rotate pull apology banana dignity region horse aspect august country exit connect unit agent curious violin tide town link unable whip"
)

//nolint:lll // we don't care about mnemonic strings
const (
	// FaucetMnemonic is mnemonic used by faucet to broadcast requested transfers.
	FaucetMnemonic = "pitch basic bundle cause toe sound warm love town crucial divorce shell olympic convince scene middle garment glimpse narrow during fix fruit suffer honey"
	// FundingMnemonic is mnemonic of used by integration testing framework to fund accounts required by integration tests.
	FundingMnemonic = "sad hobby filter tray ordinary gap half web cat hard call mystery describe member round trend friend beyond such clap frozen segment fan mistake"
	// RelayerMnemonicGaia is mnemonic used by the gaia relayer.
	RelayerMnemonicGaia = "notable rate tribe effort deny void security page regular spice safe prize engage version hour bless normal mother exercise velvet load cry front ordinary"
	// RelayerMnemonicOsmosis is mnemonic used by the osmosis relayer.
	RelayerMnemonicOsmosis = "possible barely tip truck blame regular success attend nasty range seat gun feature conduct blush wash certain nothing order have amused bring that canvas"
)

var namedMnemonicsList = []string{
	AliceMnemonic,
	BobMnemonic,
	CharlieMnemonic,
	FaucetMnemonic,
	FundingMnemonic,
	RelayerMnemonicGaia,
	RelayerMnemonicOsmosis,
}

// stakerMnemonics defines the list of the stakers used by validators.
//
//nolint:lll // we don't care about mnemonic strings
var stakerMnemonics = []string{
	"biology rigid design broccoli adult hood modify tissue swallow arctic option improve quiz cliff inject soup ozone suffer fantasy layer negative eagle leader priority",
	"enemy fix tribe swift alcohol metal salad edge episode dry tired address bless cloth error useful define rough fold swift confirm century wasp acoustic",
	"act electric demand cancel duck invest below once obvious estate interest solution drink mango reason already clean host limit stadium smoke census pattern express",
	"mercy throw finger code word craft rough then pitch pool recycle wrong lemon review syrup motion orient decrease grief rate obey hat seven grief",
	"boost accuse private unfair van room dish topic artist ice pond oven jeans acoustic prepare spot video rough soon eagle cement final science fuel",
	"report grow pear mansion east basic install true divorce combine firm possible swear anxiety time evoke erupt wrist step rotate face rhythm spoil soon",
	"meat love bamboo soon orbit adult clown impose first police december stereo walnut distance clap beauty spot lift process quarter indoor cool tone purpose",
	"vanish cabin club endorse lesson avocado notable universe chunk purity caution suit pioneer hip dawn eyebrow exercise receive pumpkin bamboo maid lobster hover peanut",
	"clown puzzle lamp keen mother home silly dutch sweet wedding object tool volume hazard meat remove law artwork elite fix clown siege portion iron",
	"marriage fox panel monitor reward cave slam habit rhythm weather decrease chimney lawsuit kite drum tiger term cupboard picnic bulk extend seek glad tornado",
	"minute high pupil method flee detail cotton lecture scissors tiger journey feel spell census label fee blame occur avocado deal giant inner tennis detect",
	"cup grain shaft ecology chalk since neutral normal scale false table guitar sock outside panda hospital afraid myself reduce salute palace page original cruel",
	"scare point movie weapon door limb salt six zone leaf stable wish swear harbor element afford senior cherry fashion device pole novel side pull",
	"valley garage afford provide turkey extra seminar strategy grunt strong wall evoke material sample artwork real anxiety flag balance mercy brisk heart bus alien",
	"life gossip angle civil wood goose equip can maze spy seat tent person master label okay message insect walnut symbol crash material popular move",
	"donkey twist olympic window oyster away margin leader shove cargo whip join sort balance cross desert genre feature impulse foot kit venture pond safe",
	"quantum believe need peace summer grape sign consider bunker exclude idea chalk monster lumber source lock fitness index armed can spirit jacket develop more",
	"cross double medal youth exchange way tree olympic thing west insect cheese destroy engage hammer corn height eyebrow else april indoor fiscal key journey",
	"practice bright squeeze gasp chronic mask double air neck powder salt mistake trim great onion total dynamic essence inflict torch outer advance outer actress",
	"rotate able mule regret explain plug only transfer employ physical festival crawl project device idea size husband together million pumpkin share spin federal list",
	"collect zoo dutch hand copper front chef dismiss mansion slow code basic wet desk key civil season impose purse saddle field expire question exhaust",
	"embrace cancel embrace axis race shoot car bamboo gate sense royal avoid sick snack manage genius cream route domain cycle follow lift wool require",
	"labor oval waste solid happy index solution traffic sand pair jeans excess volume thought outdoor animal exchange science route parade good carpet cement antique",
	"normal symbol parent vote regret weird toe print six stereo time situate urge vault cabbage humble scatter name patch walnut unique text legal lumber",
	"width trick width play borrow equip chat denial chase dry huge fold bean drama cry panel cat crush skill yard borrow jungle tennis unaware",
	"six library label minor next physical close satisfy eye nasty turkey indicate zone manual junk vintage spike protect lawn slender arch pottery appear must",
	"artefact load hip ankle author rhythm fame spoil orange great boat focus defense huge nature timber valley snack moment thought era punch element expect",
	"defy quit season recall result occur innocent area tobacco eager doll recall strike sentence degree essay section mouse wood journey mix quality vanish orbit",
	"manage trick float risk swim minimum scrub prepare deputy earth brother hub above mimic stay female neither mango hand field waste motion nurse flower",
	"electric task emerge nephew follow blue friend old exhibit desert deputy mirror coast turn cause shadow alcohol field clip climb endless gown pilot equal",
	"woman reform noodle film drift hard point dry bundle mansion key enact deal moment jewel fold debate gain muffin safe later march account gate",
	"ice deal defy struggle foster title mushroom bronze lonely unique shallow poet energy book mosquito hidden essay child room suggest balance spirit cash hunt",
}

// Wallet holds all predefined accounts.
type Wallet struct {
	stakerBalance         int64
	namedMnemonicsBalance int64
	stakerMnemonics       []string
	namedMnemonics        []string
}

// NewFundedWallet creates wallet and funds all predefined accounts.
func NewFundedWallet(network config.NetworkConfig) (*Wallet, config.NetworkConfig) {
	// distribute the remaining after stakers amount among Alice, Bob, Faucet, etc
	namedMnemonicsBalance :=
		(desiredTotalSupply - stakerBalance*int64(len(stakerMnemonics))) / int64(len(namedMnemonicsList))
	networkProvider := network.Provider.(config.DynamicConfigProvider)

	w := &Wallet{
		// We have integration tests adding new validators with min self delegation,
		// and then we kill them when test completes.
		// So if those tests run together and create validators having 33% of voting power,
		// then killing them will halt the chain.
		// That's why our main validators created here must have much higher stake.
		stakerBalance:         stakerBalance,
		namedMnemonicsBalance: namedMnemonicsBalance,
		stakerMnemonics:       stakerMnemonics,
		namedMnemonics:        namedMnemonicsList,
	}

	for _, mnemonic := range w.namedMnemonics {
		privKey, err := PrivateKeyFromMnemonic(mnemonic)
		must.OK(err)
		networkProvider = networkProvider.WithAccount(
			sdk.AccAddress(privKey.PubKey().Address()),
			sdk.NewCoins(sdk.NewInt64Coin(network.Denom(), w.namedMnemonicsBalance)),
		)
	}

	for _, mnemonic := range w.stakerMnemonics {
		privKey, err := PrivateKeyFromMnemonic(mnemonic)
		must.OK(err)
		networkProvider = networkProvider.WithAccount(
			sdk.AccAddress(privKey.PubKey().Address()),
			sdk.NewCoins(sdk.NewInt64Coin(network.Denom(), w.stakerBalance)),
		)
	}

	network.Provider = networkProvider
	return w, network
}

// GetStakersMnemonicsCount returns length of stakerMnemonics.
func (w Wallet) GetStakersMnemonicsCount() int {
	return len(w.stakerMnemonics)
}

// GetStakerMnemonicsBalance returns balance for the single staker.
func (w Wallet) GetStakerMnemonicsBalance() int64 {
	return w.stakerBalance
}

// GetStakersMnemonic returns staker mnemonic by index.
func (w Wallet) GetStakersMnemonic(index int) string {
	if len(w.stakerMnemonics) < index {
		panic(errors.New("index at GetStakersMnemonic is bigger than available mnemonic number"))
	}

	return w.stakerMnemonics[index]
}
