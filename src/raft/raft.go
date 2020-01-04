package raft

import (
	"bytes"
	"fmt"
	"labgob"
	"labrpc"
	"math/rand"
	"os"
	"sync"
	"time"
)

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

// import "bytes"
// import "labgob"

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//

const (
	isCandidate   int = 0
	isFollower    int = 1
	isLeader      int = 2
)

type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

type LogEntry struct {
	Command             interface{}
	CommandIndexInlog   int
	CommandTerm         int // term when entry was received by the leader.
}

//invoked by the leader to replicate log entries. Also serves as a heartbeat.
type AppendEntriesRequest struct {
	Term int //leaders term
	LeaderId int // so followers can redirect client requests.
	PrevLogIndex int // index of log entries immediately preceding new ones.
	PrevLogTerm  int  //term of PrevLogIndex entry.
	LogEntries []LogEntry // log entries to store. empty in case of heartbeats.
	LeaderCommit int  // leader's commit index.

}

type AppendEntriesReply struct {
	Term int //currentTerm, for leader to update itself.
	Success bool //true if follower contained entry matching prevLogIndex and prevLogTerm
}


//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	applychannel chan ApplyMsg
	applyMu     sync.Mutex


	// persistent state - all servers
	currentTerm int               // latest term server has seen, increases monotonically from 0 - >
	votedFor    int               // candidateID who received the vote.
	log         []LogEntry


	//volatile state - all servers
	commitIndex int               // index of highest log entry known to be committed.
	lastApplied int               // highest log entry applied to state machine.

	//volatile state - leaders
	nextIndex   []int             // index of next log entry to send to that server. -> init = lastlogint + 1
	matchIndex  []int             // index of highest log entry known to be replicated on server. ->  init = 0

	electiontimeout int           //TODO not sure if I should put it here.
	leaderid    int               //TODO update leaderid when you get append heartbeat from someone. me in case of self
//	iscandidate int
//	isfollower  int
//	isleader	int
	numVotes    int
	state       int
	terminatme  int


	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

}



// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here (2A).
	term = rf.currentTerm
	isleader = false
	if (rf.state == isLeader) { isleader = true}

	return term, isleader
}


//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
// persistent state - all servers

func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.log)
	data := w.Bytes()
	rf.persister.SaveRaftState(data)
}


//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	//Your code here (2C).
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	//var xxx
	//var yyy
	if d.Decode(&rf.currentTerm) != nil ||  d.Decode(&rf.votedFor) != nil ||  d.Decode(&rf.log) != nil {
	  fmt.Errorf("GOT ERROR in Reading Persisting state\n ")
	}
}


//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term          int     // candidate's term
	CandidateId   int     // candidate requesting vote
	LastLogIndex  int     // index of candidate's last log entry
	LastLogTerm   int     // term of candidate's last log entry.
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term          int // currentTerm, for candidate to update itself.
	VoteGranted   bool // true means candidate received vote.
}

