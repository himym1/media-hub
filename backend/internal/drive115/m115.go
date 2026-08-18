package drive115

// m115 implements the encrypted payload format required by 115's
// app/chrome/downurl endpoint. It is adapted from the MIT-licensed
// github.com/SheltonZhu/115driver/pkg/crypto/m115 implementation.

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"math/big"
)

type m115Key [16]byte

func generateM115Key() (m115Key, error) {
	key := m115Key{}
	_, err := io.ReadFull(rand.Reader, key[:])
	return key, err
}

func encodeM115(input []byte, key m115Key) (string, error) {
	buffer := make([]byte, 16+len(input))
	copy(buffer, key[:])
	copy(buffer[16:], input)
	xorM115(buffer[16:], deriveM115Key(key[:], 4))
	reverseM115(buffer[16:])
	xorM115(buffer[16:], m115ClientKey)
	encrypted, err := encryptM115RSA(buffer)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func decodeM115(input string, key m115Key) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return nil, err
	}
	data, err = decryptM115RSA(data)
	if err != nil {
		return nil, err
	}
	if len(data) < 16 {
		return nil, fmt.Errorf("m115 response is too short")
	}
	output := append([]byte(nil), data[16:]...)
	xorM115(output, deriveM115Key(data[:16], 12))
	reverseM115(output)
	xorM115(output, deriveM115Key(key[:], 4))
	return output, nil
}

func reverseM115(data []byte) {
	for left, right := 0, len(data)-1; left < right; left, right = left+1, right-1 {
		data[left], data[right] = data[right], data[left]
	}
}

var (
	m115Modulus, _ = new(big.Int).SetString(
		"8686980c0f5a24c4b9d43020cd2c22703ff3f450756529058b1cf88f09b86021"+
			"36477198a6e2683149659bd122c33592fdb5ad47944ad1ea4d36c6b172aad633"+
			"8c3bb6ac6227502d010993ac967d1aef00f0c8e038de2e4d3bc2ec368af2e9f1"+
			"0a6f1eda4f7262f136420c07c331b871bf139f74f3010e3c4fe57df3afb71683", 16)
	m115Exponent, _  = new(big.Int).SetString("10001", 16)
	m115RSAKeyLength = m115Modulus.BitLen() / 8
)

func encryptM115RSA(input []byte) ([]byte, error) {
	var output bytes.Buffer
	for len(input) > 0 {
		size := min(len(input), m115RSAKeyLength-11)
		if err := encryptM115RSABlock(input[:size], &output); err != nil {
			return nil, err
		}
		input = input[size:]
	}
	return output.Bytes(), nil
}

func encryptM115RSABlock(input []byte, output io.Writer) error {
	padding := make([]byte, m115RSAKeyLength-len(input)-3)
	if _, err := rand.Read(padding); err != nil {
		return err
	}
	block := make([]byte, m115RSAKeyLength)
	block[0], block[1] = 0, 2
	for index, value := range padding {
		block[index+2] = value%0xff + 1
	}
	block[len(padding)+2] = 0
	copy(block[len(padding)+3:], input)
	encrypted := new(big.Int).Exp(new(big.Int).SetBytes(block), m115Exponent, m115Modulus).Bytes()
	if fill := m115RSAKeyLength - len(encrypted); fill > 0 {
		if _, err := output.Write(make([]byte, fill)); err != nil {
			return err
		}
	}
	_, err := output.Write(encrypted)
	return err
}

func decryptM115RSA(input []byte) ([]byte, error) {
	if len(input) == 0 || len(input)%m115RSAKeyLength != 0 {
		return nil, fmt.Errorf("invalid m115 RSA payload length")
	}
	var output bytes.Buffer
	for len(input) > 0 {
		decrypted := new(big.Int).Exp(new(big.Int).SetBytes(input[:m115RSAKeyLength]), m115Exponent, m115Modulus).Bytes()
		separator := bytes.IndexByte(decrypted[1:], 0)
		if separator < 0 {
			return nil, fmt.Errorf("invalid m115 RSA padding")
		}
		if _, err := output.Write(decrypted[separator+2:]); err != nil {
			return nil, err
		}
		input = input[m115RSAKeyLength:]
	}
	return output.Bytes(), nil
}

var (
	m115KeySeed = []byte{
		0xf0, 0xe5, 0x69, 0xae, 0xbf, 0xdc, 0xbf, 0x8a, 0x1a, 0x45, 0xe8, 0xbe, 0x7d, 0xa6, 0x73, 0xb8,
		0xde, 0x8f, 0xe7, 0xc4, 0x45, 0xda, 0x86, 0xc4, 0x9b, 0x64, 0x8b, 0x14, 0x6a, 0xb4, 0xf1, 0xaa,
		0x38, 0x01, 0x35, 0x9e, 0x26, 0x69, 0x2c, 0x86, 0x00, 0x6b, 0x4f, 0xa5, 0x36, 0x34, 0x62, 0xa6,
		0x2a, 0x96, 0x68, 0x18, 0xf2, 0x4a, 0xfd, 0xbd, 0x6b, 0x97, 0x8f, 0x4d, 0x8f, 0x89, 0x13, 0xb7,
		0x6c, 0x8e, 0x93, 0xed, 0x0e, 0x0d, 0x48, 0x3e, 0xd7, 0x2f, 0x88, 0xd8, 0xfe, 0xfe, 0x7e, 0x86,
		0x50, 0x95, 0x4f, 0xd1, 0xeb, 0x83, 0x26, 0x34, 0xdb, 0x66, 0x7b, 0x9c, 0x7e, 0x9d, 0x7a, 0x81,
		0x32, 0xea, 0xb6, 0x33, 0xde, 0x3a, 0xa9, 0x59, 0x34, 0x66, 0x3b, 0xaa, 0xba, 0x81, 0x60, 0x48,
		0xb9, 0xd5, 0x81, 0x9c, 0xf8, 0x6c, 0x84, 0x77, 0xff, 0x54, 0x78, 0x26, 0x5f, 0xbe, 0xe8, 0x1e,
		0x36, 0x9f, 0x34, 0x80, 0x5c, 0x45, 0x2c, 0x9b, 0x76, 0xd5, 0x1b, 0x8f, 0xcc, 0xc3, 0xb8, 0xf5,
	}
	m115ClientKey = []byte{0x78, 0x06, 0xad, 0x4c, 0x33, 0x86, 0x5d, 0x18, 0x4c, 0x01, 0x3f, 0x46}
)

func deriveM115Key(seed []byte, size int) []byte {
	key := make([]byte, size)
	for index := range key {
		key[index] = (seed[index] + m115KeySeed[size*index]) & 0xff
		key[index] ^= m115KeySeed[size*(size-index-1)]
	}
	return key
}

func xorM115(data, key []byte) {
	mod := len(data) % 4
	for index := 0; index < mod; index++ {
		data[index] ^= key[index%len(key)]
	}
	for index := mod; index < len(data); index++ {
		data[index] ^= key[(index-mod)%len(key)]
	}
}
