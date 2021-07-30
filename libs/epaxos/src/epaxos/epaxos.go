package epaxos

import (
	"encoding/binary"
	"io"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/daginwu/monorepo/libs/epaxos/src/epaxosproto"
	"github.com/daginwu/monorepo/libs/epaxos/src/genericsmr"
	"github.com/daginwu/monorepo/libs/epaxos/src/genericsmrproto"
	"github.com/daginwu/monorepo/libs/epaxos/src/priorityqueue"
	"github.com/daginwu/monorepo/libs/epaxos/src/state"
	"github.com/daginwu/monorepo/libs/epaxos/src/timetrace"
)

const MAX_DEPTH_DEP = 10
const TRUE = uint8(1)
const FALSE = uint8(0)
const DS = 5
const ADAPT_TIME_SEC = 10

const MAX_BATCH = 1000

const COMMIT_GRACE_PERIOD = 10 * 1e9 //10 seconds

const DO_CHECKPOINTING = false
const HT_INIT_SIZE = 200000
const CHECKPOINT_PERIOD = 10000

var cpMarker []state.Command
var cpcounter = 0

type ClockSyncType uint8

const (
	CLOCK_SYNC_NONE   = 0
	CLOCK_SYNC_QUORUM = 1
	CLOCK_SYNC_ALL    = 2
	CLOCK_SYNC_SUPERQ = 3
)

type Replica struct {
	*genericsmr.Replica
	prepareChan           chan *genericsmr.RPCMessage
	preAcceptChan         chan *genericsmr.RPCMessage
	acceptChan            chan *genericsmr.RPCMessage
	commitChan            chan *genericsmr.RPCMessage
	commitShortChan       chan *genericsmr.RPCMessage
	prepareReplyChan      chan *genericsmr.RPCMessage
	preAcceptReplyChan    chan *genericsmr.RPCMessage
	preAcceptOKChan       chan *genericsmr.RPCMessage
	acceptReplyChan       chan *genericsmr.RPCMessage
	tryPreAcceptChan      chan *genericsmr.RPCMessage
	tryPreAcceptReplyChan chan *genericsmr.RPCMessage
	prepareRPC            uint8
	prepareReplyRPC       uint8
	preAcceptRPC          uint8
	preAcceptReplyRPC     uint8
	preAcceptOKRPC        uint8
	acceptRPC             uint8
	acceptReplyRPC        uint8
	commitRPC             uint8
	commitShortRPC        uint8
	tryPreAcceptRPC       uint8
	tryPreAcceptReplyRPC  uint8
	InstanceSpace         [][]*Instance // the space of all instances (used and not yet used)
	crtInstance           []int32       // highest active instance numbers that this replica knows about
	// the highest instance number that this replica has sent out messages for
	// if clock sync is enabled, this could be higher than crtInstance.
	// crtInstance will refer to the instances that have been processed
	proposeInstno      int32
	CommittedUpTo      [DS]int32 // highest committed instance per replica that this replica knows about
	ExecedUpTo         []int32   // instance up to which all commands have been executed (including iteslf)
	exec               *Exec
	conflicts          []map[state.Key]int32
	maxSeqPerKey       map[state.Key]int32
	maxSeq             int32
	latestCPReplica    int32
	latestCPInstance   int32
	clientMutex        *sync.Mutex // for synchronizing when sending replies to clients from multiple go-routines
	instancesToRecover chan *instanceId
	batchingEnabled    bool
	infiniteFix        bool
	kFast              int64
	kSlow              int64
	clockSyncType      ClockSyncType
	oneWayDelayEwma    [DS]float64 // Estimated weighted moving averages of the one-way delays to each other replica
	clockSyncEpsilon   int64       // The number of nanoseconds to add to each open after time
	clockSyncTimetrace *timetrace.Buffer
	paOrderTimetrace   *timetrace.Buffer
	clockSyncPQ        *priorityqueue.PriorityQueue
	// Keeps track of proposals for instances that haven't yet been processed
	// when clock sync is enabled
	pendingProposals map[int32][]*genericsmr.Propose
}

type Instance struct {
	Cmds           []state.Command
	ballot         int32
	Status         int8
	Seq            int32
	Deps           [DS]int32
	lb             *LeaderBookkeeping
	Index, Lowlink int
}

type instanceId struct {
	replica  int32
	instance int32
}

type RecoveryInstance struct {
	cmds            []state.Command
	status          int8
	seq             int32
	deps            [DS]int32
	preAcceptCount  int
	leaderResponded bool
}

type LeaderBookkeeping struct {
	clientProposals   []*genericsmr.Propose
	maxRecvBallot     int32
	prepareOKs        int
	allEqual          bool
	preAcceptOKs      int
	acceptOKs         int
	nacks             int
	originalDeps      [DS]int32
	committedDeps     []int32
	recoveryInst      *RecoveryInstance
	preparing         bool
	tryingToPreAccept bool
	possibleQuorum    []bool
	tpaOKs            int
	commitTime        time.Time
}

func NewReplica(id int, peerAddrList []string, thrifty bool, beacon bool,
	durable bool, batch bool, infiniteFix bool, clockSyncType ClockSyncType,
	clockSyncEpsilon int64) *Replica {
	r := &Replica{
		genericsmr.NewReplica(id, peerAddrList, thrifty),
		make(chan *genericsmr.RPCMessage, genericsmr.CHAN_BUFFER_SIZE),
		make(chan *genericsmr.RPCMessage, genericsmr.CHAN_BUFFER_SIZE),
		make(chan *genericsmr.RPCMessage, genericsmr.CHAN_BUFFER_SIZE),
		make(chan *genericsmr.RPCMessage, genericsmr.CHAN_BUFFER_SIZE),
		make(chan *genericsmr.RPCMessage, genericsmr.CHAN_BUFFER_SIZE),
		make(chan *genericsmr.RPCMessage, genericsmr.CHAN_BUFFER_SIZE),
		make(chan *genericsmr.RPCMessage, genericsmr.CHAN_BUFFER_SIZE*3),
		make(chan *genericsmr.RPCMessage, genericsmr.CHAN_BUFFER_SIZE*3),
		make(chan *genericsmr.RPCMessage, genericsmr.CHAN_BUFFER_SIZE*2),
		make(chan *genericsmr.RPCMessage, genericsmr.CHAN_BUFFER_SIZE),
		make(chan *genericsmr.RPCMessage, genericsmr.CHAN_BUFFER_SIZE),
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		make([][]*Instance, len(peerAddrList)),
		make([]int32, len(peerAddrList)),
		0, // proposeInstno
		[DS]int32{-1, -1, -1, -1, -1},
		make([]int32, len(peerAddrList)),
		nil,
		make([]map[state.Key]int32, len(peerAddrList)),
		make(map[state.Key]int32),
		0,
		0,
		-1,
		new(sync.Mutex),
		make(chan *instanceId, genericsmr.CHAN_BUFFER_SIZE),
		batch,
		infiniteFix,
		0, // kFast
		0, // kSlow
		clockSyncType,
		[DS]float64{}, // clockSyncEwma
		clockSyncEpsilon,
		timetrace.NewBuffer(),            // clockSyncTimetrace
		timetrace.NewBuffer(),            // paOrderTimetrace
		priorityqueue.NewPriorityQueue(), // clockSyncPQ
		make(map[int32][]*genericsmr.Propose, 1000), // pendingProposals
	}

	r.Beacon = beacon
	r.Durable = durable

	for i := 0; i < r.N; i++ {
		r.InstanceSpace[i] = make([]*Instance, 2*1024*1024)
		r.crtInstance[i] = 0
		r.ExecedUpTo[i] = -1
		r.conflicts[i] = make(map[state.Key]int32, HT_INIT_SIZE)
	}

	r.exec = &Exec{r}

	cpMarker = make([]state.Command, 0)

	//register RPCs
	r.prepareRPC = r.RegisterRPC(new(epaxosproto.Prepare), r.prepareChan)
	r.prepareReplyRPC = r.RegisterRPC(new(epaxosproto.PrepareReply), r.prepareReplyChan)
	r.preAcceptRPC = r.RegisterRPC(new(epaxosproto.PreAccept), r.preAcceptChan)
	r.preAcceptReplyRPC = r.RegisterRPC(new(epaxosproto.PreAcceptReply), r.preAcceptReplyChan)
	r.preAcceptOKRPC = r.RegisterRPC(new(epaxosproto.PreAcceptOK), r.preAcceptOKChan)
	r.acceptRPC = r.RegisterRPC(new(epaxosproto.Accept), r.acceptChan)
	r.acceptReplyRPC = r.RegisterRPC(new(epaxosproto.AcceptReply), r.acceptReplyChan)
	r.commitRPC = r.RegisterRPC(new(epaxosproto.Commit), r.commitChan)
	r.commitShortRPC = r.RegisterRPC(new(epaxosproto.CommitShort), r.commitShortChan)
	r.tryPreAcceptRPC = r.RegisterRPC(new(epaxosproto.TryPreAccept), r.tryPreAcceptChan)
	r.tryPreAcceptReplyRPC = r.RegisterRPC(new(epaxosproto.TryPreAcceptReply), r.tryPreAcceptReplyChan)

	go r.run()

	return r
}

