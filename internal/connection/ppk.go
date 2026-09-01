package connection

import (
	"crypto/ed25519"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	gossh "golang.org/x/crypto/ssh"
)

// isPPKFile reports whether data is a PuTTY private key file.
func isPPKFile(data []byte) bool {
	prefix := "PuTTY-User-Key-File-"
	if len(data) < len(prefix) {
		return false
	}
	return string(data[:len(prefix)]) == prefix
}

type ppkKey struct {
	version     int
	keyType     string
	encryption  string
	publicData  []byte
	privateData []byte
}

// parsePPKSigner parses a PuTTY .ppk key file and returns an ssh.Signer.
// passphrase is reserved for future encrypted-key support.
func parsePPKSigner(data []byte, passphrase string) (gossh.Signer, error) {
	_ = passphrase // reserved for encrypted PPK support
	pk, err := decodePPK(data)
	if err != nil {
		return nil, fmt.Errorf("PPK parse error: %w", err)
	}

	if pk.encryption != "none" {
		return nil, fmt.Errorf(
			"chiffrement PPK (%s) non supporté - convertissez la clé en format OpenSSH avec PuTTYgen : Conversions → Export OpenSSH key",
			pk.encryption,
		)
	}

	switch pk.keyType {
	case "ssh-rsa":
		return buildRSASigner(pk)
	case "ssh-ed25519":
		return buildEd25519Signer(pk)
	default:
		return nil, fmt.Errorf(
			"type de clé PPK %q non supporté - convertissez avec PuTTYgen : Conversions → Export OpenSSH key",
			pk.keyType,
		)
	}
}

func decodePPK(data []byte) (*ppkKey, error) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	pk := &ppkKey{}
	i := 0

	// Consume blank lines then read "Key: Value".
	readField := func(want string) (string, error) {
		for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			i++
		}
		if i >= len(lines) {
			return "", fmt.Errorf("PPK: expected field %q but reached EOF", want)
		}
		line := lines[i]
		i++
		key, val, ok := strings.Cut(line, ": ")
		if !ok {
			return "", fmt.Errorf("PPK: invalid line (missing ': '): %q", line)
		}
		if key != want {
			return "", fmt.Errorf("PPK: expected field %q, got %q", want, key)
		}
		return val, nil
	}

	// Read N base64 lines and decode them.
	readBlob := func(countStr string) ([]byte, error) {
		n, err := strconv.Atoi(strings.TrimSpace(countStr))
		if err != nil {
			return nil, fmt.Errorf("PPK: invalid line count %q", countStr)
		}
		var sb strings.Builder
		for range n {
			if i >= len(lines) {
				return nil, fmt.Errorf("PPK: unexpected EOF in base64 block")
			}
			sb.WriteString(strings.TrimSpace(lines[i]))
			i++
		}
		return base64.StdEncoding.DecodeString(sb.String())
	}

	// First line: "PuTTY-User-Key-File-N: keytype"
	if i >= len(lines) {
		return nil, fmt.Errorf("PPK: empty file")
	}
	header := lines[i]
	i++
	switch {
	case strings.HasPrefix(header, "PuTTY-User-Key-File-3: "):
		pk.version = 3
		pk.keyType = strings.TrimPrefix(header, "PuTTY-User-Key-File-3: ")
	case strings.HasPrefix(header, "PuTTY-User-Key-File-2: "):
		pk.version = 2
		pk.keyType = strings.TrimPrefix(header, "PuTTY-User-Key-File-2: ")
	default:
		return nil, fmt.Errorf("PPK: unrecognized header: %q", header)
	}

	var err error
	pk.encryption, err = readField("Encryption")
	if err != nil {
		return nil, err
	}
	if _, err = readField("Comment"); err != nil { // comment not needed
		return nil, err
	}
	pubCount, err := readField("Public-Lines")
	if err != nil {
		return nil, err
	}
	pk.publicData, err = readBlob(pubCount)
	if err != nil {
		return nil, fmt.Errorf("PPK public blob: %w", err)
	}
	privCount, err := readField("Private-Lines")
	if err != nil {
		return nil, err
	}
	pk.privateData, err = readBlob(privCount)
	if err != nil {
		return nil, fmt.Errorf("PPK private blob: %w", err)
	}
	return pk, nil
}

