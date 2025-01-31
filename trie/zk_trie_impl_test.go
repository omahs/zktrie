package trie

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	zkt "github.com/scroll-tech/zktrie/types"
)

// we do not need zktrie impl anymore, only made a wrapper for adapting testing
type zkTrieImplTestWrapper struct {
	*ZkTrieImpl
}

func newZkTrieImpl(storage ZktrieDatabase, maxLevels int) (*zkTrieImplTestWrapper, error) {
	return newZkTrieImplWithRoot(storage, &zkt.HashZero, maxLevels)
}

// NewZkTrieImplWithRoot loads a new ZkTrieImpl. If in the storage already exists one
// will open that one, if not, will create a new one.
func newZkTrieImplWithRoot(storage ZktrieDatabase, root *zkt.Hash, maxLevels int) (*zkTrieImplTestWrapper, error) {
	impl, err := NewZkTrieImplWithRoot(storage, root, maxLevels)
	if err != nil {
		return nil, err
	}

	return &zkTrieImplTestWrapper{impl}, nil
}

func (mt *zkTrieImplTestWrapper) AddWord(kPreimage, vPreimage *zkt.Byte32) error {

	if v, _ := mt.TryGet(kPreimage[:]); v != nil {
		return ErrEntryIndexAlreadyExists
	}

	return mt.ZkTrieImpl.TryUpdate(zkt.NewHashFromBytes(kPreimage[:]), 1, []zkt.Byte32{*vPreimage})
}

func (mt *zkTrieImplTestWrapper) GetLeafNodeByWord(kPreimage *zkt.Byte32) (*Node, error) {
	return mt.ZkTrieImpl.GetLeafNode(zkt.NewHashFromBytes(kPreimage[:]))
}

func (mt *zkTrieImplTestWrapper) UpdateWord(kPreimage, vPreimage *zkt.Byte32) error {
	return mt.ZkTrieImpl.TryUpdate(zkt.NewHashFromBytes(kPreimage[:]), 1, []zkt.Byte32{*vPreimage})
}

func (mt *zkTrieImplTestWrapper) DeleteWord(kPreimage *zkt.Byte32) error {
	return mt.ZkTrieImpl.TryDelete(zkt.NewHashFromBytes(kPreimage[:]))
}

func (mt *zkTrieImplTestWrapper) TryGet(key []byte) ([]byte, error) {
	return mt.ZkTrieImpl.TryGet(zkt.NewHashFromBytes(key))
}

func newTestingMerkle(t *testing.T, numLevels int) *zkTrieImplTestWrapper {
	mt, err := newZkTrieImpl(NewZkTrieMemoryDb(), numLevels)
	if err != nil {
		t.Fatal(err)
		return nil
	}
	mt.Debug = true
	assert.Equal(t, numLevels, mt.MaxLevels())
	return mt
}

func TestMerkleTree_Init(t *testing.T) {
	maxLevels := 248
	db := NewZkTrieMemoryDb()

	t.Run("Test NewZkTrieImpl", func(t *testing.T) {
		mt, err := NewZkTrieImpl(db, maxLevels)
		assert.NoError(t, err)
		assert.Equal(t, zkt.HashZero.Bytes(), mt.Root().Bytes())
	})

	t.Run("Test NewZkTrieImplWithRoot with zero hash root", func(t *testing.T) {
		mt, err := NewZkTrieImplWithRoot(db, &zkt.HashZero, maxLevels)
		assert.NoError(t, err)
		assert.Equal(t, zkt.HashZero.Bytes(), mt.Root().Bytes())
	})

	t.Run("Test NewZkTrieImplWithRoot with non-zero hash root and node exists", func(t *testing.T) {
		mt1, err := NewZkTrieImplWithRoot(db, &zkt.HashZero, maxLevels)
		assert.NoError(t, err)
		assert.Equal(t, zkt.HashZero.Bytes(), mt1.Root().Bytes())
		err = mt1.TryUpdate(zkt.NewHashFromBytes([]byte{1}), 1, []zkt.Byte32{{byte(1)}})
		assert.NoError(t, err)

		mt2, err := NewZkTrieImplWithRoot(db, mt1.Root(), maxLevels)
		assert.NoError(t, err)
		assert.Equal(t, maxLevels, mt2.maxLevels)
		assert.Equal(t, "2120d2ba46996633e29ae090371f704ae8a1fac40c782030824e93af0540e663", mt2.Root().Hex())
	})

	t.Run("Test NewZkTrieImplWithRoot with non-zero hash root and node does not exist", func(t *testing.T) {
		root := zkt.NewHashFromBytes([]byte{1, 2, 3, 4, 5})

		mt, err := NewZkTrieImplWithRoot(db, root, maxLevels)
		assert.Error(t, err)
		assert.Nil(t, mt)
	})
}

