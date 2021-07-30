package epaxos

import (
	"sort"
	"time"

	"github.com/daginwu/monorepo/libs/epaxos/src/epaxosproto"
	"github.com/daginwu/monorepo/libs/epaxos/src/genericsmrproto"
	"github.com/daginwu/monorepo/libs/epaxos/src/state"
)

const (
	WHITE int8 = iota
	GRAY
	BLACK
)

type Exec struct {
	r *Replica
}

type SCComponent struct {
	nodes []*Instance
	color int8
}

var overallRoot *Instance
var overallRep int32
var overallInst int32

func (e *Exec) executeCommand(replica int32, instance int32) bool {
	if e.r.InstanceSpace[replica][instance] == nil {
		return false
	}
	inst := e.r.InstanceSpace[replica][instance]
	if inst.Status == epaxosproto.EXECUTED {
		return true
	}
	if inst.Status != epaxosproto.COMMITTED {
		return false
	}

	overallRep = replica
	overallInst = instance
	overallRoot = inst

	if !e.findSCC(inst) {
		return false
	}

	return true
}

var stack []*Instance = make([]*Instance, 0, 100)

func (e *Exec) findSCC(root *Instance) bool {
	index := 1
	//find SCCs using Tarjan's algorithm
	stack = stack[0:0]
	return e.strongconnect(root, &index)
}

func (e *Exec) strongconnect(v *Instance, index *int) bool {
	v.Index = *index
	v.Lowlink = *index
	*index = *index + 1

	l := len(stack)
	if l == cap(stack) {
		newSlice := make([]*Instance, l, 2*l)
		copy(newSlice, stack)
		stack = newSlice
	}
	stack = stack[0 : l+1]
	stack[l] = v

	for q := int32(0); q < int32(e.r.N); q++ {
		inst := v.Deps[q]
		for i := e.r.ExecedUpTo[q] + 1; i <= inst; i++ {
			for e.r.InstanceSpace[q][i] == nil || e.r.InstanceSpace[q][i].Cmds == nil || v.Cmds == nil {
				// Sarah update: in the original code this was time.Sleep(1000 * 1000)
				return false
			}

			w := e.r.InstanceSpace[q][i]

			if w.Status == epaxosproto.EXECUTED {
				continue
			}

			// Instances that don't conflict can be skipped
			conflict := false
			for ci, _ := range v.Cmds {
				for di, _ := range w.Cmds {
					if state.Conflict(&v.Cmds[ci], &w.Cmds[di]) {
						conflict = true
					}
				}
			}
			if !conflict {
				continue
			}

			// Don't need to wait for reads
			allReads := true
			for _, cmd := range w.Cmds {
				if cmd.Op != state.GET {
					allReads = false
				}
			}
			if allReads {
				continue
			}

			// Livelock fix: any instance that has a high seq and the root
			// as a dependency will necessarily execute after it. (As will
			// any of its dependencies the root wouldn't already know about.)
			if e.r.infiniteFix &&
				(w.Seq > overallRoot.Seq || (overallRep < q && w.Seq == overallRoot.Seq)) &&
				w.Deps[overallRep] >= overallInst {
				break
			}

			for e.r.InstanceSpace[q][i].Status != epaxosproto.COMMITTED {
				// Sarah update: in the original code this was time.Sleep(1000 * 1000)
				return false
			}

			if w.Index == 0 {
				//e.strongconnect(w, index)
				if !e.strongconnect(w, index) {
					for j := l; j < len(stack); j++ {
						stack[j].Index = 0
					}
					stack = stack[0:l]
					return false
				}
				if w.Lowlink < v.Lowlink {
					v.Lowlink = w.Lowlink
				}
			} else { //if e.inStack(w)  //<- probably unnecessary condition, saves a linear search
				if w.Index < v.Lowlink {
					v.Lowlink = w.Index
				}
			}
		}
	}

	if v.Lowlink == v.Index {
		//found SCC
		list := stack[l:len(stack)]

		//execute commands in the increasing order of the Seq field
		sort.Sort(nodeArray(list))
		for _, w := range list {
			for w.Cmds == nil {
				// Sarah update: in the original code this was time.Sleep(1000 * 1000)
				return false
			}
			for idx := 0; idx < len(w.Cmds); idx++ {
				val := w.Cmds[idx].Execute(e.r.State)
				if !state.AllBlindWrites(w.Cmds) &&
					w.lb != nil && w.lb.clientProposals != nil {
					e.r.ReplyPropose(
						&genericsmrproto.ProposeReply{
							TRUE,
							w.lb.clientProposals[idx].CommandId,
							val,
							// w.lb.clientProposals[idx].Timestamp
							// Overload timestamp with time between commit and
							// execution
							time.Now().Sub(w.lb.commitTime).Nanoseconds()},
						w.lb.clientProposals[idx].Reply)
				}
			}
			w.Status = epaxosproto.EXECUTED
		}
		stack = stack[0:l]
	}
	return true
}

func (e *Exec) inStack(w *Instance) bool {
	for _, u := range stack {
		if w == u {
			return true
		}
	}
	return false
}

type nodeArray []*Instance

func (na nodeArray) Len() int {
	return len(na)
}

func (na nodeArray) Less(i, j int) bool {
	return na[i].Seq < na[j].Seq
}

func (na nodeArray) Swap(i, j int) {
	na[i], na[j] = na[j], na[i]
}