func(rf * Raft) sendAppendEntriesToAll(){

	rf.mu.Lock()

	//todo check if prevLogTerm:-1 should be different in case len(log) = 0 and when we have entries in log.
	requestAppendEntries:=AppendEntriesRequest{Term:rf.currentTerm, LeaderId:rf.me,
		PrevLogIndex:-1, PrevLogTerm:rf.currentTerm-1,LeaderCommit:rf.commitIndex}
	rf.mu.Unlock()

	fmt.Println("---------------------------------------------------")
	DPrintf("in sendAppendEntriesToAll by leader(%d) \n Leaers log is =%v\n", rf.me, rf.log )
	for i, _ := range rf.peers {
		if ( i!=rf.me ){
			requestAppendEntries.LogEntries=nil
			rf.mu.Lock()
			if (rf.matchIndex[i]>=0){
				requestAppendEntries.PrevLogTerm = rf.log[len(rf.log)-1].CommandTerm

				if (len(rf.log)>rf.matchIndex[i]){
					requestAppendEntries.PrevLogTerm = rf.log[rf.matchIndex[i]].CommandTerm

					//fmt.Println(rf.log)
					//fmt.Println(i,rf.matchIndex[i], rf.nextIndex[i])

				}
				requestAppendEntries.PrevLogIndex=rf.matchIndex[i]
			}
//			DPrintf()("appendEntries = %v for server %d \n", requestAppendEntries.LogEntries, i )
			if ( len(rf.log)-1 >= rf.nextIndex[i] ) {
				requestAppendEntries.LogEntries = rf.log[rf.nextIndex[i]:]
				//DPrintf()("Have set entries = %v for server %d as nexidx(%d)<len(rf.log)-1(%d)\n", requestAppendEntries.LogEntries,
				//	i, rf.nextIndex[i], len(rf.log)-1 )

			}
			requestAppendEntries.PrevLogIndex = rf.nextIndex[i]-1
			rf.mu.Unlock()
			go func( requestAppendEntries AppendEntriesRequest,i int) {
				replyAppendEntries :=AppendEntriesReply{}
				//requestAppendEntries.PrevLogIndex = rf.nextIndex[i]-1
				DPrintf("nextdx[%d]=%d, and entries = %v\n", i, rf.nextIndex[i], requestAppendEntries.LogEntries)
				rpcsendAppendEntriesStatus:=false
				if (rf.state == isLeader && rpcsendAppendEntriesStatus == false){ // retry indefinitely.
					rpcsendAppendEntriesStatus=rf.sendAppendEntries(i, &requestAppendEntries, &replyAppendEntries)

					//if (rf.nextIndex[i]>0 && rpcsendAppendEntriesStatus ){
					//	rf.nextIndex[i] = rf.nextIndex[i] - 1
					//	rf.matchIndex[i] = rf.matchIndex[i] - 1
					//	requestAppendEntries.LogEntries = rf.log[rf.nextIndex[i]:]
					//	requestAppendEntries.PrevLogIndex = rf.matchIndex[i]
					//	requestAppendEntries.PrevLogTerm = rf.log[rf.matchIndex[i]].CommandTerm
					//}

				}
				DPrintf("***FOR-LOOP-SUCESS**** For leader (%d), rpcsendAppendEntriesStatus[%d] for sendAppendEntries. =%t\n ",rf.me,i,rpcsendAppendEntriesStatus)
				rf.handleAppendEntriesReply(&requestAppendEntries,&replyAppendEntries,i)
				//todo if you get rejected, immediately step down and not update nextindex

			}(requestAppendEntries,i)

		}

	}
	DPrintf("End ******************** sendAppendEntriesToAll by leader(%d)\n", rf.me)


}

//HeartBeats
func (rf * Raft) periodicAppenEntries(){


	for true {
		// only if it is a leader.
		_, isLeader:=rf.GetState();
		if  isLeader == true {
			DPrintf("holy shit I (%d) am a leader. \n", rf.me)

			rf.sendAppendEntriesToAll()
			//time.Sleep(time.Duration(rf.electiontimeout + 100)*time.Millisecond)
			rf.mu.Lock()
			heartbeattime:=rf.electiontimeout/3
			//heartbeattime:=100
			rf.mu.Unlock()
			time.Sleep(time.Duration(heartbeattime) * time.Millisecond)
			//todo time.sleep (rf.electiontimeout/3) // no more than 10 times per second.
		}
		rf.mu.Lock()
		if rf.terminatme == 5 {
			DPrintf("ending periodicAppenEntries for %d \n", rf.me)
			break
		}
		rf.mu.Unlock()

	}

}

//Write an AppendEntries RPC handler method that resets the election timeout so that
//other servers don't step forward as leaders when one has already been elected.
func (rf *Raft) AppendEntries(args *AppendEntriesRequest, reply *AppendEntriesReply){
	rf.mu.Lock()
	defer rf.mu.Unlock()
	//TODO if leader. periodically send append entries.
	DPrintf("I (%d) have recevied AppendEntries from a leader(%d) \n", rf.me, args.LeaderId)

	//if ( rf.state==isFollower ){
		//if not leader
		stattus:=rf.acceptAppendEntries(args,reply) //todo return true only if it returns true.
		reply.Term = rf.currentTerm
		if ( stattus == true ){
			reply.Success = true

		} else {
			fmt.Println("FUCK off I am not gonna apply the entries you sent. ")
			reply.Success = false
		}

//		rf.genRandTimeout()
	//}

	_, isLeader:= rf.GetState()
	if (isLeader == true){
		//todo write these periodic entries somewhere else where leader starts it.
		// problem is for every append entriy it is gonna start go routing
		// this is wrong. as leader can send multiple appendEntries .
		//go rf.periodicAppenEntries() //vs periodicAppendEntries(rf)?
	}

}




