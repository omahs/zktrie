package zktrie

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
)

var Q *big.Int
var setHashScheme sync.Once
var hashNotInitErr = fmt.Errorf("hash scheme is not setup yet, call InitHashScheme before using the library")

func dummyHash([]*big.Int) (*big.Int, error) {
	return big.NewInt(0), hashNotInitErr
}

var hashScheme func([]*big.Int) (*big.Int, error) = dummyHash

func init() {
	qString := "21888242871839275222246405745257275088548364400416034343698204186575808495617"
	var ok bool
	Q, ok = new(big.Int).SetString(qString, 10) //nolint:gomnd
	if !ok {
		panic(fmt.Sprintf("Bad base 10 string %s", qString))
	}
}

func InitHashScheme(f func([]*big.Int) (*big.Int, error)) {
	setHashScheme.Do(func() {
		hashScheme = f
	})
}

// CheckBigIntInField checks if given *big.Int fits in a Field Q element
func CheckBigIntInField(a *big.Int) bool {
	return a.Cmp(Q) == -1
}

const numCharPrint = 8

// HashByteLen is the length of the Hash byte array
const HashByteLen = 32

var HashZero = Hash{}

// Hash is the generic type to store the hash in the MerkleTree, encoded in little endian
type Hash [HashByteLen]byte

// MarshalText implements the marshaler for the Hash type
func (h Hash) MarshalText() ([]byte, error) {
	return []byte(h.BigInt().String()), nil
}

// UnmarshalText implements the unmarshaler for the Hash type
func (h *Hash) UnmarshalText(b []byte) error {
	ha, err := NewHashFromString(string(b))
	copy(h[:], ha[:])
	return err
}

// String returns decimal representation in string format of the Hash
func (h Hash) String() string {
	s := h.BigInt().String()
	if len(s) < numCharPrint {
		return s
	}
	return s[0:numCharPrint] + "..."
}

// Hex returns the hexadecimal representation of the Hash
func (h Hash) Hex() string {
	return hex.EncodeToString(h.Bytes())
}

// BigInt returns the *big.Int representation of the *Hash
func (h *Hash) BigInt() *big.Int {
	return big.NewInt(0).SetBytes(ReverseByteOrder(h[:]))
}

// Bytes returns the byte representation of the *Hash in big-endian encoding.
// The function converts the byte order from little endian to big endian.
func (h *Hash) Bytes() []byte {
	b := [HashByteLen]byte{}
	copy(b[:], h[:])
	return ReverseByteOrder(b[:])
}

// NewBigIntFromHashBytes returns a *big.Int from a byte array, swapping the
// endianness in the process. This is the intended method to get a *big.Int
// from a byte array that previously has ben generated by the Hash.Bytes()
// method.
func NewBigIntFromHashBytes(b []byte) (*big.Int, error) {
	if len(b) != HashByteLen {
		return nil, fmt.Errorf("expected %d bytes, but got %d bytes", HashByteLen, len(b))
	}
	bi := new(big.Int).SetBytes(b)
	if !CheckBigIntInField(bi) {
		return nil, fmt.Errorf("NewBigIntFromHashBytes: Value not inside the Finite Field")
	}
	return bi, nil
}

// NewHashFromBigInt returns a *Hash representation of the given *big.Int
func NewHashFromBigInt(b *big.Int) *Hash {
	r := &Hash{}
	copy(r[:], ReverseByteOrder(b.Bytes()))
	return r
}

// NewHashFromBytes returns a *Hash from a byte array considered to be
// a represent of big-endian integer, it swapping the endianness
// in the process.
func NewHashFromBytes(b []byte) *Hash {
	var h Hash
	copy(h[:], ReverseByteOrder(b))
	return &h
}

// NewHashFromCheckedBytes is the intended method to get a *Hash from a byte array
// that previously has ben generated by the Hash.Bytes() method. so it check the
// size of bytes to be expected length
func NewHashFromCheckedBytes(b []byte) (*Hash, error) {
	if len(b) != HashByteLen {
		return nil, fmt.Errorf("expected %d bytes, but got %d bytes", HashByteLen, len(b))
	}
	return NewHashFromBytes(b), nil
}

// NewHashFromString returns a *Hash representation of the given decimal string
func NewHashFromString(s string) (*Hash, error) {
	bi, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("cannot parse the string to Hash")
	}
	return NewHashFromBigInt(bi), nil
}
