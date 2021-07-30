package genericsmrproto

import (
	"bufio"
	"bytes"
	"reflect"
	"testing"
)

func TestMetricsRequest(t *testing.T) {
	mr := &MetricsRequest{5}
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	mr.Marshal(w)
	w.Flush()
	unmarsh := &MetricsRequest{}
	r := bufio.NewReader(&buf)
	err := unmarsh.Unmarshal(r)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if !reflect.DeepEqual(mr, unmarsh) {
		t.Fatalf("Expected %v, got %v", mr, unmarsh)
	}
}

func TestMetricsReply(t *testing.T) {
	mr := &MetricsReply{17, 82}
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	mr.Marshal(w)
	w.Flush()
	unmarsh := &MetricsReply{}
	r := bufio.NewReader(&buf)
	err := unmarsh.Unmarshal(r)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if !reflect.DeepEqual(mr, unmarsh) {
		t.Fatalf("Expected %v, got %v", mr, unmarsh)
	}
}