func (rf * Raft) handleAppendEntriesReply(args *AppendEntriesRequest, reply *AppendEntriesReply, serverId int, ) bool{
	rf.mu.Lock()
	defer rf.mu.Unlock()
	//compare the current term with the term you sent in your original RPC.
	// If the two are different, drop the reply and return
	DPrintf("I Leader (%d) am gonna handleAppendEntriesReply from follower (%d).. reply is below:;\n", rf.me, serverId)
	fmt.Println(reply)
	if (rf.currentTerm!=args.Term){
		DPrintf("Reject reply as rf.currentTerm(%d)!=args.Term(%d)\n",rf.currentTerm,args.Term)
		//todo not sure if the original term that was sent is still retained in the args.
		return false
	}

	//todo implement where follower does not have entries what you think, decrement next index
	// and try again for that bitch.

	if (reply.Term > rf.currentTerm){
		DPrintf("I am gonna step down as reply.Term(%d) > rf.currentTerm(%d) \n",reply.Term,rf.currentTerm)
		rf.currentTerm = reply.Term
		rf.state = isFollower
		rf.votedFor = -1
		rf.persist()
	}else if (reply.Term == rf.currentTerm){

		//DPrintf()("lets see = %d\n", rf.matchIndex[0])
		//means that follower has replicated all the entries that I sent.
		if (reply.Success == true ){
			rf.matchIndex[serverId]=args.PrevLogIndex + len(args.LogEntries)
			rf.nextIndex[serverId]=args.PrevLogIndex + len(args.LogEntries) + 1 //todo read it once more
			DPrintf("Good:: Server(%d) applied my(%d) entries reply.Term(%d) == rf.currentTerm(%d),  \n " +
				"len(args.LogEntries)=%d matchindex[%d]=%d, nextindex[%d]=%d \n", serverId,rf.me,reply.Term,rf.currentTerm,
				len(args.LogEntries), serverId , rf.matchIndex[serverId], serverId,rf.nextIndex[serverId])


			//checking commiting everytime we update matchindex.
			if len(args.LogEntries) > 0 {
				rf.updateCommitIndexInLeader()
			}
		}else {
			DPrintf("server(%d) reply was false, I should decrement nextIdx[%d]\n", serverId, serverId )
			//rf.nextIndex[serverId] = rf.nextIndex[serverId]-1
			//rf.matchIndex[serverId] = rf.matchIndex[serverId]-1
			rf.nextIndex[serverId] = 0
			rf.matchIndex[serverId] = -1
		}


	}

	return false
}