// sshWire reads SSH wire-format fields (big-endian uint32 length prefix).
type sshWire struct {
	b   []byte
	pos int
}

func (w *sshWire) readBytes() ([]byte, error) {
	if w.pos+4 > len(w.b) {
		return nil, fmt.Errorf("SSH wire: buffer underflow reading length")
	}
	n := int(binary.BigEndian.Uint32(w.b[w.pos:]))
	w.pos += 4
	if w.pos+n > len(w.b) {
		return nil, fmt.Errorf("SSH wire: buffer underflow reading %d bytes", n)
	}
	out := w.b[w.pos : w.pos+n]
	w.pos += n
	return out, nil
}

func (w *sshWire) readString() (string, error) {
	b, err := w.readBytes()
	return string(b), err
}

// readMPInt reads a length-prefixed big-endian integer (SSH mpint).
// The leading zero byte (used to signal positive in two's complement) is handled
// by SetBytes treating all bytes as unsigned magnitude.
func (w *sshWire) readMPInt() (*big.Int, error) {
	b, err := w.readBytes()
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(b), nil
}

// buildRSASigner constructs an RSA signer from a decoded PPK key.
// Public blob:  string(keyType) + mpint(e) + mpint(n)
// Private blob: mpint(d) + mpint(p) + mpint(q) + mpint(iqmp)
func buildRSASigner(pk *ppkKey) (gossh.Signer, error) {
	pub := &sshWire{b: pk.publicData}
	if _, err := pub.readString(); err != nil { // skip key type string
		return nil, fmt.Errorf("RSA pub keytype: %w", err)
	}
	e, err := pub.readMPInt()
	if err != nil {
		return nil, fmt.Errorf("RSA pub e: %w", err)
	}
	n, err := pub.readMPInt()
	if err != nil {
		return nil, fmt.Errorf("RSA pub n: %w", err)
	}

	priv := &sshWire{b: pk.privateData}
	d, err := priv.readMPInt()
	if err != nil {
		return nil, fmt.Errorf("RSA priv d: %w", err)
	}
	p, err := priv.readMPInt()
	if err != nil {
		return nil, fmt.Errorf("RSA priv p: %w", err)
	}
	q, err := priv.readMPInt()
	if err != nil {
		return nil, fmt.Errorf("RSA priv q: %w", err)
	}
	// iqmp = q^-1 mod p - Precompute() recomputes this automatically.
	if _, err = priv.readMPInt(); err != nil {
		return nil, fmt.Errorf("RSA priv iqmp: %w", err)
	}

	key := &rsa.PrivateKey{
		PublicKey: rsa.PublicKey{N: n, E: int(e.Int64())},
		D:         d,
		Primes:    []*big.Int{p, q},
	}
	key.Precompute()
	if err := key.Validate(); err != nil {
		return nil, fmt.Errorf("RSA key validation: %w", err)
	}
	return gossh.NewSignerFromKey(key)
}

// buildEd25519Signer constructs an Ed25519 signer from a decoded PPK key.
// Private blob: string(64 bytes = seed + public key) - same layout as Go's ed25519.PrivateKey.
func buildEd25519Signer(pk *ppkKey) (gossh.Signer, error) {
	priv := &sshWire{b: pk.privateData}
	privBytes, err := priv.readBytes()
	if err != nil {
		return nil, fmt.Errorf("Ed25519 priv: %w", err)
	}
	if len(privBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("Ed25519 private key: expected %d bytes, got %d", ed25519.PrivateKeySize, len(privBytes))
	}
	return gossh.NewSignerFromKey(ed25519.PrivateKey(privBytes))
}
