package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/daginwu/monorepo/libs/epaxos/src/epaxos"
	"github.com/daginwu/monorepo/libs/epaxos/src/masterproto"
)

// Post for this service
var portnum *int = flag.Int("port", 7070, "Port # to listen on. Defaults to 7070")

// Master host
var masterAddr *string = flag.String("maddr", "", "Master address. Defaults to localhost.")
var masterPort *int = flag.Int("mport", 7087, "Master port.  Defaults to 7087.")

// This service addr
var myAddr *string = flag.String("addr", "", "Server address (this machine). Defaults to localhost.")
var doEpaxos *bool = flag.Bool("e", true, "Use EPaxos as the replication protocol. Defaults to false.")

// OS setting
var procs *int = flag.Int("p", 2, "GOMAXPROCS. Defaults to 2")
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var thrifty = flag.Bool("thrifty", false, "Use only as many messages as strictly required for inter-replica communication.")
var beacon = flag.Bool("beacon", false, "Send beacons to other replicas to compare their relative speeds.")
var durable = flag.Bool("durable", true, "Log to a stable store (i.e., a file in the current dir).")
var batch = flag.Bool("batch", true, "Enables batching of inter-server messages")
var infiniteFix = flag.Bool("inffix", false, "Enables a bound on execution latency for EPaxos")
var clockSyncType = flag.Int("clocksync", 1, "0 to not sync clocks, 1 to delay the opening of messages until the quorum, 2 to delay so that all process at same time, 3 to delay to CA, VA, and OR.")
var clockSyncEpsilon = flag.Float64("clockepsilon", 4, "The number of milliseconds to add as buffer for OpenAfter times.")

func main() {
	flag.Parse()

	runtime.GOMAXPROCS(*procs)

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)

		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt)
		go catchKill(interrupt)
	}
	log.Printf("Server starting on port %d\n", *portnum)

	replicaId, nodeList := registerWithMaster(fmt.Sprintf("%s:%d", *masterAddr, *masterPort))
	log.Println("replicaId: ", replicaId, "nodeList", nodeList)
	log.Println("Epaxos enable: ", *doEpaxos)
	log.Println("Starting Egalitarian Paxos replica...")
	rep := epaxos.NewReplica(
		replicaId,
		nodeList,
		*thrifty,
		*beacon,
		*durable,
		*batch,
		*infiniteFix,
		epaxos.ClockSyncType(*clockSyncType),
		int64(*clockSyncEpsilon*1e6), /* ms to ns */
	)

	rpc.Register(rep)

	rpc.HandleHTTP()
	//listen for RPC on a different port (8070 by default)
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", *portnum+1000))
	if err != nil {
		log.Fatal("listen error:", err)
	}

	http.Serve(l, nil)
}

func registerWithMaster(masterAddr string) (int, []string) {
	args := &masterproto.RegisterArgs{
		*myAddr,
		*portnum,
	}
	var reply masterproto.RegisterReply

	for done := false; !done; {
		mcli, err := rpc.DialHTTP("tcp", masterAddr)
		if err == nil {
			err = mcli.Call("Master.Register", args, &reply)
			if err == nil && reply.Ready == true {
				done = true
				break
			}
		}
		time.Sleep(1e9)
	}

	return reply.ReplicaId, reply.NodeList
}

func catchKill(interrupt chan os.Signal) {
	<-interrupt
	if *cpuprofile != "" {
		pprof.StopCPUProfile()
	}
	fmt.Println("Caught signal")
	os.Exit(0)
}