func NewLeaderBookkeeping(proposals []*genericsmr.Propose, deps [DS]int32) *LeaderBookkeeping {
	return &LeaderBookkeeping{proposals, 0, 0, true, 0, 0, 0, deps,
		[]int32{-1, -1, -1, -1, -1}, nil, false, false, nil, 0, time.Time{}}
}

//append a log entry to stable storage
func (r *Replica) recordInstanceMetadata(inst *Instance) {
	if !r.Durable {
		return
	}

	var b [9 + DS*4]byte
	binary.LittleEndian.PutUint32(b[0:4], uint32(inst.ballot))
	b[4] = byte(inst.Status)
	binary.LittleEndian.PutUint32(b[5:9], uint32(inst.Seq))
	l := 9
	for _, dep := range inst.Deps {
		binary.LittleEndian.PutUint32(b[l:l+4], uint32(dep))
		l += 4
	}
	r.StableStore.Write(b[:])
}

//write a sequence of commands to stable storage
func (r *Replica) recordCommands(cmds []state.Command) {
	if !r.Durable {
		return
	}

	if cmds == nil {
		return
	}
	for i := 0; i < len(cmds); i++ {
		cmds[i].Marshal(io.Writer(r.StableStore))
	}
}

//sync with the stable store
func (r *Replica) sync() {
	if !r.Durable {
		return
	}

	r.StableStore.Sync()
}

/* Clock goroutine */

var fastClockChan chan bool
var slowClockChan chan bool

func (r *Replica) fastClock() {
	for !r.Shutdown {
		time.Sleep(5 * 1e6) // 5 ms
		fastClockChan <- true
	}
}
func (r *Replica) slowClock() {
	for !r.Shutdown {
		time.Sleep(150 * 1e6) // 150 ms
		slowClockChan <- true
	}
}

func (r *Replica) stopAdapting() {
	time.Sleep(1000 * 1000 * 1000 * ADAPT_TIME_SEC)
	r.Beacon = false
	time.Sleep(1000 * 1000 * 1000)

	for i := 0; i < r.N-1; i++ {
		min := i
		for j := i + 1; j < r.N-1; j++ {
			if r.Ewma[r.PreferredPeerOrder[j]] < r.Ewma[r.PreferredPeerOrder[min]] {
				min = j
			}
		}
		aux := r.PreferredPeerOrder[i]
		r.PreferredPeerOrder[i] = r.PreferredPeerOrder[min]
		r.PreferredPeerOrder[min] = aux
	}

	log.Println(r.PreferredPeerOrder)
}

/* ============= */

/***********************************
   Main event processing loop      *
************************************/

func (r *Replica) run() {
	r.ConnectToPeers()

	go r.WaitForClientConnections()

	go r.executeCommands()

	if r.Id == 0 {
		//init quorum read lease
		quorum := make([]int32, r.N/2+1)
		for i := 0; i <= r.N/2; i++ {
			quorum[i] = int32(i)
		}
		r.UpdatePreferredPeerOrder(quorum)
	}

	slowClockChan = make(chan bool, 1)
	fastClockChan = make(chan bool, 1)
	go r.slowClock()

	//Enabled when batching for 5ms
	if r.batchingEnabled && MAX_BATCH > 100 {
		go r.fastClock()
	}

	if r.Beacon {
		go r.stopAdapting()
	}

	onOffProposeChan := r.ProposeChan

	for !r.Shutdown {

		if r.clockSyncType != CLOCK_SYNC_NONE {
			peakPriority := r.clockSyncPQ.PeakPriority()
			currTime := genericsmr.CurrentTime()
			if peakPriority != priorityqueue.PRI_INVALID &&
				currTime >= peakPriority {
				pa := r.clockSyncPQ.Pop().(*epaxosproto.PreAccept)
				r.handlePreAccept(pa)
			}
		}

		select {

		case propose := <-onOffProposeChan:
			//got a Propose from a client
			r.handlePropose(propose)
			//deactivate new proposals channel to prioritize the handling of other protocol messages,
			//and to allow commands to accumulate for batching
			if r.batchingEnabled {
				onOffProposeChan = nil
			}
			break

		case <-fastClockChan:
			//activate new proposals channel
			onOffProposeChan = r.ProposeChan
			break

		case prepareS := <-r.prepareChan:
			prepare := prepareS.Message.(*epaxosproto.Prepare)
			//got a Prepare message
			r.handlePrepare(prepare)
			break

		case preAcceptS := <-r.preAcceptChan:
			preAccept := preAcceptS.Message.(*epaxosproto.PreAccept)
			r.updateOneWayDelay(preAccept.SentAt, preAcceptS)
			//got a PreAccept message
			if r.clockSyncType == CLOCK_SYNC_NONE {
				r.handlePreAccept(preAccept)
			} else {
				r.clockSyncPQ.Push(preAccept, preAccept.OpenAfter)
			}
			break

		case acceptS := <-r.acceptChan:
			accept := acceptS.Message.(*epaxosproto.Accept)
			r.updateOneWayDelay(accept.SentAt, acceptS)
			//got an Accept message
			r.handleAccept(accept)
			break

		case commitS := <-r.commitChan:
			commit := commitS.Message.(*epaxosproto.Commit)
			r.updateOneWayDelay(commit.SentAt, commitS)
			//got a Commit message
			r.handleCommit(commit)
			break

		case commitS := <-r.commitShortChan:
			commit := commitS.Message.(*epaxosproto.CommitShort)
			r.updateOneWayDelay(commit.SentAt, commitS)
			//got a Commit message
			r.handleCommitShort(commit)
			break

		case prepareReplyS := <-r.prepareReplyChan:
			prepareReply := prepareReplyS.Message.(*epaxosproto.PrepareReply)
			//got a Prepare reply
			r.handlePrepareReply(prepareReply)
			break

		case preAcceptReplyS := <-r.preAcceptReplyChan:
			preAcceptReply := preAcceptReplyS.Message.(*epaxosproto.PreAcceptReply)
			//got a PreAccept reply
			if r.InstanceSpace[preAcceptReply.Replica][preAcceptReply.Instance] != nil {
				r.handlePreAcceptReply(preAcceptReply)
			} else {
				// This could happen if clock sync is enabled and we haven't yet
				// processed our own preAccept
				r.preAcceptReplyChan <- preAcceptReplyS
			}

			break

		case preAcceptOKS := <-r.preAcceptOKChan:
			preAcceptOK := preAcceptOKS.Message.(*epaxosproto.PreAcceptOK)
			//got a PreAccept reply
			if r.InstanceSpace[r.Id][preAcceptOK.Instance] != nil {
				r.handlePreAcceptOK(preAcceptOK)
			} else {
				// This could happen if clock sync is enabled and we haven't yet
				// processed our own preAccept
				r.preAcceptOKChan <- preAcceptOKS
			}
			break

		case acceptReplyS := <-r.acceptReplyChan:
			acceptReply := acceptReplyS.Message.(*epaxosproto.AcceptReply)
			//got an Accept reply
			r.handleAcceptReply(acceptReply)
			break

		case tryPreAcceptS := <-r.tryPreAcceptChan:
			tryPreAccept := tryPreAcceptS.Message.(*epaxosproto.TryPreAccept)
			r.handleTryPreAccept(tryPreAccept)
			break

		case tryPreAcceptReplyS := <-r.tryPreAcceptReplyChan:
			tryPreAcceptReply := tryPreAcceptReplyS.Message.(*epaxosproto.TryPreAcceptReply)
			r.handleTryPreAcceptReply(tryPreAcceptReply)
			break

		case beacon := <-r.BeaconChan:
			r.ReplyBeacon(beacon)
			break

		case <-slowClockChan:
			if r.Beacon {
				for q := int32(0); q < int32(r.N); q++ {
					if q == r.Id {
						continue
					}
					r.SendBeacon(q)
				}
			}
			break
		case <-r.OnClientConnect:
			break

		case iid := <-r.instancesToRecover:
			r.startRecoveryForInstance(iid.replica, iid.instance)
			break

		case metricsRequest := <-r.MetricsChan:
			log.Println("Received metrics request")

			reply := &genericsmrproto.MetricsReply{}

			switch metricsRequest.OpCode {
			case genericsmrproto.METRICSOP_CONFLICT_RATE:
				log.Println("Responding to metrics request")
				reply.KFast = r.kFast
				reply.KSlow = r.kSlow
				break
			case genericsmrproto.METRICSOP_DUMP_OWD:
				r.clockSyncTimetrace.DumpTrace("owds.txt")
				r.paOrderTimetrace.DumpTrace("pa-order.txt")
				break
			}

			reply.Marshal(metricsRequest.Reply)
			metricsRequest.Reply.Flush()
			break

		default:
			break
		}
	}
}

