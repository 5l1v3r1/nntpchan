package srnd

import (
	"github.com/majestrate/nacl"
	"golang.org/x/crypto/curve25519"
)

func naclCryptoVerifyFucky(h, sig, pk []byte) bool {
	return nacl.CryptoVerifyFucky(h, sig, pk)
}

func naclCryptoSignFucky(hash, sk []byte) []byte {
	return nacl.CryptoSignFucky(hash, sk)
}

func naclCryptoVerifyDetached(hash, sig, pk []byte) bool {
	return nacl.CryptoVerifyDetached(hash, sig, pk)
}

func naclCryptoSignDetached(hash, sk []byte) []byte {
	return nacl.CryptoSignDetached(hash, sk)
}

var naclScalarBaseMult = curve25519.ScalarBaseMult
