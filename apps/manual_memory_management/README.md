# Manual Memory Management in Go

## Objective
This project shows how to **Manual Management Memory** by **using jemelloc in Go**.

## Concept
Go is a kind of language that implements **Garbage Collection**. In other words, in some **intensive memory** use cases **(eg: Database, Streaming, Cache)**. Go will lose control of memory. So we use Jemelloc to manually manage memory. In detail, we **keep data in byte format** to keep memory usage friendly.

## Implementation
### Install jemalloc
Download jemalloc by following the blog in reference.
``` bash
./configure --with-jemalloc-prefix='je_' --with-malloc-conf='background_thread:true,metadata_thp:auto'
make
sudo make install
```

### Declare byte as struct 
``` go
type mapEnteryOrigin struct {
	UID uint64
	val []byte
}

type mapEntery []byte
```
### Declare GET/SET functions with z.Buffer
``` go
// 8 bytes for UID other for val size
func mapEnterySize(valSz int) int {
	return 8 + valSz
}

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
```

### Run test
``` bash
go test -v .
```

### Example result
``` bash
...

    buffer_test.go:48: idx: 999994 uid 9999 len(val): 24
    buffer_test.go:48: idx: 999995 uid 9999 len(val): 39
    buffer_test.go:48: idx: 999996 uid 9999 len(val): 61
    buffer_test.go:48: idx: 999997 uid 9999 len(val): 8
    buffer_test.go:48: idx: 999998 uid 9999 len(val): 55
    buffer_test.go:48: idx: 999999 uid 9999 len(val): 20
    buffer_test.go:63: DONE
         Calloc 0 B Go HeapAlloc 432 MiB
PASS
ok  	github.com/daginwu/monorepo/apps/manual_memory_management	9.112s
```


## Reference
[High Performance Manual Memory Management in Go | Manish Jain | Go Systems Conf SF 2020](https://www.youtube.com/watch?v=F_3d4oa_1lU&t=)

[Manual Memory Management in Go using jemalloc](https://dgraph.io/blog/post/manual-memory-management-golang-jemalloc/)