/***********************************
   Command execution thread        *
************************************/

func (r *Replica) executeCommands() {
	const SLEEP_TIME_NS = 1e6
	problemInstance := make([]int32, r.N)
	timeout := make([]uint64, r.N)
	for q := 0; q < r.N; q++ {
		problemInstance[q] = -1
		timeout[q] = 0
	}

	for !r.Shutdown {
		executed := false
		for q := 0; q < r.N; q++ {
			inst := int32(0)
			for inst = r.ExecedUpTo[q] + 1; inst < r.crtInstance[q]; inst++ {
				if r.InstanceSpace[q][inst] != nil && r.InstanceSpace[q][inst].Status == epaxosproto.EXECUTED {
					if inst == r.ExecedUpTo[q]+1 {
						r.ExecedUpTo[q] = inst
					}
					continue
				}
				if r.InstanceSpace[q][inst] == nil || r.InstanceSpace[q][inst].Status != epaxosproto.COMMITTED {
					if inst == problemInstance[q] {
						timeout[q] += SLEEP_TIME_NS
						if timeout[q] >= COMMIT_GRACE_PERIOD {
							r.instancesToRecover <- &instanceId{int32(q), inst}
							timeout[q] = 0
						}
					} else {
						problemInstance[q] = inst
						timeout[q] = 0
					}
					if r.InstanceSpace[q][inst] == nil {
						continue
					}
					// Sarah update: in the original code, this was break
					continue
				}
				if ok := r.exec.executeCommand(int32(q), inst); ok {
					executed = true
					if inst == r.ExecedUpTo[q]+1 {
						r.ExecedUpTo[q] = inst
					}
				}
			}
		}
		if !executed {
			time.Sleep(SLEEP_TIME_NS)
		}
	}
}

/* Ballot helper functions */

func (r *Replica) makeUniqueBallot(ballot int32) int32 {
	return (ballot << 4) | r.Id
}

func (r *Replica) makeBallotLargerThan(ballot int32) int32 {
	return r.makeUniqueBallot((ballot >> 4) + 1)
}

func isInitialBallot(ballot int32) bool {
	return (ballot >> 4) == 0
}

func replicaIdFromBallot(ballot int32) int32 {
	return ballot & 15
}

/**********************************************************************
                       clock synchronization
***********************************************************************/

var owdMsg = "Rid: %d, Owd: %d, Ewma: %d."

func (r *Replica) updateOneWayDelay(sentAt int64, m *genericsmr.RPCMessage) {
	rid := m.From
	owd := m.ReceivedAt - sentAt
	if r.oneWayDelayEwma[rid] == 0 {
		r.oneWayDelayEwma[rid] = float64(owd)
	} else {
		r.oneWayDelayEwma[rid] = 0.99*float64(r.oneWayDelayEwma[rid]) + 0.01*float64(owd)
	}

	r.clockSyncTimetrace.Record3(&owdMsg, int64(rid), owd, int64(r.oneWayDelayEwma[rid]))
}

func (r *Replica) computeOpenAfter() int64 {
	switch r.clockSyncType {
	case CLOCK_SYNC_QUORUM:
		quorumSize := r.N/2 + 1

		ewmas := make([]float64, 0, DS-1)
		for i, delay := range r.oneWayDelayEwma {
			if int32(i) == r.Id {
				continue
			}

			ewmas = append(ewmas, delay)
		}

		// sort and find furthest in quorum
		sort.Float64s(ewmas)
		// -1 for index and -1 for self
		return genericsmr.CurrentTime() + int64(ewmas[quorumSize-2]) + r.clockSyncEpsilon
	case CLOCK_SYNC_ALL:
		var max float64 = 0
		for i, delay := range r.oneWayDelayEwma {
			if int32(i) == r.Id {
				continue
			}

			if delay > max {
				max = delay
			}
		}
		return genericsmr.CurrentTime() + int64(max) + r.clockSyncEpsilon
	case CLOCK_SYNC_SUPERQ:
		var max float64 = 0
		for i, delay := range r.oneWayDelayEwma {
			if int32(i) == r.Id {
				continue
			}

			// Sync to CA, VA, and OR
			if i != 0 && i != 1 && i != 3 {
				continue
			}

			if delay > max {
				max = delay
			}
		}
		return genericsmr.CurrentTime() + int64(max) + r.clockSyncEpsilon
	default:
		return genericsmr.CurrentTime()
	}
}

/**********************************************************************
                    inter-replica communication
***********************************************************************/

func (r *Replica) replyPrepare(replicaId int32, reply *epaxosproto.PrepareReply) {
	r.SendMsg(replicaId, r.prepareReplyRPC, reply)
}

func (r *Replica) replyPreAccept(replicaId int32, reply *epaxosproto.PreAcceptReply) {
	r.SendMsg(replicaId, r.preAcceptReplyRPC, reply)
}

func (r *Replica) replyAccept(replicaId int32, reply *epaxosproto.AcceptReply) {
	r.SendMsg(replicaId, r.acceptReplyRPC, reply)
}

func (r *Replica) replyTryPreAccept(replicaId int32, reply *epaxosproto.TryPreAcceptReply) {
	r.SendMsg(replicaId, r.tryPreAcceptReplyRPC, reply)
}

func (r *Replica) bcastPrepare(replica int32, instance int32, ballot int32) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("Prepare bcast failed:", err)
		}
	}()
	args := &epaxosproto.Prepare{r.Id, replica, instance, ballot}

	n := r.N - 1
	if r.Thrifty {
		n = r.N / 2
	}
	q := r.Id
	for sent := 0; sent < n; {
		q = (q + 1) % int32(r.N)
		if q == r.Id {
			log.Println("Not enough replicas alive!")
			break
		}
		if !r.Alive[q] {
			continue
		}
		r.SendMsg(q, r.prepareRPC, args)
		sent++
	}
}

func (r *Replica) bcastPreAccept(replica int32, instance int32, ballot int32, cmds []state.Command, seq int32, deps [DS]int32) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("PreAccept bcast failed:", err)
		}
	}()

	pa := &epaxosproto.PreAccept{}
	pa.LeaderId = r.Id
	pa.Replica = replica
	pa.Instance = instance
	pa.Ballot = ballot
	pa.Command = cmds
	pa.Seq = seq
	pa.Deps = deps
	pa.OpenAfter = r.computeOpenAfter()

	n := r.N - 1
	if r.Thrifty {
		n = r.N / 2
	}

	sent := 0
	for q := 0; q < r.N-1; q++ {
		if !r.Alive[r.PreferredPeerOrder[q]] {
			continue
		}
		pa.SentAt = genericsmr.CurrentTime()
		r.SendMsg(r.PreferredPeerOrder[q], r.preAcceptRPC, pa)
		sent++
		if sent >= n {
			break
		}
	}

	if r.clockSyncType != CLOCK_SYNC_NONE {
		r.clockSyncPQ.Push(pa, pa.OpenAfter)
	}
}

