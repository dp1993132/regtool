package YTCrypto

import (
	"crypto/rand"
	"errors"

	crypto "github.com/libp2p/go-libp2p-core/crypto"
	base58 "github.com/mr-tron/base58/base58"
)

func CreateRsaKey() (string, string) {
	privateKey, publicKey, _ := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	privateKeyBytes, _ := privateKey.Raw()
	publicKeyBytes, _ := publicKey.Raw()
	return base58.Encode(privateKeyBytes), base58.Encode(publicKeyBytes)
}

func GetRsaPublicKeyByPrivateKey(privateKeyStr string) (string, error) {
	privateKeyBytes, err := base58.Decode(privateKeyStr)
	if err != nil {
		return "", err
	}
	privateKey, err := crypto.UnmarshalRsaPrivateKey(privateKeyBytes)
	if err != nil {
		return "", err
	}
	publicKey := privateKey.GetPublic()
	raw, err := publicKey.Raw()
	if err != nil {
		return "", err
	}
	return base58.Encode(raw), nil
}

func RsaSign(privateKeyStr string, data []byte) (string, error) {
	privateKeyBytes, err := base58.Decode(privateKeyStr)
	if err != nil {
		return "", err
	}
	privateKey, err := crypto.UnmarshalRsaPrivateKey(privateKeyBytes)
	if err != nil {
		return "", err
	}
	sig, err := privateKey.Sign(data)
	if err != nil {
		return "", err
	}
	return base58.Encode(sig), nil
}

func RsaVerify(publicKeyStr string, data []byte, sig string) (bool, error) {
	publicKeyBytes, err := base58.Decode(publicKeyStr)
	if err != nil {
		return false, err
	}
	publicKey, err := crypto.UnmarshalRsaPublicKey(publicKeyBytes)
	if err != nil {
		return false, err
	}
	sigBytes, err := base58.Decode(sig)
	if err != nil {
		return false, err
	}
	return publicKey.Verify(data, sigBytes)
}

func RsaEncrypt(publicKeyStr string, data []byte) ([]byte, error) {
	return nil, nil
}

const (
	RSAPUB  = "rsapub"
	RSAPRIV = "rsapriv"
)

func ConvertKeyToString(key crypto.Key) (string, error) {
	keyBytes, err := key.Raw()
	if err != nil {
		return "", err
	}
	keyStr := base58.Encode(keyBytes)
	return keyStr, nil
}

func ConvertStringToKey(key string, keyType string) (crypto.Key, error) {
	keyBytes, err := base58.Decode(key)
	if err != nil {
		return nil, err
	}
	if keyType == RSAPUB {
		return crypto.UnmarshalRsaPublicKey(keyBytes)
	}
	if keyType == RSAPRIV {
		return crypto.UnmarshalRsaPrivateKey(keyBytes)
	}
	return nil, errors.New("key type is not exist.")
}