//tell the leader that their log matches the leader’s log up to and including
// the prevLogIndex included in the AppendEntries arguments.
func (rf * Raft) insertEntriesAtIndexInLog(at int, from int, args *AppendEntriesRequest ){
	if ( len(rf.log)>at ){ //truncate
		rf.log[at].CommandTerm = args.LogEntries[from].CommandTerm
		rf.log[at].CommandIndexInlog = args.LogEntries[from].CommandIndexInlog
		rf.log[at].Command = args.LogEntries[from].Command
		/////////////////////////////////////////////////////
		rf.log = rf.log[:at+1]
		DPrintf("(me %d ) command inserted = %v, command term = %d\n",rf.me ,rf.log[at].Command, rf.log[at].CommandTerm )

	}else{
		rf.log = append(rf.log, args.LogEntries[from])
		DPrintf("(me %d) command appended = %v, command term = %d\n",rf.me ,rf.log[at].Command, rf.log[at].CommandTerm)
	}

	DPrintf("(me %d) args.LeaderCommit=%d, rf.commitIndex= %d rf.lastapplied=%d\n",
		rf.me ,args.LeaderCommit,rf.commitIndex,rf.lastApplied)

	rf.persist()

}
func (rf * Raft) acceptAppendEntries(args *AppendEntriesRequest, reply *AppendEntriesReply) bool {

	conflictingIndex:=-1
	offset:=0
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.votedFor=-1
		rf.state=isFollower
	}
	reply.Term = rf.currentTerm

	if (len(rf.log)>0){

		if (len(rf.log)<=args.PrevLogIndex){
			return false
			//DPrintf()("prevLogIDX=%d\n, log=%v", args.PrevLogIndex, rf.log)
			//os.Exit(88)
		}
		//DPrintf()(" rf.log[%d].CommandTerm(%d) should == args.PrevLogTerm(%d)\n",
		//	args.PrevLogIndex, rf.log[args.PrevLogIndex].CommandTerm, args.PrevLogTerm)
		if (args.PrevLogIndex>=0){
			if (rf.log[args.PrevLogIndex].CommandTerm != args.PrevLogTerm){
				return false
			}

		}


	}

	if (rf.currentTerm > args.Term){
		DPrintf("I am Follwer(%d) and I dont acceptAppendEntries from leader(%d) as rf.currentTerm(%d) > args.Term(%d)\n",
			rf.me, args.LeaderId,rf.currentTerm, args.Term)
		return false
	}

	rf.genRandTimeout() // I wont reseet if term is outdated.

	if (args.PrevLogIndex <= (len(rf.log)-1)) {
		DPrintf("Good, my(%d) lastidex(len(log)-1 -> %d) >= args.PrevLogIndex(%d) and args.log = %v\n",
			rf.me,(len(rf.log)-1), args.PrevLogIndex, args.LogEntries)
		for i := 0; i < len(args.LogEntries); i++ {

			if ( args.PrevLogIndex+1+i>len(rf.log)-1 ) {
				//we dont have an element at that index;; truncate.
				conflictingIndex = args.PrevLogIndex+i+1
				//rf.lastApplied = args.PrevLogIndex+i

				break;
			}

			if ( rf.log[args.PrevLogIndex+i+1].CommandTerm != args.Term ){
				//found conflicting entry, same index but different term;; truncate.
				conflictingIndex = args.PrevLogIndex+i+1
				//rf.lastApplied = args.PrevLogIndex+i
				break;
			}
			offset = offset+1

		}
		//now replace all the entries starting at conflicting index
		if (conflictingIndex != -1 ){
			for i := 0; i < len(args.LogEntries)-offset; i++ {
				rf.insertEntriesAtIndexInLog(conflictingIndex+i, i+offset, args  )
			}
			DPrintf("follower(%d) log after inserting all entries look=%v\n", rf.me, rf.log )
		}
		if ( args.LeaderCommit > rf.commitIndex ){
			//commitIndex = min ( LeaderCommit, index of last new entry )
			lastind:=len(rf.log)-1
			if (lastind >= args.LeaderCommit){
				rf.commitIndex = args.LeaderCommit
			} else if (args.LeaderCommit > lastind){
				rf.commitIndex = lastind
			}
			rf.apply()

		}
		//if (rf.lastApplied < rf.commitIndex ){
		//	DPrintf()("(me %d) gonna call apply as my lastapplied=%d  cmmitIdx=%d & mylog=%v\n",
		//		rf.me ,rf.lastApplied, rf.commitIndex, rf.log)
		//	rf.apply()
		//}

		if ( rf.state == isLeader ){
			DPrintf("OH FUCK, I was a leader(%d), but now  I have to step down as args.Term(%d) >= rf.currentTerm(%d)\n",
				rf.me, args.Term, rf.currentTerm)
		}
		if ( rf.state == isCandidate ){
			DPrintf("OH FUCK, I was a candidate (%d), but now  I have to step down as args.Term(%d) >= rf.currentTerm(%d)\n",
				rf.me, args.Term, rf.currentTerm)
		}
		rf.state = isFollower


		//if (rf.currentTerm == args.Term ){
		//	DPrintf()(" HELP:: I was gonna reply True after accepting append entries but unfortunately it seems like" +
		//		"I have applied entries assuming wrong term. Help me. \n")
		//}
		DPrintf("I am a follower(%d) and I have accepted AppendEntries\n", rf.me)
		return true

	}else{
		DPrintf("Leader(%d) is running ahead!! args.PrevLogindex=%d and rf.log.len=%d -- I am(%d) not accepting Entries \n",
			args.LeaderId, args.PrevLogIndex, len(rf.log), rf.me )
	}
	DPrintf("I am a follower(%d) and I dont acceptAppendEntries as rf.lastApplied(%d) < args.PrevLogIndex(%d)\n",rf.me,rf.lastApplied, args.PrevLogIndex)
	return false

}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesRequest, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}