var tpa epaxosproto.TryPreAccept

func (r *Replica) bcastTryPreAccept(replica int32, instance int32, ballot int32, cmds []state.Command, seq int32, deps [DS]int32) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("PreAccept bcast failed:", err)
		}
	}()
	tpa.LeaderId = r.Id
	tpa.Replica = replica
	tpa.Instance = instance
	tpa.Ballot = ballot
	tpa.Command = cmds
	tpa.Seq = seq
	tpa.Deps = deps
	args := &tpa

	for q := int32(0); q < int32(r.N); q++ {
		if q == r.Id {
			continue
		}
		if !r.Alive[q] {
			continue
		}
		r.SendMsg(q, r.tryPreAcceptRPC, args)
	}
}

var ea epaxosproto.Accept

func (r *Replica) bcastAccept(replica int32, instance int32, ballot int32, count int32, seq int32, deps [DS]int32) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("Accept bcast failed:", err)
		}
	}()

	ea.LeaderId = r.Id
	ea.Replica = replica
	ea.Instance = instance
	ea.Ballot = ballot
	ea.Count = count
	ea.Seq = seq
	ea.Deps = deps
	args := &ea

	n := r.N - 1
	if r.Thrifty {
		n = r.N / 2
	}

	sent := 0
	for q := 0; q < r.N-1; q++ {
		if !r.Alive[r.PreferredPeerOrder[q]] {
			continue
		}
		args.SentAt = genericsmr.CurrentTime()
		r.SendMsg(r.PreferredPeerOrder[q], r.acceptRPC, args)
		sent++
		if sent >= n {
			break
		}
	}
}

var ec epaxosproto.Commit
var ecs epaxosproto.CommitShort

func (r *Replica) bcastCommit(replica int32, instance int32, cmds []state.Command, seq int32, deps [DS]int32) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("Commit bcast failed:", err)
		}
	}()
	ec.LeaderId = r.Id
	ec.Replica = replica
	ec.Instance = instance
	ec.Command = cmds
	ec.Seq = seq
	ec.Deps = deps
	args := &ec
	ecs.LeaderId = r.Id
	ecs.Replica = replica
	ecs.Instance = instance
	ecs.Count = int32(len(cmds))
	ecs.Seq = seq
	ecs.Deps = deps
	argsShort := &ecs

	r.InstanceSpace[replica][instance].lb.commitTime = time.Now()

	sent := 0
	for q := 0; q < r.N-1; q++ {
		if !r.Alive[r.PreferredPeerOrder[q]] {
			continue
		}
		// Sarah update: this used to be && sent >= r.N/2, but lost correctness
		// if the preferred peer order changed since first sending messages for
		// this instance
		if r.Thrifty {
			args.SentAt = genericsmr.CurrentTime()
			r.SendMsg(r.PreferredPeerOrder[q], r.commitRPC, args)
		} else {
			argsShort.SentAt = genericsmr.CurrentTime()
			r.SendMsg(r.PreferredPeerOrder[q], r.commitShortRPC, argsShort)
			sent++
		}
	}
}

/******************************************************************
               Helper functions
*******************************************************************/

func (r *Replica) clearHashtables() {
	for q := 0; q < r.N; q++ {
		r.conflicts[q] = make(map[state.Key]int32, HT_INIT_SIZE)
	}
}

func (r *Replica) updateCommitted(replica int32) {
	for r.InstanceSpace[replica][r.CommittedUpTo[replica]+1] != nil &&
		(r.InstanceSpace[replica][r.CommittedUpTo[replica]+1].Status == epaxosproto.COMMITTED ||
			r.InstanceSpace[replica][r.CommittedUpTo[replica]+1].Status == epaxosproto.EXECUTED) {
		r.CommittedUpTo[replica] = r.CommittedUpTo[replica] + 1
	}
}

func (r *Replica) updateConflicts(cmds []state.Command, replica int32, instance int32, seq int32) {
	for i := 0; i < len(cmds); i++ {
		// Reads can't generate conflicts
		if cmds[i].Op == state.GET {
			continue
		}

		if d, present := r.conflicts[replica][cmds[i].K]; present {
			// If this is the highest instance on this key for this replica that we've seen
			if d < instance {
				r.conflicts[replica][cmds[i].K] = instance
			}
		} else {
			// If we haven't seen an instance on this key from this replica before
			r.conflicts[replica][cmds[i].K] = instance
		}
		if s, present := r.maxSeqPerKey[cmds[i].K]; present {
			// If we are seeing a greater sequence number on this key that we have seen before
			if s < seq {
				r.maxSeqPerKey[cmds[i].K] = seq
			}
		} else {
			// If this is the first key we're seeing on this instance
			r.maxSeqPerKey[cmds[i].K] = seq
		}
	}
}

func (r *Replica) updateAttributes(cmds []state.Command, seq int32, deps [DS]int32, replica int32, instance int32) (int32, [DS]int32, bool) {
	changed := false
	for q := 0; q < r.N; q++ {
		for i := 0; i < len(cmds); i++ {
			if d, present := (r.conflicts[q])[cmds[i].K]; present {
				if d > deps[q] {
					deps[q] = d
					changed = true
				}
			}
		}
	}
	for i := 0; i < len(cmds); i++ {
		if s, present := r.maxSeqPerKey[cmds[i].K]; present {
			if seq <= s {
				changed = true
				seq = s + 1
			}
		}
	}

	return seq, deps, changed
}

func (r *Replica) mergeAttributes(seq1 int32, deps1 [DS]int32, seq2 int32, deps2 [DS]int32) (int32, [DS]int32, bool) {
	equal := true
	if seq1 != seq2 {
		equal = false
		if seq2 > seq1 {
			seq1 = seq2
		}
	}
	for q := 0; q < r.N; q++ {
		if deps1[q] != deps2[q] {
			equal = false
			if deps2[q] > deps1[q] {
				deps1[q] = deps2[q]
			}
		}
	}
	return seq1, deps1, equal
}

func equal(deps1 *[DS]int32, deps2 *[DS]int32) bool {
	for i := 0; i < len(deps1); i++ {
		if deps1[i] != deps2[i] {
			return false
		}
	}
	return true
}

/**********************************************************************

                            PHASE 1

***********************************************************************/

func (r *Replica) handlePropose(propose *genericsmr.Propose) {
	//TODO!! Handle client retries

	batchSize := 1
	if r.batchingEnabled {
		batchSize = len(r.ProposeChan) + 1
		if batchSize > MAX_BATCH {
			batchSize = MAX_BATCH
		}
	}

	instNo := r.proposeInstno
	r.proposeInstno++
	if r.clockSyncType == CLOCK_SYNC_NONE {
		r.crtInstance[r.Id]++
	}

	cmds := make([]state.Command, batchSize)
	proposals := make([]*genericsmr.Propose, batchSize)
	cmds[0] = propose.Command
	proposals[0] = propose
	for i := 1; i < batchSize; i++ {
		prop := <-r.ProposeChan
		cmds[i] = prop.Command
		proposals[i] = prop
	}

	r.startPhase1(r.Id, instNo, 0, proposals, cmds, batchSize)
}

func (r *Replica) startPhase1(replica int32, instance int32, ballot int32, proposals []*genericsmr.Propose, cmds []state.Command, batchSize int) {
	//init command attributes

	seq := int32(0)
	var deps [DS]int32
	for q := 0; q < r.N; q++ {
		deps[q] = -1
	}

	if r.clockSyncType == CLOCK_SYNC_NONE {
		seq, deps, _ = r.updateAttributes(cmds, seq, deps, replica, instance)

		r.InstanceSpace[r.Id][instance] = &Instance{
			cmds,
			ballot,
			epaxosproto.PREACCEPTED,
			seq,
			deps,
			NewLeaderBookkeeping(proposals, deps), 0, 0,
		}

		r.updateConflicts(cmds, r.Id, instance, seq)

		if seq >= r.maxSeq {
			r.maxSeq = seq + 1
		}

		r.recordInstanceMetadata(r.InstanceSpace[r.Id][instance])
		r.recordCommands(cmds)
		r.sync()
	} else {
		r.pendingProposals[instance] = proposals
	}

	r.bcastPreAccept(r.Id, instance, ballot, cmds, seq, deps)

	cpcounter += batchSize

	if r.Id == 0 && DO_CHECKPOINTING && cpcounter >= CHECKPOINT_PERIOD {
		cpcounter = 0

		//Propose a checkpoint command to act like a barrier.
		//This allows replicas to discard their dependency hashtables.
		r.crtInstance[r.Id]++
		instance++

		r.maxSeq++
		for q := 0; q < r.N; q++ {
			deps[q] = r.crtInstance[q] - 1
		}

		r.InstanceSpace[r.Id][instance] = &Instance{
			cpMarker,
			0,
			epaxosproto.PREACCEPTED,
			r.maxSeq,
			deps,
			&LeaderBookkeeping{nil, 0, 0, true, 0, 0, 0, deps, nil, nil, false,
				false, nil, 0, time.Time{}},
			0,
			0,
		}

		r.latestCPReplica = r.Id
		r.latestCPInstance = instance

		//discard dependency hashtables
		r.clearHashtables()

		r.recordInstanceMetadata(r.InstanceSpace[r.Id][instance])
		r.sync()

		r.bcastPreAccept(r.Id, instance, 0, cpMarker, r.maxSeq, deps)
	}
}