func TestMerkleTree_AddUpdateGetWord(t *testing.T) {
	mt := newTestingMerkle(t, 10)

	testData := []struct {
		key        byte
		initialVal byte
		updatedVal byte
	}{
		{1, 2, 7},
		{3, 4, 8},
		{5, 6, 9},
	}

	for _, td := range testData {
		err := mt.AddWord(zkt.NewByte32FromBytes([]byte{td.key}), &zkt.Byte32{td.initialVal})
		assert.NoError(t, err)

		node, err := mt.GetLeafNodeByWord(zkt.NewByte32FromBytes([]byte{td.key}))
		assert.NoError(t, err)
		assert.Equal(t, 1, len(node.ValuePreimage))
		assert.Equal(t, (&zkt.Byte32{td.initialVal})[:], node.ValuePreimage[0][:])
	}

	err := mt.AddWord(zkt.NewByte32FromBytes([]byte{5}), &zkt.Byte32{7})
	assert.Equal(t, ErrEntryIndexAlreadyExists, err)

	for _, td := range testData {
		err := mt.UpdateWord(zkt.NewByte32FromBytes([]byte{td.key}), &zkt.Byte32{td.updatedVal})
		assert.NoError(t, err)

		node, err := mt.GetLeafNodeByWord(zkt.NewByte32FromBytes([]byte{td.key}))
		assert.NoError(t, err)
		assert.Equal(t, 1, len(node.ValuePreimage))
		assert.Equal(t, (&zkt.Byte32{td.updatedVal})[:], node.ValuePreimage[0][:])
	}

	_, err = mt.GetLeafNodeByWord(&zkt.Byte32{100})
	assert.Equal(t, ErrKeyNotFound, err)
}

func TestMerkleTree_Deletion(t *testing.T) {
	t.Run("Check root consistency", func(t *testing.T) {
		mt := newTestingMerkle(t, 10)
		hashes := make([][]byte, 7)
		hashes[0] = mt.Root().Bytes()

		for i := 0; i < 6; i++ {
			err := mt.AddWord(zkt.NewByte32FromBytes([]byte{byte(i)}), &zkt.Byte32{byte(i)})
			assert.NoError(t, err)
			hashes[i+1] = mt.Root().Bytes()
		}

		for i := 5; i >= 0; i-- {
			err := mt.DeleteWord(zkt.NewByte32FromBytes([]byte{byte(i)}))
			assert.NoError(t, err)
			assert.Equal(t, hashes[i], mt.Root().Bytes())
		}
	})

	t.Run("Check depth", func(t *testing.T) {
		mt := newTestingMerkle(t, 10)
		key1 := zkt.NewByte32FromBytes([]byte{67}) //0b1000011
		err := mt.AddWord(key1, &zkt.Byte32{67})
		rootPhase1 := mt.Root().Bytes()
		assert.NoError(t, err)
		key2 := zkt.NewByte32FromBytes([]byte{131}) //0b10000011
		err = mt.AddWord(key2, &zkt.Byte32{131})
		assert.NoError(t, err)
		rootPhase2 := mt.Root().Bytes()

		assertKeyDepth := func(key *zkt.Byte32, expectedDep int) {
			levelCnt := 0
			err := mt.prove(zkt.NewHashFromBytes(key[:]), 0,
				func(*Node) error {
					levelCnt++
					return nil
				},
			)
			assert.NoError(t, err)
			assert.Equal(t, expectedDep, levelCnt)
		}

		assertKeyDepth(key1, 8)
		assertKeyDepth(key2, 8)

		err = mt.DeleteWord(key2)
		assert.NoError(t, err)

		assertKeyDepth(key1, 1)
		assert.Equal(t, rootPhase1, mt.Root().Bytes())

		err = mt.AddWord(key2, &zkt.Byte32{131})
		assert.NoError(t, err)
		assert.Equal(t, rootPhase2, mt.Root().Bytes())
		assertKeyDepth(key1, 8)

		// delete node with parent sibling (fail before a410f14)
		key3 := zkt.NewByte32FromBytes([]byte{19}) //0b10011
		err = mt.AddWord(key3, &zkt.Byte32{19})
		assert.NoError(t, err)

		err = mt.DeleteWord(key3)
		assert.NoError(t, err)
		assertKeyDepth(key1, 8)
		assert.Equal(t, rootPhase2, mt.Root().Bytes())

		key4 := zkt.NewByte32FromBytes([]byte{4}) //0b100, so it is 2 level node (fail before d1c735)
		err = mt.AddWord(key4, &zkt.Byte32{4})
		assert.NoError(t, err)

		assertKeyDepth(key4, 2)

		err = mt.DeleteWord(key4)
		assert.NoError(t, err)
		assert.Equal(t, rootPhase2, mt.Root().Bytes())
	})
}

