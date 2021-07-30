package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strings"
	"sync"
	"time"

	"github.com/daginwu/monorepo/libs/epaxos/src/genericsmrproto"
	"github.com/daginwu/monorepo/libs/epaxos/src/masterproto"
)

var portnum *int = flag.Int("port", 7087, "Port # to listen on. Defaults to 7087")
var numNodes *int = flag.Int("N", 3, "Number of replicas. Defaults to 3.")
var nodeIPs *string = flag.String("ips", ":7070,:7071,:7072", "Space separated list of IP addresses (ordered). The leader will be 0")

type Master struct {
	N              int
	nodeList       []string
	addrList       []string
	portList       []int
	lock           *sync.Mutex
	nodes          []*rpc.Client
	leader         []bool
	alive          []bool
	expectAddrList []string
	connected      []bool
	nConnected     int
}

func main() {
	flag.Parse()

	log.Printf("Master starting on port %d\n", *portnum)
	log.Printf("...waiting for %d replicas\n", *numNodes)

	ips := []string{}
	if *nodeIPs != "" {
		ips = strings.Split(*nodeIPs, ",")
		log.Println("Ordered replica ips:", ips, len(ips))
	}

	master := &Master{
		*numNodes,
		make([]string, *numNodes),
		make([]string, *numNodes),
		make([]int, *numNodes),
		new(sync.Mutex),
		make([]*rpc.Client, *numNodes),
		make([]bool, *numNodes),
		make([]bool, *numNodes),
		ips,
		make([]bool, *numNodes),
		0,
	}

	rpc.Register(master)
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", *portnum))
	if err != nil {
		log.Fatal("Master listen error:", err)
	}

	go master.run()

	http.Serve(l, nil)
}

func (master *Master) run() {
	for true {
		master.lock.Lock()
		if master.nConnected == master.N {
			master.lock.Unlock()
			break
		}
		master.lock.Unlock()
		time.Sleep(100000000)
	}
	time.Sleep(2000000000)

	// connect to SMR servers
	for i := 0; i < master.N; i++ {
		var err error
		addr := fmt.Sprintf("%s:%d", master.addrList[i], master.portList[i]+1000)
		master.nodes[i], err = rpc.DialHTTP("tcp", addr)
		if err != nil {
			log.Fatalf("Error connecting to replica %d: %v\n", i, err)
		}
		master.leader[i] = false
	}
	master.leader[0] = true

	for true {
		time.Sleep(3000 * 1000 * 1000)
		new_leader := false
		for i, node := range master.nodes {
			err := node.Call("Replica.Ping", new(genericsmrproto.PingArgs), new(genericsmrproto.PingReply))
			if err != nil {
				//log.Printf("Replica %d has failed to reply\n", i)
				master.alive[i] = false
				if master.leader[i] {
					// neet to choose a new leader
					new_leader = true
					master.leader[i] = false
				}
			} else {
				master.alive[i] = true
			}
		}
		if !new_leader {
			continue
		}
		for i, new_master := range master.nodes {
			if master.alive[i] {
				err := new_master.Call("Replica.BeTheLeader", new(genericsmrproto.BeTheLeaderArgs), new(genericsmrproto.BeTheLeaderReply))
				if err == nil {
					master.leader[i] = true
					log.Printf("Replica %d is the new leader.", i)
					break
				}
			}
		}
	}
}

func (master *Master) Register(args *masterproto.RegisterArgs, reply *masterproto.RegisterReply) error {
	master.lock.Lock()
	defer master.lock.Unlock()

	addrPort := fmt.Sprintf("%s:%d", args.Addr, args.Port)

	i := master.N + 1

	log.Println("Received Register", addrPort, master.nodeList)

	for index, ap := range master.nodeList {
		if ap == addrPort {
			i = index
			break
		}
	}

	if i == master.N+1 {
		for index, a := range master.expectAddrList {
			if args.Addr == a {
				i = index
				if !master.connected[i] {
					break
				}
			}
		}
	}

	if i == master.N+1 {
		log.Println("Received register from bad IP:", addrPort)
		return nil
	}

	log.Println("Ended up with index", i)

	if !master.connected[i] {
		master.nodeList[i] = addrPort
		master.addrList[i] = args.Addr
		master.portList[i] = args.Port
		master.connected[i] = true
		master.nConnected++
	}

	if master.nConnected == master.N {
		log.Println("All connected!")
		reply.Ready = true
		reply.ReplicaId = i
		reply.NodeList = master.nodeList
	} else {
		reply.Ready = false
	}

	return nil
}

func (master *Master) GetLeader(args *masterproto.GetLeaderArgs, reply *masterproto.GetLeaderReply) error {
	time.Sleep(4 * 1000 * 1000)
	for i, l := range master.leader {
		if l {
			*reply = masterproto.GetLeaderReply{i}
			break
		}
	}
	return nil
}

func (master *Master) GetReplicaList(args *masterproto.GetReplicaListArgs, reply *masterproto.GetReplicaListReply) error {
	master.lock.Lock()
	defer master.lock.Unlock()

	if master.nConnected == master.N {
		reply.ReplicaList = master.nodeList
		reply.Ready = true
	} else {
		reply.Ready = false
	}
	return nil
}