func (r *Replica) handlePreAccept(preAccept *epaxosproto.PreAccept) {
	inst := r.InstanceSpace[preAccept.LeaderId][preAccept.Instance]

	if preAccept.Seq >= r.maxSeq {
		r.maxSeq = preAccept.Seq + 1
	}

	if inst != nil && (inst.Status == epaxosproto.COMMITTED || inst.Status == epaxosproto.ACCEPTED) {
		//reordered handling of commit/accept and pre-accept
		if inst.Cmds == nil {
			r.InstanceSpace[preAccept.LeaderId][preAccept.Instance].Cmds = preAccept.Command
			r.updateConflicts(preAccept.Command, preAccept.Replica, preAccept.Instance, preAccept.Seq)
		}
		r.recordCommands(preAccept.Command)
		r.sync()
		return
	}

	if preAccept.Instance >= r.crtInstance[preAccept.Replica] {
		r.crtInstance[preAccept.Replica] = preAccept.Instance + 1
	}

	//update attributes for command
	seq, deps, changed := r.updateAttributes(preAccept.Command, preAccept.Seq, preAccept.Deps, preAccept.Replica, preAccept.Instance)
	uncommittedDeps := false
	for q := 0; q < r.N; q++ {
		if deps[q] > r.CommittedUpTo[q] {
			uncommittedDeps = true
			break
		}
	}
	status := epaxosproto.PREACCEPTED_EQ
	if changed {
		status = epaxosproto.PREACCEPTED
	}

	msg := "OpenAfter: %d, Instance: %d.%d, Seq: %d, Deps: {0.%d, 1.%d, 2.%d, 3.%d, 4.%d}"
	r.paOrderTimetrace.Record9(&msg, preAccept.OpenAfter, int64(preAccept.Replica),
		int64(preAccept.Instance), int64(seq), int64(deps[0]), int64(deps[1]), int64(deps[2]),
		int64(deps[3]), int64(deps[4]))

	if inst != nil {
		if preAccept.Ballot < inst.ballot {
			r.replyPreAccept(preAccept.LeaderId,
				&epaxosproto.PreAcceptReply{
					preAccept.Replica,
					preAccept.Instance,
					FALSE,
					inst.ballot,
					inst.Seq,
					inst.Deps,
					r.CommittedUpTo})
			return
		} else {
			inst.Cmds = preAccept.Command
			inst.Seq = seq
			inst.Deps = deps
			inst.ballot = preAccept.Ballot
			inst.Status = status
		}
	} else {
		var lb *LeaderBookkeeping = nil
		if preAccept.Replica == r.Id {
			proposals := r.pendingProposals[preAccept.Instance]
			delete(r.pendingProposals, preAccept.Instance)
			lb = NewLeaderBookkeeping(proposals, deps)
		}
		r.InstanceSpace[preAccept.Replica][preAccept.Instance] = &Instance{
			preAccept.Command,
			preAccept.Ballot,
			status,
			seq,
			deps,
			lb, 0, 0,
		}
	}

	// Sarah: Updated 1/14 from preAccept.Seq to seq
	r.updateConflicts(preAccept.Command, preAccept.Replica, preAccept.Instance, seq)

	r.recordInstanceMetadata(r.InstanceSpace[preAccept.Replica][preAccept.Instance])
	r.recordCommands(preAccept.Command)
	r.sync()

	if len(preAccept.Command) == 0 {
		//checkpoint
		//update latest checkpoint info
		r.latestCPReplica = preAccept.Replica
		r.latestCPInstance = preAccept.Instance

		//discard dependency hashtables
		r.clearHashtables()
	}

	if preAccept.Replica != r.Id { // Don't reply to ourself
		if changed || uncommittedDeps || preAccept.Replica != preAccept.LeaderId || !isInitialBallot(preAccept.Ballot) {
			r.replyPreAccept(preAccept.LeaderId,
				&epaxosproto.PreAcceptReply{
					preAccept.Replica,
					preAccept.Instance,
					TRUE,
					preAccept.Ballot,
					seq,
					deps,
					r.CommittedUpTo})
		} else {
			pok := &epaxosproto.PreAcceptOK{preAccept.Instance}
			r.SendMsg(preAccept.LeaderId, r.preAcceptOKRPC, pok)
		}
	}
}

func (r *Replica) handlePreAcceptReply(pareply *epaxosproto.PreAcceptReply) {
	inst := r.InstanceSpace[pareply.Replica][pareply.Instance]

	if inst.Status != epaxosproto.PREACCEPTED && inst.Status != epaxosproto.PREACCEPTED_EQ {
		// we've moved on, this is a delayed reply
		return
	}

	if inst.ballot != pareply.Ballot {
		return
	}

	if pareply.OK == FALSE {
		// TODO: there is probably another active leader
		inst.lb.nacks++
		if pareply.Ballot > inst.lb.maxRecvBallot {
			inst.lb.maxRecvBallot = pareply.Ballot
		}
		if inst.lb.nacks >= r.N/2 {
			// TODO
		}
		return
	}

	originalSeq := inst.Seq
	originalDeps := make([]int32, len(inst.Deps))
	copy(originalDeps, inst.Deps[:])

	inst.lb.preAcceptOKs++

	var equal bool
	inst.Seq, inst.Deps, equal = r.mergeAttributes(inst.Seq, inst.Deps, pareply.Seq, pareply.Deps)
	if (r.N <= 3 && !r.Thrifty) || inst.lb.preAcceptOKs > 1 {
		inst.lb.allEqual = inst.lb.allEqual && equal
	}

	r.updateConflicts(inst.Cmds, pareply.Replica, pareply.Instance, inst.Seq)

	// If clock sync is enabled, the originating replica can't have a higher seq
	// or deps than the responses
	if inst.lb.preAcceptOKs == 1 && r.clockSyncType != CLOCK_SYNC_NONE {
		notEqual := originalSeq > inst.Seq
		for i, _ := range inst.Deps {
			notEqual = notEqual || originalDeps[i] > inst.Deps[i]
		}
		if notEqual {
			inst.lb.allEqual = inst.lb.allEqual && equal
		}
	}

	// Sarah update: && allCommitted was required to take the fast path, but
	// that is not part of the protocol
	//can we commit on the fast path?
	if inst.lb.preAcceptOKs >= r.N/2 && inst.lb.allEqual && isInitialBallot(inst.ballot) {
		r.kFast++

		r.InstanceSpace[pareply.Replica][pareply.Instance].Status = epaxosproto.COMMITTED
		r.updateCommitted(pareply.Replica)
		if inst.lb.clientProposals != nil && state.AllBlindWrites(inst.Cmds) {
			// give clients the all clear
			for i := 0; i < len(inst.lb.clientProposals); i++ {
				r.ReplyPropose(
					&genericsmrproto.ProposeReply{
						TRUE,
						inst.lb.clientProposals[i].CommandId,
						state.NIL,
						// Overload timestamp with commit timestamp
						// inst.lb.clientProposals[i].Timestamp
						time.Now().Sub(inst.lb.commitTime).Nanoseconds()},
					inst.lb.clientProposals[i].Reply)
			}
		}

		r.recordInstanceMetadata(inst)
		r.sync() //is this necessary here?

		r.bcastCommit(pareply.Replica, pareply.Instance, inst.Cmds, inst.Seq, inst.Deps)
	} else if inst.lb.preAcceptOKs >= r.N/2 {
		r.kSlow++

		inst.Status = epaxosproto.ACCEPTED
		r.bcastAccept(pareply.Replica, pareply.Instance, inst.ballot, int32(len(inst.Cmds)), inst.Seq, inst.Deps)
	}
	//TODO: take the slow path if messages are slow to arrive
}