func TestZkTrieImpl_Add(t *testing.T) {
	k1 := zkt.NewByte32FromBytes([]byte{1})
	k2 := zkt.NewByte32FromBytes([]byte{2})
	k3 := zkt.NewByte32FromBytes([]byte{3})

	kvMap := map[*zkt.Byte32]*zkt.Byte32{
		k1: zkt.NewByte32FromBytes([]byte{1}),
		k2: zkt.NewByte32FromBytes([]byte{2}),
		k3: zkt.NewByte32FromBytes([]byte{3}),
	}

	t.Run("Add 1 and 2 in different orders", func(t *testing.T) {
		orders := [][]*zkt.Byte32{
			{k1, k2},
			{k2, k1},
		}

		roots := make([][]byte, len(orders))
		for i, order := range orders {
			mt := newTestingMerkle(t, 10)
			for _, key := range order {
				value := kvMap[key]
				err := mt.AddWord(key, value)
				assert.NoError(t, err)
			}
			roots[i] = mt.Root().Bytes()
		}

		assert.Equal(t, roots[0], roots[1])
	})

	t.Run("Add 1, 2, 3 in different orders", func(t *testing.T) {
		orders := [][]*zkt.Byte32{
			{k1, k2, k3},
			{k1, k3, k2},
			{k2, k1, k3},
			{k2, k3, k1},
			{k3, k1, k2},
			{k3, k2, k1},
		}

		roots := make([][]byte, len(orders))
		for i, order := range orders {
			mt := newTestingMerkle(t, 10)
			for _, key := range order {
				value := kvMap[key]
				err := mt.AddWord(key, value)
				assert.NoError(t, err)
			}
			roots[i] = mt.Root().Bytes()
		}

		for i := 1; i < len(roots); i++ {
			assert.Equal(t, roots[0], roots[i])
		}
	})

	t.Run("Add twice", func(t *testing.T) {
		keys := []*zkt.Byte32{k1, k2, k3}

		mt := newTestingMerkle(t, 10)
		for _, key := range keys {
			err := mt.AddWord(key, kvMap[key])
			assert.NoError(t, err)

			err = mt.AddWord(key, kvMap[key])
			assert.Equal(t, ErrEntryIndexAlreadyExists, err)
		}
	})
}

