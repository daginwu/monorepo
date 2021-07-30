package epaxosproto

import (
	"bufio"
	"bytes"
	"reflect"
	"testing"

	"github.com/daginwu/monorepo/libs/epaxos/src/state"
)

func TestPreAccept(t *testing.T) {
	m := &PreAccept{
		LeaderId:  7,
		Replica:   29,
		Instance:  3,
		Command:   []state.Command{state.Command{1, 7, 9}},
		Seq:       12,
		Deps:      [5]int32{1, 2, 3, 4, 5},
		SentAt:    56321,
		OpenAfter: 341,
	}
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	m.Marshal(w)
	w.Flush()
	unmarsh := &PreAccept{}
	r := bufio.NewReader(&buf)
	err := unmarsh.Unmarshal(r)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if !reflect.DeepEqual(m, unmarsh) {
		t.Fatalf("Expected %v, got %v", m, unmarsh)
	}
}
func TestAccept(t *testing.T) {
	m := &Accept{
		LeaderId: 7,
		Replica:  29,
		Instance: 3,
		Count:    5,
		Seq:      12,
		Deps:     [5]int32{1, 2, 3, 4, 5},
		SentAt:   56321,
	}
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	m.Marshal(w)
	w.Flush()
	unmarsh := &Accept{}
	r := bufio.NewReader(&buf)
	err := unmarsh.Unmarshal(r)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if !reflect.DeepEqual(m, unmarsh) {
		t.Fatalf("Expected %v, got %v", m, unmarsh)
	}
}

func TestCommit(t *testing.T) {
	m := &Commit{
		LeaderId: 7,
		Replica:  29,
		Instance: 3,
		Command:  []state.Command{state.Command{1, 7, 9}},
		Seq:      12,
		Deps:     [5]int32{1, 2, 3, 4, 5},
		SentAt:   56321,
	}
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	m.Marshal(w)
	w.Flush()
	unmarsh := &Commit{}
	r := bufio.NewReader(&buf)
	err := unmarsh.Unmarshal(r)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if !reflect.DeepEqual(m, unmarsh) {
		t.Fatalf("Expected %v, got %v", m, unmarsh)
	}
}

func TestCommitShort(t *testing.T) {
	m := &CommitShort{
		LeaderId: 7,
		Replica:  29,
		Instance: 3,
		Count:    5,
		Seq:      12,
		Deps:     [5]int32{1, 2, 3, 4, 5},
		SentAt:   56321,
	}
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	m.Marshal(w)
	w.Flush()
	unmarsh := &CommitShort{}
	r := bufio.NewReader(&buf)
	err := unmarsh.Unmarshal(r)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if !reflect.DeepEqual(m, unmarsh) {
		t.Fatalf("Expected %v, got %v", m, unmarsh)
	}
}