func (r *Replica) handlePreAcceptOK(pareply *epaxosproto.PreAcceptOK) {
	inst := r.InstanceSpace[r.Id][pareply.Instance]

	if inst.Status != epaxosproto.PREACCEPTED && inst.Status != epaxosproto.PREACCEPTED_EQ {
		// we've moved on, this is a delayed reply
		return
	}

	if !isInitialBallot(inst.ballot) {
		return
	}

	inst.lb.preAcceptOKs++

	// Sarah update: && allCommitted was required to take the fast path, but
	// that is not part of the protocol
	// can we commit on the fast path?
	if inst.lb.preAcceptOKs >= r.N/2 && inst.lb.allEqual && isInitialBallot(inst.ballot) {
		r.kFast++

		r.InstanceSpace[r.Id][pareply.Instance].Status = epaxosproto.COMMITTED
		r.updateCommitted(r.Id)
		if inst.lb.clientProposals != nil && state.AllBlindWrites(inst.Cmds) {
			// give clients the all clear
			for i := 0; i < len(inst.lb.clientProposals); i++ {
				r.ReplyPropose(
					&genericsmrproto.ProposeReply{
						TRUE,
						inst.lb.clientProposals[i].CommandId,
						state.NIL,
						inst.lb.clientProposals[i].Timestamp},
					inst.lb.clientProposals[i].Reply)
			}
		}

		r.recordInstanceMetadata(inst)
		r.sync() //is this necessary here?

		r.bcastCommit(r.Id, pareply.Instance, inst.Cmds, inst.Seq, inst.Deps)
	} else if inst.lb.preAcceptOKs >= r.N/2 {
		r.kSlow++

		inst.Status = epaxosproto.ACCEPTED
		r.bcastAccept(r.Id, pareply.Instance, inst.ballot, int32(len(inst.Cmds)), inst.Seq, inst.Deps)
	}
	//TODO: take the slow path if messages are slow to arrive
}

/**********************************************************************

                        PHASE 2

***********************************************************************/

func (r *Replica) handleAccept(accept *epaxosproto.Accept) {
	inst := r.InstanceSpace[accept.LeaderId][accept.Instance]

	if accept.Seq >= r.maxSeq {
		r.maxSeq = accept.Seq + 1
	}

	if inst != nil && (inst.Status == epaxosproto.COMMITTED || inst.Status == epaxosproto.EXECUTED) {
		return
	}

	if accept.Instance >= r.crtInstance[accept.LeaderId] {
		r.crtInstance[accept.LeaderId] = accept.Instance + 1
	}

	if inst != nil {
		if accept.Ballot < inst.ballot {
			r.replyAccept(accept.LeaderId, &epaxosproto.AcceptReply{accept.Replica, accept.Instance, FALSE, inst.ballot})
			return
		}
		inst.Status = epaxosproto.ACCEPTED
		inst.Seq = accept.Seq
		inst.Deps = accept.Deps

		r.updateConflicts(inst.Cmds, accept.Replica, accept.Instance, inst.Seq)
	} else {
		r.InstanceSpace[accept.LeaderId][accept.Instance] = &Instance{
			nil,
			accept.Ballot,
			epaxosproto.ACCEPTED,
			accept.Seq,
			accept.Deps,
			nil, 0, 0}

		if accept.Count == 0 {
			//checkpoint
			//update latest checkpoint info
			r.latestCPReplica = accept.Replica
			r.latestCPInstance = accept.Instance

			//discard dependency hashtables
			r.clearHashtables()
		}
	}

	r.recordInstanceMetadata(r.InstanceSpace[accept.Replica][accept.Instance])
	r.sync()

	r.replyAccept(accept.LeaderId,
		&epaxosproto.AcceptReply{
			accept.Replica,
			accept.Instance,
			TRUE,
			accept.Ballot})
}

func (r *Replica) handleAcceptReply(areply *epaxosproto.AcceptReply) {
	inst := r.InstanceSpace[areply.Replica][areply.Instance]

	if inst.Status != epaxosproto.ACCEPTED {
		// we've move on, these are delayed replies, so just ignore
		return
	}

	if inst.ballot != areply.Ballot {
		return
	}

	if areply.OK == FALSE {
		// TODO: there is probably another active leader
		inst.lb.nacks++
		if areply.Ballot > inst.lb.maxRecvBallot {
			inst.lb.maxRecvBallot = areply.Ballot
		}
		if inst.lb.nacks >= r.N/2 {
			// TODO
		}
		return
	}

	inst.lb.acceptOKs++

	if inst.lb.acceptOKs+1 > r.N/2 {
		r.InstanceSpace[areply.Replica][areply.Instance].Status = epaxosproto.COMMITTED
		r.updateCommitted(areply.Replica)
		if inst.lb.clientProposals != nil && state.AllBlindWrites(inst.Cmds) {
			// give clients the all clear
			for i := 0; i < len(inst.lb.clientProposals); i++ {
				r.ReplyPropose(
					&genericsmrproto.ProposeReply{
						TRUE,
						inst.lb.clientProposals[i].CommandId,
						state.NIL,
						inst.lb.clientProposals[i].Timestamp},
					inst.lb.clientProposals[i].Reply)
			}
		}

		r.recordInstanceMetadata(inst)
		r.sync() //is this necessary here?

		r.bcastCommit(areply.Replica, areply.Instance, inst.Cmds, inst.Seq, inst.Deps)
	}
}

/**********************************************************************

                            COMMIT

***********************************************************************/

func (r *Replica) handleCommit(commit *epaxosproto.Commit) {
	inst := r.InstanceSpace[commit.Replica][commit.Instance]

	if commit.Seq >= r.maxSeq {
		r.maxSeq = commit.Seq + 1
	}

	if commit.Instance >= r.crtInstance[commit.Replica] {
		r.crtInstance[commit.Replica] = commit.Instance + 1
	}

	if inst != nil {
		if inst.lb != nil && inst.lb.clientProposals != nil && len(commit.Command) == 0 {
			//someone committed a NO-OP, but we have proposals for this instance
			//try in a different instance
			for _, p := range inst.lb.clientProposals {
				r.ProposeChan <- p
			}
			inst.lb = nil
		}
		// Sarah update: the line below is needed in case a replica became
		// preferred between the PreAccept and Accept phase. In that case,
		// inst will not be nil, but we won't know the commands of the instance.
		// This is only relevant when thrifty is enabled.
		inst.Cmds = commit.Command
		inst.Seq = commit.Seq
		inst.Deps = commit.Deps
		inst.Status = epaxosproto.COMMITTED
	} else {
		r.InstanceSpace[commit.Replica][int(commit.Instance)] = &Instance{
			commit.Command,
			0,
			epaxosproto.COMMITTED,
			commit.Seq,
			commit.Deps,
			nil,
			0,
			0,
		}

		if len(commit.Command) == 0 {
			//checkpoint
			//update latest checkpoint info
			r.latestCPReplica = commit.Replica
			r.latestCPInstance = commit.Instance

			//discard dependency hashtables
			r.clearHashtables()
		}
	}
	// Sarah: moved this so that it happens in either case
	r.updateConflicts(commit.Command, commit.Replica, commit.Instance, commit.Seq)
	r.updateCommitted(commit.Replica)

	r.recordInstanceMetadata(r.InstanceSpace[commit.Replica][commit.Instance])
	r.recordCommands(commit.Command)
}

