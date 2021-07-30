package genericsmrproto

import (
	"github.com/daginwu/monorepo/libs/epaxos/src/state"
)

const (
	PROPOSE uint8 = iota
	PROPOSE_REPLY
	GENERIC_SMR_BEACON
	GENERIC_SMR_BEACON_REPLY
	METRICS_REQUEST
	METRICS_REPLY
)

const (
	METRICSOP_CONFLICT_RATE uint8 = iota
	METRICSOP_DUMP_OWD
)

type Propose struct {
	CommandId int32
	Command   state.Command
	Timestamp int64
}

type ProposeReply struct {
	OK        uint8
	CommandId int32
	Value     state.Value
	Timestamp int64
}

// handling stalls and failures

type Beacon struct {
	Timestamp uint64
}

type BeaconReply struct {
	Timestamp uint64
}

type PingArgs struct {
	ActAsLeader uint8
}

type PingReply struct {
}

type BeTheLeaderArgs struct {
}

type BeTheLeaderReply struct {
}

// handling experiment requests for metrics

type MetricsRequest struct {
	OpCode uint8
}

type MetricsReply struct {
	KFast int64 // The number of instances that have committed on the fast path
	KSlow int64 // The number of instances that have committed on the slow path
}
