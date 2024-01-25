package cosmos

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"github.com/InjectiveLabs/sdk-go/chain/crypto/ethsecp256k1"
	"github.com/InjectiveLabs/sdk-go/chain/crypto/hd"
	"github.com/cosmos/cosmos-sdk/codec/types"
	cosmcrypto "github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	cosmtypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/pkg/errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/InjectiveLabs/sdk-go/chain/codec"
	cosmoscdc "github.com/cosmos/cosmos-sdk/codec"
)

const (
	DefaultKeyName = "validator"
)

type KeyringConfig struct {
	KeyringDir,
	KeyringAppName,
	KeyringBackend,
	KeyFrom,
	KeyPassphrase,
	PrivateKey string
	UseLedger bool
}

func (cfg KeyringConfig) withPrivateKey() bool {
	return len(cfg.PrivateKey) > 9
}

type Keyring struct {
	keyring.Keyring

	Addr cosmtypes.AccAddress
}

func NewKeyring(cfg KeyringConfig) (Keyring, error) {
	if cfg.withPrivateKey() {
		return newInMemoryKeyring(cfg)
	}

	return newCosmosKeyring(cfg)

}

func newInMemoryKeyring(cfg KeyringConfig) (Keyring, error) {
	if cfg.UseLedger {
		return Keyring{}, errors.New("cannot use both private key and Ledger")
	}

	pk := cfg.PrivateKey
	if strings.HasPrefix(pk, "0x") {
		pk = pk[2:]
	}

	pkRaw, err := hex.DecodeString(pk)
	if err != nil {
		return Keyring{}, errors.Wrap(err, "invalid private key")
	}

	var (
		cosmosPK   = &ethsecp256k1.PrivKey{Key: pkRaw}
		cosmosAddr = cosmtypes.AccAddress(cosmosPK.PubKey().Address())
		keyName    = DefaultKeyName
	)

	from, err := cosmtypes.AccAddressFromBech32(cfg.KeyFrom)
	if err != nil {
		keyName = cfg.KeyFrom // use it as key name
	}

	if err == nil && !bytes.Equal(from.Bytes(), cosmosAddr.Bytes()) {
		return Keyring{}, errors.Errorf("expected account address %s but got %s from the private key", from.String(), cosmosAddr.String())
	}

	k, err := KeyringForPrivKey(keyName, cosmosPK)
	if err != nil {
		return Keyring{}, errors.Wrap(err, "failed to initialize cosmos keyring")
	}

	kr := Keyring{
		Keyring: k,
		Addr:    cosmosAddr,
	}

	return kr, nil
}

func newCosmosKeyring(cfg KeyringConfig) (Keyring, error) {
	if len(cfg.KeyFrom) == 0 {
		return Keyring{}, errors.New("insufficient cosmos details provided")
	}

	keyringDir := cfg.KeyringDir
	if !filepath.IsAbs(keyringDir) {
		dir, err := filepath.Abs(keyringDir)
		if err != nil {
			return Keyring{}, errors.Wrap(err, "failed to get absolute path of keyring dir")
		}

		keyringDir = dir
	}

	var reader io.Reader = os.Stdin
	if len(cfg.KeyPassphrase) > 0 {
		reader = newPassReader(cfg.KeyPassphrase)
	}

	kr, err := keyring.New(
		cfg.KeyringAppName,
		cfg.KeyringBackend,
		keyringDir,
		reader,
		Codec(),
		hd.EthSecp256k1Option(),
	)

	if err != nil {
		return Keyring{}, errors.Wrap(err, "failed to initialize cosmos keyring")
	}

	var keyRecord *keyring.Record
	if cosmosAddr, err := cosmtypes.AccAddressFromBech32(cfg.KeyFrom); err != nil {
		r, err := kr.KeyByAddress(cosmosAddr)
		if err != nil {
			return Keyring{}, err
		}

		keyRecord = r
	} else {
		r, err := kr.Key(cfg.KeyFrom)
		if err != nil {
			return Keyring{}, err
		}

		keyRecord = r
	}

	switch keyRecord.GetType() {
	case keyring.TypeLocal:
		// kb has a key and it's totally usable
		addr, err := keyRecord.GetAddress()
		if err != nil {
			return Keyring{}, errors.Wrap(err, "failed to get address from key record")
		}

		k := Keyring{
			Keyring: kr,
			Addr:    addr,
		}

		return k, nil
	case keyring.TypeLedger:
		// the kb stores references to ledger keys, so we must explicitly
		// check that. kb doesn't know how to scan HD keys - they must be added manually before
		if !cfg.UseLedger {
			return Keyring{}, errors.Errorf("key %s is a Ledger reference, enable Ledger option", keyRecord.Name)
		}

		addr, err := keyRecord.GetAddress()
		if err != nil {
			return Keyring{}, errors.Wrap(err, "failed to get address from key record")

		}

		k := Keyring{
			Keyring: kr,
			Addr:    addr,
		}

		return k, nil
	default:
		return Keyring{}, errors.Errorf("unsupported key type: %s", keyRecord.GetType())
	}
}

// KeyringForPrivKey creates a temporary in-mem keyring for a PrivKey.
// Allows to init Context when the key has been provided in plaintext and parsed.
func KeyringForPrivKey(name string, privKey cryptotypes.PrivKey) (keyring.Keyring, error) {
	kb := keyring.NewInMemory(Codec(), hd.EthSecp256k1Option())
	tmpPhrase := randPhrase(64)
	armored := cosmcrypto.EncryptArmorPrivKey(privKey, tmpPhrase, privKey.Type())
	err := kb.ImportPrivKey(name, armored, tmpPhrase)
	if err != nil {
		err = errors.Wrap(err, "failed to import privkey")
		return nil, err
	}

	return kb, nil
}

func Codec() cosmoscdc.Codec {
	interfaceRegistry := types.NewInterfaceRegistry()
	codec.RegisterInterfaces(interfaceRegistry)
	codec.RegisterLegacyAminoCodec(cosmoscdc.NewLegacyAmino())

	return cosmoscdc.NewProtoCodec(interfaceRegistry)
}

func randPhrase(size int) string {
	buf := make([]byte, size)
	_, err := rand.Read(buf)
	if err != nil {
		panic("rand failed")
	}

	return string(buf)
}

func newPassReader(pass string) io.Reader {
	return &passReader{
		pass: pass,
		buf:  new(bytes.Buffer),
	}
}

type passReader struct {
	pass string
	buf  *bytes.Buffer
}

var _ io.Reader = &passReader{}

func (r *passReader) Read(p []byte) (n int, err error) {
	n, err = r.buf.Read(p)
	if err == io.EOF || n == 0 {
		r.buf.WriteString(r.pass + "\n")

		n, err = r.buf.Read(p)
	}

	return
}