func (r *Replica) handleCommitShort(commit *epaxosproto.CommitShort) {
	inst := r.InstanceSpace[commit.Replica][commit.Instance]

	if commit.Instance >= r.crtInstance[commit.Replica] {
		r.crtInstance[commit.Replica] = commit.Instance + 1
	}

	if inst != nil {
		if inst.lb != nil && inst.lb.clientProposals != nil {
			//try command in a different instance
			for _, p := range inst.lb.clientProposals {
				r.ProposeChan <- p
			}
			inst.lb = nil
		}
		inst.Seq = commit.Seq
		inst.Deps = commit.Deps
		inst.Status = epaxosproto.COMMITTED
	} else {
		r.InstanceSpace[commit.Replica][commit.Instance] = &Instance{
			nil,
			0,
			epaxosproto.COMMITTED,
			commit.Seq,
			commit.Deps,
			nil, 0, 0}

		if commit.Count == 0 {
			//checkpoint
			//update latest checkpoint info
			r.latestCPReplica = commit.Replica
			r.latestCPInstance = commit.Instance

			//discard dependency hashtables
			r.clearHashtables()
		}
	}
	r.updateCommitted(commit.Replica)

	r.recordInstanceMetadata(r.InstanceSpace[commit.Replica][commit.Instance])
}

/**********************************************************************

                      RECOVERY ACTIONS

***********************************************************************/

func (r *Replica) startRecoveryForInstance(replica int32, instance int32) {
	var nildeps [DS]int32

	if r.InstanceSpace[replica][instance] == nil {
		r.InstanceSpace[replica][instance] = &Instance{nil, 0, epaxosproto.NONE, 0, nildeps, nil, 0, 0}
	}

	inst := r.InstanceSpace[replica][instance]
	if inst.lb == nil {
		inst.lb = &LeaderBookkeeping{nil, -1, 0, false, 0, 0, 0, nildeps, nil,
			nil, true, false, nil, 0, time.Time{}}

	} else {
		inst.lb = &LeaderBookkeeping{inst.lb.clientProposals, -1, 0, false, 0,
			0, 0, nildeps, nil, nil, true, false, nil, 0, time.Time{}}
	}

	if inst.Status == epaxosproto.ACCEPTED {
		inst.lb.recoveryInst = &RecoveryInstance{inst.Cmds, inst.Status, inst.Seq, inst.Deps, 0, false}
		inst.lb.maxRecvBallot = inst.ballot
	} else if inst.Status >= epaxosproto.PREACCEPTED {
		inst.lb.recoveryInst = &RecoveryInstance{inst.Cmds, inst.Status, inst.Seq, inst.Deps, 1, (r.Id == replica)}
	}

	//compute larger ballot
	inst.ballot = r.makeBallotLargerThan(inst.ballot)

	r.bcastPrepare(replica, instance, inst.ballot)
}

func (r *Replica) handlePrepare(prepare *epaxosproto.Prepare) {
	inst := r.InstanceSpace[prepare.Replica][prepare.Instance]
	var preply *epaxosproto.PrepareReply
	var nildeps [DS]int32

	if inst == nil {
		r.InstanceSpace[prepare.Replica][prepare.Instance] = &Instance{
			nil,
			prepare.Ballot,
			epaxosproto.NONE,
			0,
			nildeps,
			nil, 0, 0}
		preply = &epaxosproto.PrepareReply{
			r.Id,
			prepare.Replica,
			prepare.Instance,
			TRUE,
			-1,
			epaxosproto.NONE,
			nil,
			-1,
			nildeps}
	} else {
		ok := TRUE
		if prepare.Ballot < inst.ballot {
			ok = FALSE
		} else {
			inst.ballot = prepare.Ballot
		}
		preply = &epaxosproto.PrepareReply{
			r.Id,
			prepare.Replica,
			prepare.Instance,
			ok,
			inst.ballot,
			inst.Status,
			inst.Cmds,
			inst.Seq,
			inst.Deps}
	}

	r.replyPrepare(prepare.LeaderId, preply)
}

func (r *Replica) handlePrepareReply(preply *epaxosproto.PrepareReply) {
	inst := r.InstanceSpace[preply.Replica][preply.Instance]

	if inst.lb == nil || !inst.lb.preparing {
		// we've moved on -- these are delayed replies, so just ignore
		// TODO: should replies for non-current ballots be ignored?
		return
	}

	if preply.OK == FALSE {
		// TODO: there is probably another active leader, back off and retry later
		inst.lb.nacks++
		return
	}

	//Got an ACK (preply.OK == TRUE)

	inst.lb.prepareOKs++

	if preply.Status == epaxosproto.COMMITTED || preply.Status == epaxosproto.EXECUTED {
		r.InstanceSpace[preply.Replica][preply.Instance] = &Instance{
			preply.Command,
			inst.ballot,
			epaxosproto.COMMITTED,
			preply.Seq,
			preply.Deps,
			nil, 0, 0}
		r.bcastCommit(preply.Replica, preply.Instance, inst.Cmds, preply.Seq, preply.Deps)
		//TODO: check if we should send notifications to clients
		return
	}

	if preply.Status == epaxosproto.ACCEPTED {
		if inst.lb.recoveryInst == nil || inst.lb.maxRecvBallot < preply.Ballot {
			inst.lb.recoveryInst = &RecoveryInstance{preply.Command, preply.Status, preply.Seq, preply.Deps, 0, false}
			inst.lb.maxRecvBallot = preply.Ballot
		}
	}

	if (preply.Status == epaxosproto.PREACCEPTED || preply.Status == epaxosproto.PREACCEPTED_EQ) &&
		(inst.lb.recoveryInst == nil || inst.lb.recoveryInst.status < epaxosproto.ACCEPTED) {
		if inst.lb.recoveryInst == nil {
			inst.lb.recoveryInst = &RecoveryInstance{preply.Command, preply.Status, preply.Seq, preply.Deps, 1, false}
		} else if preply.Seq == inst.Seq && equal(&preply.Deps, &inst.Deps) {
			inst.lb.recoveryInst.preAcceptCount++
		} else if preply.Status == epaxosproto.PREACCEPTED_EQ {
			// If we get different ordering attributes from pre-acceptors, we must go with the ones
			// that agreed with the initial command leader (in case we do not use Thrifty).
			// This is safe if we use thrifty, although we can also safely start phase 1 in that case.
			inst.lb.recoveryInst = &RecoveryInstance{preply.Command, preply.Status, preply.Seq, preply.Deps, 1, false}
		}
		if preply.AcceptorId == preply.Replica {
			//if the reply is from the initial command leader, then it's safe to restart phase 1
			inst.lb.recoveryInst.leaderResponded = true
			return
		}
	}

	if inst.lb.prepareOKs < r.N/2 {
		return
	}

	//Received Prepare replies from a majority

	ir := inst.lb.recoveryInst

	if ir != nil {
		//at least one replica has (pre-)accepted this instance
		if ir.status == epaxosproto.ACCEPTED ||
			(!ir.leaderResponded && ir.preAcceptCount >= r.N/2 && (r.Thrifty || ir.status == epaxosproto.PREACCEPTED_EQ)) {
			//safe to go to Accept phase
			inst.Cmds = ir.cmds
			inst.Seq = ir.seq
			inst.Deps = ir.deps
			inst.Status = epaxosproto.ACCEPTED
			inst.lb.preparing = false
			r.bcastAccept(preply.Replica, preply.Instance, inst.ballot, int32(len(inst.Cmds)), inst.Seq, inst.Deps)
		} else if !ir.leaderResponded && ir.preAcceptCount >= (r.N/2+1)/2 {
			//send TryPreAccepts
			//but first try to pre-accept on the local replica
			inst.lb.preAcceptOKs = 0
			inst.lb.nacks = 0
			inst.lb.possibleQuorum = make([]bool, r.N)
			for q := 0; q < r.N; q++ {
				inst.lb.possibleQuorum[q] = true
			}
			if conf, q, i := r.findPreAcceptConflicts(ir.cmds, preply.Replica, preply.Instance, ir.seq, ir.deps); conf {
				if r.InstanceSpace[q][i].Status >= epaxosproto.COMMITTED {
					//start Phase1 in the initial leader's instance
					r.startPhase1(preply.Replica, preply.Instance, inst.ballot, inst.lb.clientProposals, ir.cmds, len(ir.cmds))
					return
				} else {
					inst.lb.nacks = 1
					inst.lb.possibleQuorum[r.Id] = false
				}
			} else {
				inst.Cmds = ir.cmds
				inst.Seq = ir.seq
				inst.Deps = ir.deps
				inst.Status = epaxosproto.PREACCEPTED
				inst.lb.preAcceptOKs = 1
			}
			inst.lb.preparing = false
			inst.lb.tryingToPreAccept = true
			r.bcastTryPreAccept(preply.Replica, preply.Instance, inst.ballot, inst.Cmds, inst.Seq, inst.Deps)
		} else {
			//start Phase1 in the initial leader's instance
			inst.lb.preparing = false
			r.startPhase1(preply.Replica, preply.Instance, inst.ballot, inst.lb.clientProposals, ir.cmds, len(ir.cmds))
		}
	} else {
		//try to finalize instance by proposing NO-OP
		var noop_deps [DS]int32
		// commands that depended on this instance must look at all previous instances
		noop_deps[preply.Replica] = preply.Instance - 1
		inst.lb.preparing = false
		r.InstanceSpace[preply.Replica][preply.Instance] = &Instance{
			nil,
			inst.ballot,
			epaxosproto.ACCEPTED,
			0,
			noop_deps,
			inst.lb, 0, 0}
		r.bcastAccept(preply.Replica, preply.Instance, inst.ballot, 0, 0, noop_deps)
	}
}

