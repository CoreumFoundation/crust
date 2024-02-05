package bridgexrpl

// mnemonics generating well-known keys to create predictable wallets so manual operation is easier.
//
//nolint:lll // we don't care about mnemonic strings
const (
	// CoreumAdminMnemonic is the mnemonic used to set up the bridge on Coreum network.
	CoreumAdminMnemonic = "analyst evil lucky job exhaust inform note where grant file already exit vibrant come finger spatial absorb enter aisle orange soldier false attend response"
	// XRPLAdminMnemonic is the mnemonic used to set up the bridge on XRPL network.
	XRPLAdminMnemonic = "clutch fashion rocket cheap section ensure alter legend jeans smoke peanut hair sword room soldier pride peace silly answer orange leave pact reform village"
)

// Mnemonics holds mnemonic used by relayer.
type Mnemonics struct {
	Coreum string
	XRPL   string
}

// RelayerMnemonics is the list of mnemonics used by the relayers.
//
//nolint:lll // we don't care about mnemonic strings
var RelayerMnemonics = []Mnemonics{
	{
		Coreum: "secret sun grow glide normal robust power crime rug food zone can card label blush pill exist culture hen indoor artefact spirit inner quality",
		XRPL:   "nuclear waste easy soldier mosquito inflict faculty margin body drive topic sting loop arm minute detail vague universe follow liquid expire sausage episode gesture",
	},
	{
		Coreum: "stem clown area marriage tomorrow radio observe nephew gallery emerge ring bless sail gloom aerobic minimum can genuine clerk busy midnight mask bus alley",
		XRPL:   "foil edit office adjust orphan mushroom horn awesome mobile treat net message hotel food walk leaf recycle pool radio exotic improve mention sentence alarm",
	},
	{
		Coreum: "coast gain pill ridge emotion claim bus fashion liar feature earth victory shop paddle powder ring action vibrant churn kiss athlete frequent cover double",
		XRPL:   "text inmate twenty stomach athlete head noodle fortune surge slot script eyebrow vapor effort differ judge vacuum erode solid quarter enforce position artwork genius",
	},
	{
		Coreum: "dice quick social basic morning defense birth silly embrace fatal tornado couple truck age obtain drama wheel mountain wreck umbrella spider present perfect large",
		XRPL:   "goat fish barrel afford voice coil injury run trade retire solution unique lawn oil cattle lazy audit joke long grace income neglect mail sell",
	},
}