func TestZkTrieImpl_Update(t *testing.T) {
	k1 := zkt.NewByte32FromBytes([]byte{1})
	k2 := zkt.NewByte32FromBytes([]byte{2})
	k3 := zkt.NewByte32FromBytes([]byte{3})

	t.Run("Update 1", func(t *testing.T) {
		mt1 := newTestingMerkle(t, 10)
		err := mt1.AddWord(k1, zkt.NewByte32FromBytes([]byte{1}))
		assert.NoError(t, err)
		root1 := mt1.Root().Bytes()

		mt2 := newTestingMerkle(t, 10)
		err = mt2.AddWord(k1, zkt.NewByte32FromBytes([]byte{2}))
		assert.NoError(t, err)
		err = mt2.UpdateWord(k1, zkt.NewByte32FromBytes([]byte{1}))
		assert.NoError(t, err)
		root2 := mt2.Root().Bytes()

		assert.Equal(t, root1, root2)
	})

	t.Run("Update 2", func(t *testing.T) {
		mt1 := newTestingMerkle(t, 10)
		err := mt1.AddWord(k1, zkt.NewByte32FromBytes([]byte{1}))
		assert.NoError(t, err)
		err = mt1.AddWord(k2, zkt.NewByte32FromBytes([]byte{2}))
		assert.NoError(t, err)
		root1 := mt1.Root().Bytes()

		mt2 := newTestingMerkle(t, 10)
		err = mt2.AddWord(k1, zkt.NewByte32FromBytes([]byte{1}))
		assert.NoError(t, err)
		err = mt2.AddWord(k2, zkt.NewByte32FromBytes([]byte{3}))
		assert.NoError(t, err)
		err = mt2.UpdateWord(k2, zkt.NewByte32FromBytes([]byte{2}))
		assert.NoError(t, err)
		root2 := mt2.Root().Bytes()

		assert.Equal(t, root1, root2)
	})

	t.Run("Update 1, 2, 3", func(t *testing.T) {
		mt1 := newTestingMerkle(t, 10)
		mt2 := newTestingMerkle(t, 10)
		keys := []*zkt.Byte32{k1, k2, k3}
		for i, key := range keys {
			err := mt1.AddWord(key, zkt.NewByte32FromBytes([]byte{byte(i)}))
			assert.NoError(t, err)
		}
		for i, key := range keys {
			err := mt2.AddWord(key, zkt.NewByte32FromBytes([]byte{byte(i + 3)}))
			assert.NoError(t, err)
		}
		for i, key := range keys {
			err := mt1.UpdateWord(key, zkt.NewByte32FromBytes([]byte{byte(i + 6)}))
			assert.NoError(t, err)
			err = mt2.UpdateWord(key, zkt.NewByte32FromBytes([]byte{byte(i + 6)}))
			assert.NoError(t, err)
		}
		assert.Equal(t, mt1.Root().Bytes(), mt2.Root().Bytes())
	})

	t.Run("Update same value", func(t *testing.T) {
		mt := newTestingMerkle(t, 10)
		keys := []*zkt.Byte32{k1, k2, k3}
		for _, key := range keys {
			err := mt.AddWord(key, zkt.NewByte32FromBytes([]byte{1}))
			assert.NoError(t, err)
			err = mt.UpdateWord(key, zkt.NewByte32FromBytes([]byte{1}))
			assert.NoError(t, err)
			node, err := mt.GetLeafNodeByWord(key)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(node.ValuePreimage))
			assert.Equal(t, zkt.NewByte32FromBytes([]byte{1}).Bytes(), node.ValuePreimage[0][:])
		}
	})

	t.Run("Update non-existent word", func(t *testing.T) {
		mt := newTestingMerkle(t, 10)
		err := mt.UpdateWord(k1, zkt.NewByte32FromBytes([]byte{1}))
		assert.NoError(t, err)
		node, err := mt.GetLeafNodeByWord(k1)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(node.ValuePreimage))
		assert.Equal(t, zkt.NewByte32FromBytes([]byte{1}).Bytes(), node.ValuePreimage[0][:])
	})
}