func (r *Replica) handleTryPreAccept(tpa *epaxosproto.TryPreAccept) {
	inst := r.InstanceSpace[tpa.Replica][tpa.Instance]
	if inst != nil && inst.ballot > tpa.Ballot {
		// ballot number too small
		r.replyTryPreAccept(tpa.LeaderId, &epaxosproto.TryPreAcceptReply{
			r.Id,
			tpa.Replica,
			tpa.Instance,
			FALSE,
			inst.ballot,
			tpa.Replica,
			tpa.Instance,
			inst.Status})
	}
	if conflict, confRep, confInst := r.findPreAcceptConflicts(tpa.Command, tpa.Replica, tpa.Instance, tpa.Seq, tpa.Deps); conflict {
		// there is a conflict, can't pre-accept
		r.replyTryPreAccept(tpa.LeaderId, &epaxosproto.TryPreAcceptReply{
			r.Id,
			tpa.Replica,
			tpa.Instance,
			FALSE,
			inst.ballot,
			confRep,
			confInst,
			r.InstanceSpace[confRep][confInst].Status})
	} else {
		// can pre-accept
		if tpa.Instance >= r.crtInstance[tpa.Replica] {
			r.crtInstance[tpa.Replica] = tpa.Instance + 1
		}
		if inst != nil {
			inst.Cmds = tpa.Command
			inst.Deps = tpa.Deps
			inst.Seq = tpa.Seq
			inst.Status = epaxosproto.PREACCEPTED
			inst.ballot = tpa.Ballot
		} else {
			r.InstanceSpace[tpa.Replica][tpa.Instance] = &Instance{
				tpa.Command,
				tpa.Ballot,
				epaxosproto.PREACCEPTED,
				tpa.Seq,
				tpa.Deps,
				nil, 0, 0,
			}
		}
		r.replyTryPreAccept(tpa.LeaderId, &epaxosproto.TryPreAcceptReply{r.Id, tpa.Replica, tpa.Instance, TRUE, inst.ballot, 0, 0, 0})
	}
}

func (r *Replica) findPreAcceptConflicts(cmds []state.Command, replica int32, instance int32, seq int32, deps [DS]int32) (bool, int32, int32) {
	inst := r.InstanceSpace[replica][instance]
	if inst != nil && len(inst.Cmds) > 0 {
		if inst.Status >= epaxosproto.ACCEPTED {
			// already ACCEPTED or COMMITTED
			// we consider this a conflict because we shouldn't regress to PRE-ACCEPTED
			return true, replica, instance
		}
		if inst.Seq == tpa.Seq && equal(&inst.Deps, &tpa.Deps) {
			// already PRE-ACCEPTED, no point looking for conflicts again
			return false, replica, instance
		}
	}
	for q := int32(0); q < int32(r.N); q++ {
		for i := r.ExecedUpTo[q]; i < r.crtInstance[q]; i++ {
			if replica == q && instance == i {
				// no point checking past instance in replica's row, since replica would have
				// set the dependencies correctly for anything started after instance
				break
			}
			if i == deps[q] {
				//the instance cannot be a dependency for itself
				continue
			}
			inst := r.InstanceSpace[q][i]
			if inst == nil || inst.Cmds == nil || len(inst.Cmds) == 0 {
				continue
			}
			if inst.Deps[replica] >= instance {
				// instance q.i depends on instance replica.instance, it is not a conflict
				continue
			}
			if state.ConflictBatch(inst.Cmds, cmds) {
				if i > deps[q] ||
					(i < deps[q] && inst.Seq >= seq && (q != replica || inst.Status > epaxosproto.PREACCEPTED_EQ)) {
					// this is a conflict
					return true, q, i
				}
			}
		}
	}
	return false, -1, -1
}

func (r *Replica) handleTryPreAcceptReply(tpar *epaxosproto.TryPreAcceptReply) {
	inst := r.InstanceSpace[tpar.Replica][tpar.Instance]
	if inst == nil || inst.lb == nil || !inst.lb.tryingToPreAccept || inst.lb.recoveryInst == nil {
		return
	}

	ir := inst.lb.recoveryInst

	if tpar.OK == TRUE {
		inst.lb.preAcceptOKs++
		inst.lb.tpaOKs++
		if inst.lb.preAcceptOKs >= r.N/2 {
			//it's safe to start Accept phase
			inst.Cmds = ir.cmds
			inst.Seq = ir.seq
			inst.Deps = ir.deps
			inst.Status = epaxosproto.ACCEPTED
			inst.lb.tryingToPreAccept = false
			inst.lb.acceptOKs = 0
			r.bcastAccept(tpar.Replica, tpar.Instance, inst.ballot, int32(len(inst.Cmds)), inst.Seq, inst.Deps)
			return
		}
	} else {
		inst.lb.nacks++
		if tpar.Ballot > inst.ballot {
			//TODO: retry with higher ballot
			return
		}
		inst.lb.tpaOKs++
		if tpar.ConflictReplica == tpar.Replica && tpar.ConflictInstance == tpar.Instance {
			//TODO: re-run prepare
			inst.lb.tryingToPreAccept = false
			return
		}
		inst.lb.possibleQuorum[tpar.AcceptorId] = false
		inst.lb.possibleQuorum[tpar.ConflictReplica] = false
		notInQuorum := 0
		for q := 0; q < r.N; q++ {
			if !inst.lb.possibleQuorum[tpar.AcceptorId] {
				notInQuorum++
			}
		}
		if tpar.ConflictStatus >= epaxosproto.COMMITTED || notInQuorum > r.N/2 {
			//abandon recovery, restart from phase 1
			inst.lb.tryingToPreAccept = false
			r.startPhase1(tpar.Replica, tpar.Instance, inst.ballot, inst.lb.clientProposals, ir.cmds, len(ir.cmds))
		}
		if notInQuorum == r.N/2 {
			//this is to prevent defer cycles
			if present, dq, _ := deferredByInstance(tpar.Replica, tpar.Instance); present {
				if inst.lb.possibleQuorum[dq] {
					//an instance whose leader must have been in this instance's quorum has been deferred for this instance => contradiction
					//abandon recovery, restart from phase 1
					inst.lb.tryingToPreAccept = false
					r.startPhase1(tpar.Replica, tpar.Instance, inst.ballot, inst.lb.clientProposals, ir.cmds, len(ir.cmds))
				}
			}
		}
		if inst.lb.tpaOKs >= r.N/2 {
			//defer recovery and update deferred information
			updateDeferred(tpar.Replica, tpar.Instance, tpar.ConflictReplica, tpar.ConflictInstance)
			inst.lb.tryingToPreAccept = false
		}
	}
}

//helper functions and structures to prevent defer cycles while recovering

var deferMap map[uint64]uint64 = make(map[uint64]uint64)

func updateDeferred(dr int32, di int32, r int32, i int32) {
	daux := (uint64(dr) << 32) | uint64(di)
	aux := (uint64(r) << 32) | uint64(i)
	deferMap[aux] = daux
}

func deferredByInstance(q int32, i int32) (bool, int32, int32) {
	aux := (uint64(q) << 32) | uint64(i)
	daux, present := deferMap[aux]
	if !present {
		return false, 0, 0
	}
	dq := int32(daux >> 32)
	di := int32(daux)
	return true, dq, di
}
