package raftkv

import (
	"fmt"
	"labgob"
	"labrpc"
	"log"
	"os"
	"raft"
	//"runtime/debug"
	"sync"
)

const Debug = 0

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		log.Printf(format, a...)
	}
	return
}


type Op struct {
	Operation string
	Key       string
	Value     string
	Id        int64
	Seq       int
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
}

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg
	testCh  chan Op

	db      map[string]string
	indexToCmtedOpCh  map[int]chan Op
	lastAppliedIndex  int
	lastAppliedTerm   int
   	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
	cache  map[int64]int
}


func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {


	// Your code here.
	//todo check if that id in args is already in the queue.
	op:=Op{Operation:"Get", Key:args.Key, Id:args.Id}
	raftindex, term, isLeader:=kv.rf.Start(op)

	kv.mu.Lock()


	if isLeader==false || raftindex==-1 || term==-1 { //todo do I really need all these checks besides just leader
		reply.WrongLeader = true
		reply.Err = ErrNoKey
		fmt.Printf("Me(%d)Get, returning False rightway.\n", kv.me)
		kv.mu.Unlock()
		return
	}
	_, ok:=kv.indexToCmtedOpCh[raftindex]
	if !ok{
		kv.indexToCmtedOpCh[raftindex]=make(chan Op,1)
	}
	fmt.Println(len(kv.indexToCmtedOpCh))
	kv.mu.Unlock()

	opp:=<-kv.indexToCmtedOpCh[raftindex]
	//if opp.Id == args.Id{

	if 1 == 1{
	//if opp.Id == args.Id{
		fmt.Printf("*** Yeah Same id Get == %d\n", opp.Id)
		fmt.Println("db=",kv.db)
		fmt.Println("key=",opp.Key)
		value, ok := kv.db[opp.Key]
		if !ok{
			os.Exit(88)
		}
		reply.Value = value
		reply.WrongLeader = false
		reply.Err=OK
	}else {
		fmt.Printf("!!!! Different opp.id (%d)!= args.Id(%d)\n", opp.Id,args.Id)
		reply.WrongLeader = true
		reply.Err=ErrNoKey
	}

}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.

	//todo check if that id in args is already in the queue.
	op:=Op{Operation:args.Op, Key:args.Key, Id:args.Id, Value:args.Value, Seq:args.Seq}
	raftindex, term, isLeader:=kv.rf.Start(op)

	kv.mu.Lock()


	fmt.Printf("Me(%d):: raftindex=%d, term=%d, isLeader=%v\n", kv.me,raftindex, term, isLeader)
	if isLeader==false || raftindex==-1 || term==-1 { //todo do I really need all these checks besides just leader
		reply.WrongLeader = true
		reply.Err = ErrNoKey
		fmt.Printf("Me(%d)PutAppend, returning False rightway.\n", kv.me)
		kv.mu.Unlock()
		return
	}


	channel, ok:=kv.indexToCmtedOpCh[raftindex]
	if !ok{
		channel = make(chan Op,1)
		kv.indexToCmtedOpCh[raftindex]=channel
	}
	fmt.Println(len(kv.indexToCmtedOpCh))
	kv.mu.Unlock()

	opp:=<-kv.indexToCmtedOpCh[raftindex]
	fmt.Println("JE SIR== ",opp)

	//if opp.Id == args.Id{

	if 1 == 1{
		fmt.Printf("*** Yeah Same id PutAppend == %d\n", opp.Id)
		fmt.Printf("db[%s]=%s now \n", opp.Key, kv.db[opp.Key])
		reply.WrongLeader = false
		reply.Err=OK
	}else {
		fmt.Printf("!!!! Different opp.id (%d)!= args.Id(%d)\n", opp.Id,args.Id)
		reply.WrongLeader = true
		reply.Err=ErrNoKey
	}

}

//
// the tester calls Kill() when a KVServer instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (kv *KVServer) Kill() {
	kv.rf.Kill()
	// Your code here, if desired.
}

//
// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// the k/v server should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
// StartKVServer() must return quickly, so it should start goroutines
// for any long-running work.
//

func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *KVServer {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// You may need initialization code here.
	///////////////////////////////////////
	kv.testCh = make (chan Op, 1)
	kv.db= make(map[string]string)
	kv.indexToCmtedOpCh= make(map[int]chan Op)
	kv.cache=make(map[int64]int)
	/////////////////////////////////////////
	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	// You may need initialization code here.
	go kv.waitOnApplyFromRaft()

	return kv
}

func (kv *KVServer) waitOnApplyFromRaft() {

	for true{
		applyMsgVal:=<-kv.applyCh
		kv.mu.Lock()
		fmt.Printf("raft server(%d) sent entry %v on channel \n", kv.me, applyMsgVal)

		op:=applyMsgVal.Command.(Op)

		//apply the val
		fmt.Printf("length of indexChannel = %d,  applyMsgvalCOmmandidx=%d\n", len(kv.indexToCmtedOpCh), applyMsgVal.CommandIndex)
		kv.applyy(op)

		_, isleader:=kv.rf.GetState()
		fmt.Println(isleader)
		//if isleader == true{
			channel, ok:=kv.indexToCmtedOpCh[applyMsgVal.CommandIndex]
			if !ok{
				channel=make(chan Op , 1)
				kv.indexToCmtedOpCh[applyMsgVal.CommandIndex]=channel
			}
			kv.indexToCmtedOpCh[applyMsgVal.CommandIndex]<-op

		//}

		kv.mu.Unlock()

	}
}

func (kv *KVServer) applyy(op Op) {
	//it can be put or append

	if (kv.cache[op.Id] < op.Seq ) {
		if op.Operation == "Append" {
			kv.db[op.Key] = kv.db[op.Key] + op.Value

		} else if op.Operation == "Put" {
			kv.db[op.Key] = op.Value
		}
		kv.cache[op.Id]=op.Seq
		fmt.Printf("Apply:: key=%s, val=%s & My(%d) db looks like = %v\n", op.Key, op.Value, kv.me, kv.db)
	}
}