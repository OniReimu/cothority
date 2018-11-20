package trie

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStaging(t *testing.T) {
	testMemAndDisk(t, testStaging)
}

func testStaging(t *testing.T, db DB) {
	// Initialise a trie.
	testTrie, err := NewTrie(db, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)

	// Create some keys
	for i := 90; i < 100; i++ {
		k := []byte{byte(i)}
		require.NoError(t, testTrie.Set(k, k))
		val, err := testTrie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, k)
	}

	// Create the staging trie
	sTrie := testTrie.MakeStagingTrie()

	// Test that we can get values from the source trie
	for i := 90; i < 100; i++ {
		k := []byte{byte(i)}
		val, err := sTrie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, k)
	}

	// Add new values and test that we can get them.
	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		require.NoError(t, sTrie.Set(k, k))
		val, err := sTrie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, k)
	}

	// Overwrite values, only go up to 95 (not overwriting everything)
	// because we also want to see what happens when deleting
	// non-overwritten values later.
	for i := 90; i < 95; i++ {
		k := []byte{byte(i)}
		v := []byte{byte(i + 1)}
		require.NoError(t, sTrie.Set(k, v))
		val, err := sTrie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, v)
	}

	// Delete values from the staging trie
	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		require.NoError(t, sTrie.Delete(k))
		val, err := sTrie.Get(k)
		require.NoError(t, err)
		require.Nil(t, val)
	}

	// Delete values from the source trie
	for i := 90; i < 100; i++ {
		k := []byte{byte(i)}
		require.NoError(t, sTrie.Delete(k))
		val, err := sTrie.Get(k)
		require.NoError(t, err)
		require.Nil(t, val)
	}
}

func TestStagingCommit(t *testing.T) {
	testMemAndDisk(t, testStagingCommit)
}

func testStagingCommit(t *testing.T, db DB) {
	// Initialise a trie.
	testTrie, err := NewTrie(db, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)
	testTrie.noHashKey = true

	sTrie := testTrie.MakeStagingTrie()

	// Make set/delete transactions and then commit, make sure they exist
	// in the source Trie.
	// Test that we can get values from the source trie
	for i := 100; i < 200; i++ {
		k := []byte{byte(i)}
		require.NoError(t, sTrie.Set(k, k))
	}
	for i := 100; i < 150; i++ {
		k := []byte{byte(i)}
		require.NoError(t, sTrie.Delete(k))
	}

	require.NoError(t, sTrie.Commit())

	for i := 100; i < 150; i++ {
		// missing
		k := []byte{byte(i)}
		val, err := testTrie.Get(k)
		require.NoError(t, err)
		require.Nil(t, val)
	}
	for i := 150; i < 200; i++ {
		// exists
		k := []byte{byte(i)}
		val, err := testTrie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, k)
	}

	// Make more transactions and commit, check that old ones stay the same
	// and new ones exist.
	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		require.NoError(t, sTrie.Set(k, k))
	}
	for i := 100; i < 150; i++ {
		k := []byte{byte(i)}
		require.NoError(t, sTrie.Set(k, k))
	}

	require.NoError(t, sTrie.Commit())

	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		val, err := testTrie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, k)
	}
	for i := 100; i < 200; i++ {
		k := []byte{byte(i)}
		val, err := testTrie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, k)
	}
}

func TestStagingGetRoot(t *testing.T) {
	disk := newDiskDB(t)
	defer delDiskDB(t, disk)

	testTrie, err := NewTrie(disk, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)

	sTrie := testTrie.MakeStagingTrie()

	// We should start with the same root
	initialRoot := testTrie.GetRoot()
	require.NotNil(t, initialRoot)
	require.Equal(t, initialRoot, sTrie.GetRoot())

	// The root of the staging trie should match the real trie after making
	// operations.
	for i := 100; i < 200; i++ {
		k := []byte{byte(i)}
		require.NoError(t, sTrie.Set(k, k))
	}
	for i := 100; i < 150; i++ {
		k := []byte{byte(i)}
		require.NoError(t, sTrie.Delete(k))
	}
	eRoot := sTrie.GetRoot()

	// The initial root shouldn't change.
	require.Equal(t, initialRoot, testTrie.GetRoot())

	// Commit the staging trie, then the source root should match the root
	// on the previously computed staging trie.
	require.NoError(t, sTrie.Commit())
	require.Equal(t, eRoot, testTrie.GetRoot())
	require.Equal(t, eRoot, sTrie.GetRoot())
}