//
// example RequestVote RPC handler.
//

func (rf *Raft) RecieveRequestVote(args *RequestVoteArgs, reply *RequestVoteReply){
	rf.mu.Lock()
	defer rf.mu.Unlock()

	//paper section 5.4 describe if my log is more upto date than the candidate, then reject it.
	DPrintf("received request from a candidate (%d) to vote for him. I am = %d and rf.currenterm=%d\n,",
		args.CandidateId, rf.me, rf.currentTerm)
	fmt.Println(args)

	//if ( args.LastLogTerm < rf.log[rf.lastApplied].CommandTerm ){ todo see which one works.
	//if (args.LastLogTerm < rf.currentTerm){
	//	DPrintf()("Not voting because. args.LastLogTerm(%d) < rf.currentTerm(%d)\n", args.LastLogTerm, rf.currentTerm)
	//	reply.Term = rf.currentTerm
	//	reply.VoteGranted = false
	//}


	flag:=0
	if (len(rf.log)>0) {
		fmt.Println("logLength is > 0")

		//checking for more uptodate log
		if ( args.LastLogTerm < rf.log[len(rf.log)-1].CommandTerm )  {
			DPrintf("I (%d) Not voting for Leader(%d) for term=%d, because. because my last command (%d) has term(%d) > leader, last term (%d) \n",
				rf.me,args.CandidateId,args.Term,rf.log[len(rf.log)-1].Command ,rf.log[len(rf.log)-1].CommandTerm ,args.LastLogTerm)
			reply.Term = rf.currentTerm
			reply.VoteGranted = false
			if ( args.Term > rf.currentTerm ){
				rf.currentTerm = args.Term
				rf.votedFor = -1
				rf.state = isFollower
				rf.persist()
			}
			flag=1
			return
		}

		if ( args.LastLogTerm == rf.log[len(rf.log)-1].CommandTerm )&& (len(rf.log)-1 > args.LastLogIndex ) {
			DPrintf("I (%d) Not voting for Leader(%d) for term=%d, because. args.LastLogindex(%d) < last command idx from my log is =%d\n",
				rf.me, args.CandidateId, args.Term, args.LastLogIndex, len(rf.log)-1)
			reply.Term = rf.currentTerm
			reply.VoteGranted = false
			if ( args.Term > rf.currentTerm ){ //reduces time for old terms to catch up
				rf.currentTerm = args.Term
				rf.votedFor = -1
				rf.state = isFollower
				rf.persist()
			}
			flag=1
			return
		}

	}

	//receiver implementation by monster from fig2.
	if ( args.Term < rf.currentTerm ){
		DPrintf("I (%d) Not voting for Leader(%d) for term=%d, because. args.Term(%d) < rf.currentTerm(%d), also votedFor=%d\n",rf.me,args.CandidateId, args.Term, args.Term, rf.currentTerm,rf.votedFor)
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		flag = 1
		return
	}
	if flag == 1 {os.Exit(55)}
	myterm:=rf.currentTerm
	if (args.Term > myterm){
		DPrintf("Good:: args.Term(%d) >= rf.currentTerm(%d)\n", args.Term, rf.currentTerm)
		DPrintf("Should be args.LastLogIndex(%d)>=rf.myLastLogidx (%d): args.LastLogTerm(%d) >= " +
			"rf.currentTerm(%d)\n", args.LastLogIndex, len(rf.log)-1, args.LastLogTerm, rf.currentTerm)
		//if (rf.votedFor != -1 ){
		//if ((args.LastLogIndex >= (len(rf.log)-1)) && (args.LastLogTerm >= rf.currentTerm)){
		//if ( (args.LastLogTerm >= rf.currentTerm)){
		if ( true){


		DPrintf("Gonna vote for him. votedFor = %d \n", rf.votedFor)
			// I have already voted in current term but a new RPC comes with greater term.
			if (args.Term > myterm){
				rf.votedFor = -1  //since I am going in new term,
			}
			if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) {
				rf.currentTerm = args.Term
				reply.VoteGranted = true
				reply.Term = args.Term
				rf.votedFor = args.CandidateId
				rf.genRandTimeout()
				rf.persist()
			} else{
				DPrintf("I am not able to vote  ")
			}
			if (rf.state == isLeader ){
				//todo if it is correct, cuz I am stepping down as a leader while voting for another
				// . There was something mentioned in the paper. read that
				DPrintf("Motherfucker, I am gonna step down. %d on term %d. I was a leader" +
					" in previous term\n", rf.me, rf.currentTerm)

			}
			rf.state = isFollower

			// student guide says to reset when you grant vote
		} else{
			DPrintf("I am not able to vote usama please check here.2 \n")
		}

	}


}

