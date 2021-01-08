package YTCrypto

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/ripemd160"

	ecrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/mr-tron/base58"
)

//CreateKey generate private key/public key pair
func CreateKey() (string, string) {
	privKey, _ := ecrypto.GenerateKey()
	privKeyBytes := ecrypto.FromECDSA(privKey)
	rawPrivKeyBytes := append([]byte{}, 0x80)
	rawPrivKeyBytes = append(rawPrivKeyBytes, privKeyBytes...)
	checksum := sha256Sum(rawPrivKeyBytes)
	checksum = sha256Sum(checksum)
	rawPrivKeyBytes = append(rawPrivKeyBytes, checksum[0:4]...)
	privateKey := base58.Encode(rawPrivKeyBytes)

	pubKey := privKey.PublicKey
	pubKeyBytes := ecrypto.CompressPubkey(&pubKey)
	checksum = ripemd160Sum(pubKeyBytes)
	rawPublicKeyBytes := append(pubKeyBytes, checksum[0:4]...)
	publicKey := base58.Encode(rawPublicKeyBytes)
	return privateKey, publicKey
}

func GetPublicKeyByPrivateKey(privateKey string) (string, error) {
	pk, err := getRawPrivateKey(privateKey)
	if err != nil {
		return "", err
	}
	privKey, err := ecrypto.ToECDSA(pk)
	pubKey := privKey.PublicKey
	pubKeyBytes := ecrypto.CompressPubkey(&pubKey)
	checksum := ripemd160Sum(pubKeyBytes)
	rawPublicKeyBytes := append(pubKeyBytes, checksum[0:4]...)
	publicKey := base58.Encode(rawPublicKeyBytes)
	return publicKey, nil
}

//recover public key from data and signature
func Ecrecover(data []byte, signature string) (string, error) {
	if !strings.HasPrefix(signature, "SIG_K1_") {
		return "", errors.New("prefix of signature is illegal.")
	}
	signature = strings.TrimPrefix(signature, "SIG_K1_")
	sigbytes, _ := base58.Decode(signature)
	sign := sigbytes[0:65]
	checksum := sigbytes[65:]
	sign1 := append([]byte{}, sign...)
	ck := ripemd160Sum(append(sign1, 'K', '1'))
	if !bytes.Equal(checksum, ck[0:4]) {
		return "", errors.New("checksum of signature is invalid.")
	}
	sign[0] -= 4
	sign[0] -= 27
	signx := append(sign[1:65], sign[0])
	recPubkey, err := ecrypto.Ecrecover(sha256Sum(data), signx)
	if err != nil {
		return "", errors.New("recover public key failed.")
	}
	recPublicKey, err := ecrypto.UnmarshalPubkey(recPubkey)
	if err != nil {
		return "", errors.New("unmarshal public key failed.")
	}
	recPublicKeyBytes := ecrypto.CompressPubkey(recPublicKey)
	checksum = ripemd160Sum(recPublicKeyBytes)
	rawRecPublicKeyBytes := append(recPublicKeyBytes, checksum[0:4]...)
	return base58.Encode(rawRecPublicKeyBytes), nil
}

//Sign create signature for data by private key
func Sign(privateKey string, data []byte) (string, error) {
	pk, err := getRawPrivateKey(privateKey)
	if err != nil {
		return "", err
	}
	privKey, err := ecrypto.ToECDSA(pk)
	if err != nil {
		return "", err
	}
	sign, err := ecrypto.Sign(sha256Sum(data), privKey)
	if err != nil {
		return "", err
	}
	sign[64] += 4
	sign[64] += 27
	sign = append(sign[64:], sign[0:64]...)
	sign1 := append([]byte{}, sign...)
	checksum := ripemd160Sum(append(sign1, 'K', '1'))
	signature := append(sign, checksum[0:4]...)
	return fmt.Sprintf("%s%s", "SIG_K1_", base58.Encode(signature)), nil
}