func TestStagingGetProof(t *testing.T) {
	disk := newDiskDB(t)
	defer delDiskDB(t, disk)

	testTrie, err := NewTrie(disk, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)

	sTrie := testTrie.MakeStagingTrie()

	// Make some staging operations and check for proofs.
	var existProofs []*Proof
	var absenceProofs []*Proof
	for i := 0; i < 20; i++ {
		k := []byte{byte(i)}
		require.NoError(t, sTrie.Set(k, k))
	}
	for i := 10; i < 20; i++ {
		k := []byte{byte(i)}
		require.NoError(t, sTrie.Delete(k))
	}

	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		p, err := sTrie.GetProof(k)
		require.NoError(t, err)
		require.True(t, p.Match(k))
		existProofs = append(existProofs, p)
	}
	for i := 10; i < 20; i++ {
		k := []byte{byte(i)}
		p, err := sTrie.GetProof(k)
		ok, err := p.Exists(k)
		require.NoError(t, err)
		require.False(t, ok)
		absenceProofs = append(absenceProofs, p)
	}

	// Commit the operations and make the same proofs on the source trie,
	// make sure they're the same.
	require.NoError(t, sTrie.Commit())
	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		p, err := testTrie.GetProof(k)
		require.NoError(t, err)
		require.True(t, p.Match(k))

		// The root and the leaf are the same, so the proof must be the
		// same due to properties of hash functions.
		require.Equal(t, existProofs[i].GetRoot(), p.GetRoot())
		require.Equal(t, existProofs[i].Leaf.hash(sTrie.source.nonce), p.Leaf.hash(testTrie.nonce))
	}
	for i := 10; i < 20; i++ {
		k := []byte{byte(i)}
		p, err := testTrie.GetProof(k)
		ok, err := p.Exists(k)
		require.NoError(t, err)
		require.False(t, ok)

		// The root and the leaf are the same, so the proof must be the
		// same due to properties of hash functions.
		require.Equal(t, absenceProofs[i-10].GetRoot(), p.GetRoot())
		require.Equal(t, absenceProofs[i-10].Empty.hash(sTrie.source.nonce), p.Empty.hash(testTrie.nonce))
	}
}

func TestStagingClone(t *testing.T) {
	testMemAndDisk(t, testStagingClone)
}

func testStagingClone(t *testing.T, db DB) {
	testTrie, err := NewTrie(db, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)

	// Make some staging operations and get the first root
	sTrie := testTrie.MakeStagingTrie()
	for i := 0; i < 20; i++ {
		k := []byte{byte(i)}
		require.NoError(t, sTrie.Set(k, k))
	}
	for i := 10; i < 20; i++ {
		k := []byte{byte(i)}
		require.NoError(t, sTrie.Delete(k))
	}
	root1 := sTrie.GetRoot()

	sTrie2 := sTrie.Clone()
	require.Equal(t, root1, sTrie2.GetRoot())
}

func TestStagingBatch(t *testing.T) {
	testMemAndDisk(t, testStagingBatch)
}

func testStagingBatch(t *testing.T, db DB) {
	testTrie, err := NewTrie(db, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)

	// Make two staging tries, batch operation should produce the same
	// result as the normal operation.
	sTrie1 := testTrie.MakeStagingTrie()
	sTrie2 := testTrie.MakeStagingTrie()

	var pairs []KVPair
	for i := 0; i < 20; i++ {
		k := []byte{byte(i)}
		pairs = append(pairs, kvPair{
			op:  OpSet,
			key: k,
			val: k,
		})
		require.NoError(t, sTrie1.Set(k, k))
	}
	for i := 10; i < 20; i++ {
		k := []byte{byte(i)}
		pairs = append(pairs, kvPair{
			op:  OpDel,
			key: k,
			val: k,
		})
		require.NoError(t, sTrie1.Delete(k))
	}
	root1 := sTrie1.GetRoot()

	require.NoError(t, sTrie2.Batch(pairs))
	require.Equal(t, root1, sTrie2.GetRoot())
}