func (rf *Raft) LeaderSendRequestVotes(args *RequestVoteArgs){
	DPrintf(" I am a candidate ( %d), for term=%d and gonna request for votes. \n", rf.me, rf.currentTerm)
	if (rf.state == isCandidate ) {
		rf.numVotes =0
		//vvv:=0
		for i, _ := range rf.peers {
			//if (rf.iscandidate == 1 ) {
			if (i!=rf.me ){
				//kind of resetting the reply for every peer.
				go func(i int, args *RequestVoteArgs ) {


					VotedNeeded:= len(rf.peers)/2
					replyy:=RequestVoteReply{}
					replyy.VoteGranted = false
					replyy.Term = -1

					voteRPCStatus:=false
					voteRPCStatus=rf.sendRequestVote(i, args, &replyy)
					rf.mu.Lock()
					defer rf.mu.Unlock()
					DPrintf("My (%d) voteRequestStatus from %d is = %t\n", rf.me, i, voteRPCStatus )
					if (voteRPCStatus == false ){DPrintf("%d might be dead, or lossy \n", i)}
					if (voteRPCStatus == true) {

						DPrintf("DEBUG:: me=%d, rf.currTerm=%d, reply.term =%d, reply.VoteG=%t cand=%d & votes=%d \n",
							rf.me, rf.currentTerm, replyy.Term, replyy.VoteGranted ,i, rf.numVotes)
						if ( (replyy.VoteGranted == true) && (replyy.Term == args.Term) && (rf.state == isCandidate)&&(replyy.Term == rf.currentTerm)) {

							rf.numVotes = rf.numVotes + 1
							//vvv = vvv +1
							DPrintf("My (%d) Votes are =%d needed=%d\n", rf.me, rf.numVotes ,VotedNeeded)
							if ( rf.numVotes >=VotedNeeded ){
							//if ( vvv >=VotedNeeded ){
								rf.state = isLeader
								rf.leaderid = rf.me
								for i, _ := range rf.peers {
									rf.nextIndex[i] = len(rf.log)
									rf.matchIndex[i] = len(rf.log)-1
								}

								//rf.sendAppendEntriesToAll()

							}

						} else{
							DPrintf("Follower(%d) did not grant me(%d) vote, currentTerm=%d, reply.Term=%d, reply.Vote=%t state=%d.!!! \n",
								i,rf.me, rf.currentTerm, replyy.Term, replyy.VoteGranted, rf.state)
							//todo delete this block above ; it is for debug only
							if (replyy.Term > rf.currentTerm ){
								DPrintf("From Server (%d): reply Term %d > current.Term %d I am stepping down.  !!! \n",
									i, replyy.Term, rf.currentTerm)
								//means there is some else leader, I should step down
								rf.currentTerm = replyy.Term
								rf.state = isFollower
								rf.votedFor = -1



							}

						}

					}

				}(i, args )

			}


		}


	}

}

func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).


	//if (rf.isfollower == 1) {
	//DPrintf()("I (%d) am a follower received \n ", rf.me)
	//fmt.Println(args)
	fmt.Println("----")
	rf.RecieveRequestVote(args,reply)
	//}



	// check if reply than update the term and become follower.
	// if majority of vote, become a leader.
	// todo if received AppendEntries from a new leader, become follower.


	// rf.leaderid = args.CandidateId //I am stepping down as a leader.

	// todo, if Leader:



}


//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}


//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//

//
// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (rf *Raft) Kill() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.terminatme = 5
	// Your code here, if desired.
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//