//Verify verify signature for data by public key
func Verify(publicKey string, data []byte, signature string) bool {
	if !strings.HasPrefix(signature, "SIG_K1_") {
		return false
	}
	signature = strings.TrimPrefix(signature, "SIG_K1_")
	sigbytes, _ := base58.Decode(signature)
	sign := sigbytes[0:65]
	checksum := sigbytes[65:]
	sign1 := append([]byte{}, sign...)
	ck := ripemd160Sum(append(sign1, 'K', '1'))
	if !bytes.Equal(checksum, ck[0:4]) {
		return false
	}
	sign[0] -= 4
	sign[0] -= 27
	signx := append(sign[1:65], sign[0])
	recPubkey, err := ecrypto.Ecrecover(sha256Sum(data), signx)
	if err != nil {
		return false
	}
	recPublicKey, err := ecrypto.UnmarshalPubkey(recPubkey)
	if err != nil {
		return false
	}
	recPublicKeyBytes := ecrypto.CompressPubkey(recPublicKey)
	checksum = ripemd160Sum(recPublicKeyBytes)
	rawRecPublicKeyBytes := append(recPublicKeyBytes, checksum[0:4]...)
	rawPublicKeyBytes, err := base58.Decode(publicKey)
	if err != nil {
		return false
	}
	if len(rawPublicKeyBytes) == 33 {
		return bytes.Equal(rawPublicKeyBytes, rawRecPublicKeyBytes[0:33])
	} else if len(rawPublicKeyBytes) == 37 {
		return bytes.Equal(rawPublicKeyBytes, rawRecPublicKeyBytes[0:37])
	} else {
		return false
	}
}

//ECCEncrypt encrypt data by public key
func ECCEncrypt(pt []byte, publicKey string) ([]byte, error) {
	publicKeyBytes, err := getRawPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	publicKeyECDSA, err := ecrypto.DecompressPubkey(publicKeyBytes)
	if err != nil {
		return nil, err
	}
	publicKeyECIES := ecies.ImportECDSAPublic(publicKeyECDSA)
	ct, err := ecies.Encrypt(rand.Reader, publicKeyECIES, pt, nil, nil)
	return ct, err
}

//ECCDecrypt decrypt data by private key
func ECCDecrypt(ct []byte, privateKey string) ([]byte, error) {
	privateKeyBytes, err := getRawPrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	privateKeyECDSA, err := ecrypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return nil, err
	}
	privateKeyECIES := ecies.ImportECDSA(privateKeyECDSA)
	pt, err := privateKeyECIES.Decrypt(ct, nil, nil)
	return pt, err
}

func getRawPublicKey(publicKey string) ([]byte, error) {
	pubKey, err := base58.Decode(publicKey)
	if err != nil {
		return nil, err
	}
	if len(pubKey) == 33 && (pubKey[0] == 0x2 || pubKey[0] == 0x3) {
		return pubKey, nil
	}
	if len(pubKey) == 37 && (pubKey[0] == 0x2 || pubKey[0] == 0x3) {
		rawPubKey := pubKey[0:33]
		checksum := pubKey[33:]
		verify := ripemd160Sum(rawPubKey)
		verifyCode := verify[0:4]
		if !bytes.Equal(checksum, verifyCode) {
			return nil, errors.New("checksum is not correct")
		}
		return rawPubKey[0:33], nil
	}
	return nil, errors.New("length of public key must be 33 or 37")
}

func getRawPrivateKey(privateKey string) ([]byte, error) {
	privKey, err := base58.Decode(privateKey)
	if err != nil {
		return nil, err
	}
	if privKey[0] != 0x80 {
		return nil, fmt.Errorf("Expected version %x , instead got %x", 0x80, privKey[0])
	}
	if len(privKey) == 33 {
		return privKey[1:], nil
	}
	if len(privKey) != 37 {
		return nil, errors.New("length of private key must be 33 or 37")
	}
	rawPrivKey := privKey[0:33]
	checksum := privKey[33:]
	verify := sha256Sum(rawPrivKey)
	verify = sha256Sum(verify[:])
	verifyCode := verify[0:4]
	if !bytes.Equal(checksum, verifyCode) {
		return nil, errors.New("checksum is not correct")
	}
	return rawPrivKey[1:], nil
}

func sha256Sum(bytes []byte) []byte {
	h := sha256.New()
	h.Write(bytes)
	return h.Sum(nil)
}

func ripemd160Sum(bytes []byte) []byte {
	h := ripemd160.New()
	h.Write(bytes)
	return h.Sum(nil)
}
