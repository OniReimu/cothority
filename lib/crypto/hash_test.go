package crypto_test

import (
	"bytes"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/edwards/ed25519"
	"io/ioutil"
	"testing"
)

var hashSuite = ed25519.NewAES128SHA256Ed25519(false)

func TestHash(t *testing.T) {
	buf := make([]byte, 245)
	hashed, err := crypto.Hash(hashSuite.Hash(), buf)
	if err != nil {
		t.Fatal("Error hashing" + err.Error())
	}
	hasher := hashSuite.Hash()
	hasher.Write(buf)
	b := hasher.Sum(nil)
	if !bytes.Equal(b, hashed) {
		t.Fatal("Hashes are not equals")
	}
}

func TestHashStream(t *testing.T) {
	var buff bytes.Buffer
	str := "Hello World"
	buff.WriteString(str)
	hashed, err := crypto.HashStream(hashSuite.Hash(), &buff)
	if err != nil {
		t.Fatal("error hashing" + err.Error())
	}
	h := hashSuite.Hash()
	h.Write([]byte(str))
	b := h.Sum(nil)
	if !bytes.Equal(b, hashed) {
		t.Fatal("hashes not equal")
	}
}

func TestHashBytes(t *testing.T) {
	str := "Hello World"
	hashed, err := crypto.HashBytes(hashSuite.Hash(), []byte(str))
	if err != nil {
		t.Fatal("error hashing" + err.Error())
	}
	h := hashSuite.Hash()
	h.Write([]byte(str))
	b := h.Sum(nil)
	if !bytes.Equal(b, hashed) {
		t.Fatal("hashes not equal")
	}
}

func TestHashFile(t *testing.T) {
	tmpfile := "/tmp/hash_test.bin"
	for _, i := range []int{16, 128, 1024} {
		str := make([]byte, i)
		err := ioutil.WriteFile(tmpfile, str, 0777)
		if err != nil {
			t.Fatal("Couldn't write file")
		}

		hash, err := crypto.HashFile(hashSuite.Hash(), tmpfile)
		if err != nil {
			t.Fatal("Couldn't hash", tmpfile, err)
		}
		if len(hash) != 32 {
			t.Fatal("Length of sha256 should be 32")
		}
		hash2, err := crypto.HashFileSuite(hashSuite, tmpfile)
		if bytes.Compare(hash, hash2) != 0 {
			t.Fatal("HashFile and HashFileSuite should give the same result")
		}
	}
}

func TestHashChunk(t *testing.T) {
	tmpfile := "/tmp/hash_test.bin"
	str := make([]byte, 1234)
	err := ioutil.WriteFile(tmpfile, str, 0777)
	if err != nil {
		t.Fatal("Couldn't write file")
	}

	for _, i := range []int{16, 128, 1024} {
		dbg.Lvl3("Reading", i, "bytes")
		hash, err := crypto.HashFileChunk(ed25519.NewAES128SHA256Ed25519(false).Hash(),
			tmpfile, i)
		if err != nil {
			t.Fatal("Couldn't hash", tmpfile, err)
		}
		if len(hash) != 32 {
			t.Fatal("Length of sha256 should be 32")
		}
	}
}

func TestHashSuite(t *testing.T) {
	var buff bytes.Buffer
	content := make([]byte, 100)
	buff.Write(content)
	var buff2 bytes.Buffer
	buff2.Write(content)
	hashed, err := crypto.HashStream(hashSuite.Hash(), &buff)
	hashedSuite, err2 := crypto.HashStreamSuite(hashSuite, &buff2)
	if err != nil || err2 != nil {
		t.Fatal("error hashing" + err.Error() + err2.Error())
	}
	if !bytes.Equal(hashed, hashedSuite) {
		t.Fatal("hashes not equals")
	}
}

func TestHashArgs(t *testing.T) {
	str1 := binstring("cosi")
	str2 := binstring("rocks")
	hash1, err := crypto.HashArgs(hashSuite.Hash(), str1)
	if err != nil {
		t.Fatal(err)
	}
	hash2, err := crypto.HashArgs(hashSuite.Hash(), str1, str1)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(hash1, hash2) == 0 {
		t.Fatal("Making a hash from a string and stringstring should be different")
	}
	hash1, _ = crypto.HashArgsSuite(hashSuite, str1, str2)
	hash2, _ = crypto.HashArgsSuite(hashSuite, str2, str1)
	if bytes.Compare(hash1, hash2) == 0 {
		t.Fatal("Making a hash from str1str2 should be different from str2str1")
	}
}

type binstring string

func (b binstring) MarshalBinary() ([]byte, error) {
	return []byte(b), nil
}