func (rf * Raft ) genRandTimeout() int {

	// function to generate a random timeout for each peer.
	baseValueElection:=1000//1000

	rand.Seed(time.Now().UnixNano())
	variableValue:=rand.Intn(1000) //1000

	rf.electiontimeout = baseValueElection + variableValue


	DPrintf(" electiontimeout=%d , serverid = %d \n", rf.electiontimeout, rf.me)
	return 0
}


func leaderElection(rf *Raft, me int ){
	for true {
		//DPrintf()("i am leader %d\n", rf.me)

		// if I am a follower todo check if rf.isfollower ==1 should be here or not. Can leader himself timeout and start election?
		rf.mu.Lock()
		statee:=rf.state
		ff:=rf.electiontimeout
		rf.mu.Unlock()
		if (statee!=isLeader){

			if (ff == 0 ){
				DPrintf("I (%d) am becoming candidate and starting election = \n", rf.me)
				//logindex := len(rf.log) // I think you use rf.commitindex etc.
				rf.mu.Lock()
				rf.currentTerm = rf.currentTerm + 1 //increment the term.
				rf.state = isCandidate
				rf.votedFor = me //vote for self.
				rf.genRandTimeout()


				// vote your self before sending out

				requestVoteArgs:=RequestVoteArgs{Term:rf.currentTerm, CandidateId:me,
					LastLogIndex: len(rf.log)-1}
				if ( len(rf.log)>0 ){ //means element exists
					requestVoteArgs.LastLogTerm = rf.log[len(rf.log)-1].CommandTerm
				}
				//DPrintf()("2. I am debugging  %d\n",rf.log[0].CommandIndexInlog)
				rf.mu.Unlock()
				rf.LeaderSendRequestVotes(&requestVoteArgs)
				//rf.RequestVote(&requestVoteArgs,&requestVoteReply)

			}

			time.Sleep(1 * time.Millisecond)
			rf.mu.Lock()
			rf.electiontimeout = rf.electiontimeout - 1
			rf.mu.Unlock()


		}
		rf.mu.Lock()
		if rf.terminatme == 5 {
			DPrintf("ending leaderElection for %d \n", rf.me)
			break
		}
		rf.mu.Unlock()
		//fmt.Println("()()()()()()()")


	}

	// 		becomes leader itself or accept other as a leader.
}
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	index := -1
	term := -1
	isLeader := false
	fmt.Println(command)
	term, isLeader = rf.GetState()
	DPrintf("JJKJKI and state == %v and me =%d \n", isLeader,rf.me )
	if ( isLeader == true ){
		var logentry LogEntry;
		//logentry.CommandIndexInlog = rf.lastApplied + 1
		logentry.Command = command
		logentry.CommandTerm = term
		rf.log=append(rf.log, logentry)
		logentry.CommandIndexInlog = len(rf.log)-1
		index = logentry.CommandIndexInlog+1
		DPrintf("have put command (%d) for rf(%d) len(log)=%d \n", logentry.Command, rf.me, len(rf.log))
		rf.persist()
		fmt.Println(rf.log)
		//os.Exit(6)
		//
		//fmt.Println("GONNA SLEEEP and send ")
		//applymsg:=ApplyMsg{}
		//applymsg.Command = "testcommand"
		//applymsg.CommandIndex = 88
		//applymsg.CommandValid = true
		//fmt.Printf("okay I(%d) am gonna send command = %v, to channel \n",rf.me,applymsg.Command )
		//rf.applychannel<-applymsg
		//
		//time.Sleep(time.Second*4)
		//fmt.Println("SENT*************")

	}



	//todo
	// send parallel AppendRPC and upon majority of replication, update commit index
	// even if it fails, server tries indefinitely (even if it has responded to client )
	// appending an entry to leaders log will update its last log index as a result > nextindex
	// of follower, so start appendRPC.
	// if success increase nextindex and matchindex for that follower
	//else decrease index and try again
	return index, term, isLeader
}

func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.applychannel = applyCh

	rf.genRandTimeout()
	rf.state = isFollower
	rf.votedFor = -1
	rf.leaderid = -1
	rf.lastApplied = -1
	rf.commitIndex = -1
	rf.matchIndex = make([]int, len(rf.peers))
	rf.nextIndex = make([]int, len(rf.peers))

	for i, _ := range rf.nextIndex {
		rf.nextIndex[i]  = 0
		rf.matchIndex[i] = -1
	}
	// Your initialization code here (2A, 2B, 2C).
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	go rf.periodicAppenEntries()
	go leaderElection(rf, rf.me)


	return rf
}

