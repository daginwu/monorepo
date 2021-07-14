package main

import (
	"encoding/binary"
	"log"
	"math/rand"
	"runtime"
	"testing"

	"github.com/dgraph-io/ristretto/z"
	"github.com/dustin/go-humanize"
)

type mapEnteryOrigin struct {
	UID uint64
	val []byte
}

type mapEntery []byte

// 8 bytes for UID other for val size
func mapEnterySize(valSz int) int {
	return 8 + valSz
}

// Set
func mapEnteryMarshal(dst []byte, uid uint64, val []byte) {
	binary.BigEndian.PutUint64(dst[:8], uid)
	copy(dst[8:], val)
}

// Get the value we store in Memory
func (me mapEntery) UID() uint64 {
	return binary.BigEndian.Uint64(me[:8])
}

func (me mapEntery) Val() []byte {
	return me[8:]
}

func TestBuffer(t *testing.T) {

	// Setup z.Buffer initial
	buf := z.NewBuffer(1024, "MapEnteryTest")
	// Setup MapEntery Number for this test case
	N := 1000000

	printBuffer := func() {
		idx := 0

		// Iterate through the buffer
		err := buf.SliceIterate(func(s []byte) error {
			me := mapEntery(s)
			t.Logf(
				"idx: %d uid %d len(val): %d \n",
				idx,
				me.UID(),
				len(me.Val()),
			)
			idx++
			return nil
		})
		if err != nil {
			log.Fatal(err)
		}

		// Print Memory usage
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		t.Logf("DONE\n Calloc %s Go HeapAlloc %s",
			humanize.IBytes(uint64(z.NumAllocBytes())),
			humanize.IBytes(ms.HeapAlloc),
		)
	}

	// Make null buffer
	tmp := make([]byte, 1024)

	// Generate N MapEntery
	for i := 0; i < N; i++ {

		// Generate MapEntery Value's SIZE VALUE
		valSz := rand.Intn(64)
		// Generate MapEntery Value's VALUE
		rand.Read(tmp[:valSz])

		// Allocate memory space for THIS MapEntery Tuple
		dst := buf.SliceAllocate(mapEnterySize(valSz))

		// Store to Dgraph's z.buffer
		mapEnteryMarshal(
			dst,
			uint64(rand.Int63n(10000)),
			tmp[:valSz],
		)

	}

	// Sort buffer by UID
	buf.SortSlice(func(left, right []byte) bool {
		lme := mapEntery(left)
		rme := mapEntery(right)
		return lme.UID() < rme.UID()
	})

	printBuffer()

}