func TestZkTrieImpl_Delete(t *testing.T) {
	k1 := zkt.NewByte32FromBytes([]byte{1})
	k2 := zkt.NewByte32FromBytes([]byte{2})
	k3 := zkt.NewByte32FromBytes([]byte{3})
	k4 := zkt.NewByte32FromBytes([]byte{4})

	t.Run("Test deletion leads to empty tree", func(t *testing.T) {
		emptyMT := newTestingMerkle(t, 10)

		mt1 := newTestingMerkle(t, 10)
		err := mt1.AddWord(k1, zkt.NewByte32FromBytes([]byte{1}))
		assert.NoError(t, err)
		err = mt1.DeleteWord(k1)
		assert.NoError(t, err)
		assert.Equal(t, zkt.HashZero.Bytes(), mt1.Root().Bytes())
		assert.Equal(t, emptyMT.Root().Bytes(), mt1.Root().Bytes())

		keys := []*zkt.Byte32{k1, k2, k3, k4}
		mt2 := newTestingMerkle(t, 10)
		for _, key := range keys {
			err := mt2.AddWord(key, zkt.NewByte32FromBytes([]byte{1}))
			assert.NoError(t, err)
		}
		for _, key := range keys {
			err := mt2.DeleteWord(key)
			assert.NoError(t, err)
		}
		assert.Equal(t, zkt.HashZero.Bytes(), mt2.Root().Bytes())
		assert.Equal(t, emptyMT.Root().Bytes(), mt2.Root().Bytes())

		mt3 := newTestingMerkle(t, 10)
		for _, key := range keys {
			err := mt3.AddWord(key, zkt.NewByte32FromBytes([]byte{1}))
			assert.NoError(t, err)
		}
		for i := len(keys) - 1; i >= 0; i-- {
			err := mt3.DeleteWord(keys[i])
			assert.NoError(t, err)
		}
		assert.Equal(t, zkt.HashZero.Bytes(), mt3.Root().Bytes())
		assert.Equal(t, emptyMT.Root().Bytes(), mt3.Root().Bytes())
	})

	t.Run("Test equivalent trees after deletion", func(t *testing.T) {
		keys := []*zkt.Byte32{k1, k2, k3, k4}

		mt1 := newTestingMerkle(t, 10)
		for i, key := range keys {
			err := mt1.AddWord(key, zkt.NewByte32FromBytes([]byte{byte(i + 1)}))
			assert.NoError(t, err)
		}
		err := mt1.DeleteWord(k1)
		assert.NoError(t, err)
		err = mt1.DeleteWord(k2)
		assert.NoError(t, err)

		mt2 := newTestingMerkle(t, 10)
		err = mt2.AddWord(k3, zkt.NewByte32FromBytes([]byte{byte(3)}))
		assert.NoError(t, err)
		err = mt2.AddWord(k4, zkt.NewByte32FromBytes([]byte{byte(4)}))
		assert.NoError(t, err)

		assert.Equal(t, mt1.Root().Bytes(), mt2.Root().Bytes())

		mt3 := newTestingMerkle(t, 10)
		for i, key := range keys {
			err := mt3.AddWord(key, zkt.NewByte32FromBytes([]byte{byte(i + 1)}))
			assert.NoError(t, err)
		}
		err = mt3.DeleteWord(k1)
		assert.NoError(t, err)
		err = mt3.DeleteWord(k3)
		assert.NoError(t, err)

		mt4 := newTestingMerkle(t, 10)
		err = mt4.AddWord(k2, zkt.NewByte32FromBytes([]byte{2}))
		assert.NoError(t, err)
		err = mt4.AddWord(k4, zkt.NewByte32FromBytes([]byte{4}))
		assert.NoError(t, err)

		assert.Equal(t, mt3.Root().Bytes(), mt4.Root().Bytes())
	})

	t.Run("Test repeat deletion", func(t *testing.T) {
		mt := newTestingMerkle(t, 10)
		err := mt.AddWord(k1, zkt.NewByte32FromBytes([]byte{1}))
		assert.NoError(t, err)
		err = mt.DeleteWord(k1)
		assert.NoError(t, err)
		err = mt.DeleteWord(k1)
		assert.Equal(t, ErrKeyNotFound, err)
	})

	t.Run("Test deletion of non-existent node", func(t *testing.T) {
		mt := newTestingMerkle(t, 10)
		err := mt.DeleteWord(k1)
		assert.Equal(t, ErrKeyNotFound, err)
	})
}

