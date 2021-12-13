package random

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/crypto/chacha20"
)

// TODO: update description, RFC and lengths

// We use Chacha20, to build a cryptographically secure random number generator
// that uses the ChaCha algorithm.
//
// ChaCha is a stream cipher designed by Daniel J. Bernstein[^1], that we use as an PRG. It is
// an improved variant of the Salsa20 cipher family.
//
// We use Chacha20 with a 256-bit key, a 192-bit stream identifier and a 32-bit counter as
// as specified in RFC 8439 [^2].
// The encryption key is used as the PRG seed while the stream identifer is used as a nonce
// to customize the PRG. The PRG outputs are the successive encryptions of a constant message.
//
// A 32-bit counter over 64-byte blocks allows 256 GiB of output before cycling,
// and the stream identifier allows 2^192 unique streams of output per seed.
// It is the caller's responsibility to avoid the PRG output cycling.
//
// [^1]: D. J. Bernstein, [*ChaCha, a variant of Salsa20*](
//       https://cr.yp.to/chacha.html)
//
// [^2]: [RFC 8439: ChaCha20 and Poly1305 for IETF Protocols](
//       https://datatracker.ietf.org/doc/html/rfc8439)

// The PRG core, implements the randCore interface
type chachaCore struct {
	cipher chacha20.Cipher

	// Only used for State/Restore functionality

	// Counter of bytes encrypted so far by the sream cipher.
	// Note this is different than the internal counter of the chacha state
	// that counts the encrypted blocks of 512 bits.
	bytesCounter uint64
	// initial seed
	seed []byte
	// initial customizer
	customizer []byte
}

// The main PRG, implements the Rand interface
type chachaPRG struct {
	genericPRG
	core *chachaCore
}

const (
	keySize   = chacha20.KeySize
	nonceSize = chacha20.NonceSize

	// Chacha20SeedLen is the seed length of the Chacha based PRG, it is fixed to 32 bytes.
	Chacha20SeedLen = keySize
	// Chacha20CustomizerMaxLen is the maximum length of the nonce used as a PRG customizer, it is fixed to 24 bytes.
	// Shorter customizers are padded by zeros to 24 bytes.
	Chacha20CustomizerMaxLen = nonceSize
)

// NewChacha20 returns a new Chacha20-based PRG, seeded with
// the input seed (32 bytes) and a customizer (up to 12 bytes).
//
// It is recommended to sample the seed uniformly at random.
// The function errors if the the seed is different than 32 bytes,
// or if the customizer is larger than 12 bytes.
func NewChacha20(seed []byte, customizer []byte) (*chachaPRG, error) {

	// check the key size
	if len(seed) != Chacha20SeedLen {
		return nil, fmt.Errorf("chacha20 seed length should be %d, got %d", Chacha20SeedLen, len(seed))
	}

	// TODO: update by adding a maximum length and padding
	// check the nonce size
	if len(customizer) != Chacha20CustomizerMaxLen {
		return nil, fmt.Errorf("new Rand streamID should be %d bytes", Chacha20CustomizerMaxLen)
	}

	// create the Chacha20 state, initialized with the seed as a key, and the customizer as a streamID.
	chacha, err := chacha20.NewUnauthenticatedCipher(seed, customizer)
	if err != nil {
		return nil, fmt.Errorf("chacha20 instance creation failed: %w", err)
	}

	// init the state
	core := &chachaCore{
		cipher:       *chacha,
		bytesCounter: 0,
		seed:         seed,
		customizer:   customizer,
	}
	prg := &chachaPRG{
		genericPRG: genericPRG{
			randCore: core,
		},
		core: core,
	}
	return prg, nil
}

// TODO : update GoDoc
func (c *chachaCore) Read(buffer []byte) {
	// encrypt an empty message
	// TODO: optimize by using a constant empty buffer (check if less than 512)
	for i := 0; i < len(buffer); i++ {
		buffer[i] = 0
	}
	c.cipher.XORKeyStream(buffer, buffer)
	// increase the counter
	c.bytesCounter += uint64(len(buffer))
}

// State returns the internal state of the concatenated Chacha20s
// (this is used for serde purposes)
// TODO: update the name (serialize, encode, marshall ?)
func (c *chachaPRG) State() []byte {
	bytes := append(c.core.seed, c.core.customizer...)
	counter := make([]byte, 8)
	binary.LittleEndian.PutUint64(counter, c.core.bytesCounter)
	bytes = append(bytes, counter...)
	// output is seed || streamID || counter
	return bytes
}

// TODO: add go doc
func Restore(stateBytes []byte) (*chachaPRG, error) {
	// inpout should be seed (32 bytes) || streamID (12 bytes) || bytesCounter (8 bytes)
	const expectedLen = keySize + nonceSize + 8

	// check input length
	if len(stateBytes) != expectedLen {
		return nil, fmt.Errorf("Rand state length should be of %d bytes, got %d", expectedLen, len(stateBytes))
	}

	seed := stateBytes[:keySize]
	streamID := stateBytes[keySize : keySize+nonceSize]
	bytesCounter := binary.LittleEndian.Uint64(stateBytes[keySize+nonceSize:])

	// create the Chacha20 instance with seed and streamID
	chacha, err := chacha20.NewUnauthenticatedCipher(seed, streamID)
	if err != nil {
		return nil, fmt.Errorf("Chacha20 instance creation failed: %w", err)
	}
	// set the block counter, each chacha internal block is 512 bits
	const bytesPerBlock = 512 >> 3
	blockCount := uint32(bytesCounter / bytesPerBlock)
	remainingBytes := bytesCounter % bytesPerBlock
	chacha.SetCounter(blockCount)
	// query the remaining bytes and to catch the stored chacha state
	remainderStream := make([]byte, remainingBytes)
	chacha.XORKeyStream(remainderStream, remainderStream)

	core := &chachaCore{
		cipher:       *chacha,
		bytesCounter: bytesCounter,
		seed:         seed,
		customizer:   streamID,
	}

	prg := &chachaPRG{
		genericPRG: genericPRG{
			randCore: core,
		},
		core: core,
	}
	return prg, nil
}