func (rf * Raft)updateCommitIndexInLeader(){

	VotedNeeded:= (len(rf.peers)/2 ) //what about myself?
	currenCommitindex:=rf.commitIndex

	fmt.Println(" Gonna Update Commit Index, let see !!!!!!!!!!!!!!")
	//rf.mu.Lock()
	//defer rf.mu.Unlock()
	fmt.Println(" Got lock for Commit Index, let see !!!!!!!!!!!!!!")

	//	currentAppliedIndex:=rf.lastApplied todo do we need to worry about it here?
	NumVotes:=0
	majorityOnIndex:=true

	for majorityOnIndex {
		fmt.Println(" in Majority looop !!")

		// can be optimzed by starting on commitIndex instead from zero.
		for i, _ := range rf.nextIndex {
			if (rf.matchIndex[i]>currenCommitindex ){
				DPrintf("matchindex[%d]=%d, is gonna vote for currenCommitindex=%d\n", i, rf.matchIndex[i], currenCommitindex+1)
				NumVotes = NumVotes + 1
			}
		}
		DPrintf(" Got some vote in Commit looop !! %d \n", NumVotes)
		//os.Exit(3)

		if (NumVotes >=VotedNeeded && (len(rf.log)>(currenCommitindex + 1))){
			// leader can only update commit index in its own term
			DPrintf("len(log)=%d, currcommit=%d me(%d)\n", len(rf.log), currenCommitindex, rf.me)
			//if (true){
		//		if (rf.log[currenCommitindex + 1].CommandTerm == rf.currentTerm){
				majorityOnIndex = true
			    currenCommitindex = currenCommitindex +1
				NumVotes =0 // to check again for another commit index.
			if (rf.log[currenCommitindex].CommandTerm == rf.currentTerm){
				DPrintf("Got consensus and increased the commitindex  = %d -> %d \n",currenCommitindex-1, currenCommitindex)
				rf.commitIndex = currenCommitindex
			} else {
				DPrintf("Sadly Command(%d) has term(%d) and my ownterm is (%d) so not gonna update commidx=%d\n",
					rf.log[currenCommitindex].Command,rf.log[currenCommitindex].CommandTerm,rf.currentTerm,rf.commitIndex)
			}

		//	} else {
		//		majorityOnIndex = false
		//		DPrintf()("WEIRED :: I have got majority to commit but sadly command does not belong to my term" +
		//			"currentCommitindex = %d,rf.log[currenCommitindex + 1].commandTerm = %d, rf.currentTerm=%d ",
		//			currenCommitindex, rf.log[currenCommitindex + 1].CommandTerm, rf.currentTerm)
		//	}

		} else{
			DPrintf("could not get majority for commitindex = %d\n", currenCommitindex+1)
			majorityOnIndex = false
		}
	}




	if (rf.lastApplied < rf.commitIndex ){
		rf.apply()
	}

}

func (rf * Raft) apply (){


	////rf.mu.Unlock()
	rf.applyMu.Lock()
	//check apply after commit index

	lastAppliedIndex:= rf.lastApplied
	for i:=lastAppliedIndex+1;i<=rf.commitIndex ;i++  {
		applymsg:=ApplyMsg{}
		applymsg.Command = rf.log[i].Command
		applymsg.CommandIndex = i+1
		applymsg.CommandValid = true
		fmt.Printf("okay I(%d) am gonna send command = %v, to channel \n",rf.me,applymsg.Command )

		rf.applychannel<-applymsg
		fmt.Println("SENT COMMAND \n" )
		rf.lastApplied = i
	}
	////if rf.commitIndex == 0 {os.Exit(56)}
	//if it does not work, see if we can put it in go func
	rf.applyMu.Unlock()
}



func (rf * Raft) printlog() {

	DPrintf("gonna print log for peer =%d\n", rf.me  )

	for i, _ := range rf.log {

		DPrintf("command appended = %s, command term = %d, command index in log = %d \n",
			rf.log[i].Command, rf.log[i].CommandTerm, rf.log[i].CommandIndexInlog  )
	}

}


//todo todo Paper says each server will vote at max for one server in one term.
//