func TestMerkleTree_BuildAndVerifyZkTrieProof(t *testing.T) {
	zkTrie := newTestingMerkle(t, 10)

	testData := []struct {
		key   *big.Int
		value byte
	}{
		{big.NewInt(1), 2},
		{big.NewInt(3), 4},
		{big.NewInt(5), 6},
		{big.NewInt(7), 8},
		{big.NewInt(9), 10},
	}

	nonExistentKey := big.NewInt(11)

	getNode := func(hash *zkt.Hash) (*Node, error) {
		node, err := zkTrie.GetNode(hash)
		if err != nil {
			return nil, err
		}
		return node, nil
	}

	t.Run("Test with existent key", func(t *testing.T) {
		for _, td := range testData {
			err := zkTrie.AddWord(zkt.NewByte32FromBytes([]byte{byte(td.key.Int64())}), &zkt.Byte32{td.value})
			assert.NoError(t, err)

			node, err := zkTrie.GetLeafNodeByWord(zkt.NewByte32FromBytes([]byte{byte(td.key.Int64())}))
			assert.NoError(t, err)
			assert.Equal(t, 1, len(node.ValuePreimage))
			assert.Equal(t, (&zkt.Byte32{td.value})[:], node.ValuePreimage[0][:])

			proof, node, err := BuildZkTrieProof(zkTrie.rootHash, td.key, 10, getNode)
			assert.NoError(t, err)

			valid := VerifyProofZkTrie(zkTrie.rootHash, proof, node)
			assert.True(t, valid)
		}
	})

	t.Run("Test with non-existent key", func(t *testing.T) {
		proof, node, err := BuildZkTrieProof(zkTrie.rootHash, nonExistentKey, 10, getNode)
		assert.NoError(t, err)

		valid := VerifyProofZkTrie(zkTrie.rootHash, proof, node)
		assert.False(t, valid)

		hash, err := proof.Verify(node.nodeHash, node.NodeKey)
		assert.Error(t, err)
		assert.Nil(t, hash)
	})
}

func TestMerkleTree_GraphViz(t *testing.T) {
	mt := newTestingMerkle(t, 10)

	var buffer bytes.Buffer
	err := mt.GraphViz(&buffer, nil)
	assert.NoError(t, err)
	assert.Equal(t, "--------\nGraphViz of the ZkTrieImpl with RootHash 0\ndigraph hierarchy {\nnode [fontname=Monospace,fontsize=10,shape=box]\n}\nEnd of GraphViz of the ZkTrieImpl with RootHash 0\n--------\n", buffer.String())
	buffer.Reset()

	key1 := zkt.NewByte32FromBytes([]byte{1}) //0b1
	err = mt.AddWord(key1, &zkt.Byte32{1})
	assert.NoError(t, err)
	key2 := zkt.NewByte32FromBytes([]byte{3}) //0b11
	err = mt.AddWord(key2, &zkt.Byte32{3})
	assert.NoError(t, err)

	err = mt.GraphViz(&buffer, nil)
	assert.NoError(t, err)
	assert.Equal(t, "--------\nGraphViz of the ZkTrieImpl with RootHash 4467834053890953620178129130613022752584671477523987938903027600190138488269\ndigraph hierarchy {\nnode [fontname=Monospace,fontsize=10,shape=box]\n\"44678340...\" -> {\"empty0\" \"63478298...\"}\n\"empty0\" [style=dashed,label=0];\n\"63478298...\" -> {\"14984317...\" \"12008367...\"}\n\"14984317...\" [style=filled];\n\"12008367...\" [style=filled];\n}\nEnd of GraphViz of the ZkTrieImpl with RootHash 4467834053890953620178129130613022752584671477523987938903027600190138488269\n--------\n", buffer.String())
	buffer.Reset()
}
