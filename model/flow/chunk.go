package flow

// ChunkBody - body section of a chunk
type ChunkBody struct {

	// the ID of the collection this chunk corresponds to
	CollectionIndex uint

	// execution info
	StartState      StateCommitment // start state when starting executing this chunk
	EventCollection Identifier      // Events generated by executing results

	// Computation consumption info
	TotalComputationUsed            uint64 // total amount of computation used by running all txs in this chunk
	FirstTransactionComputationUsed uint64 // first tx in this chunk computation usage
}

// Chunk is an aggregate execution info about a sequence of transactions
type Chunk struct {
	ChunkBody

	Index uint64 // chunk index inside the ER (starts from zero)
	// EndState inferred from next chunk or from the ER
	EndState StateCommitment
}

// ID returns a unique id for this entity
func (ch *Chunk) ID() Identifier {
	return MakeID(ch.ChunkBody)
}

// Checksum provides a cryptographic commitment for a chunk content
func (ch *Chunk) Checksum() Identifier {
	return MakeID(ch)
}

// Note that this is the basic version of the List, we need to substitute it with something like Merkel tree at some point
type ChunkList struct {
	Chunks []*Chunk
}

func (cl *ChunkList) Fingerprint() Identifier {
	return MerkleRoot(GetIDs(cl)...)
}

func (cl *ChunkList) Insert(ch *Chunk) {
	cl.Chunks = append(cl.Chunks, ch)
}

func (cl *ChunkList) Items() []*Chunk {
	return cl.Chunks
}

// ByChecksum returns an entity from the list by entity fingerprint
func (cl *ChunkList) ByChecksum(cs Identifier) (*Chunk, bool) {
	for _, ch := range cl.Chunks {
		if ch.Checksum() == cs {
			return ch, true
		}
	}
	return nil, false
}

// ByIndex returns an entity from the list by index
func (cl *ChunkList) ByIndex(i uint64) *Chunk {
	return cl.Chunks[i]
}

// ByIndexWithProof returns an entity from the list by index and proof of membership
func (cl *ChunkList) ByIndexWithProof(i uint64) (*Chunk, Proof) {
	return cl.Chunks[i], nil
}

// Size returns the number of Chunks in the list
func (cl *ChunkList) Size() int {
	return len(cl.Chunks)
}
