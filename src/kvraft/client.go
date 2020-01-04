package raftkv

import (
	"fmt"
	"labrpc"
	"os"
)
import "crypto/rand"
import "math/big"


type Clerk struct {
	servers []*labrpc.ClientEnd
	lastSuccessfulServer int
	sequence int
	// You will have to modify this struct.
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// You'll have to add code here.
	return ck
}

//
// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) Get(key string) string {
	args:=GetArgs{Key:key,Id:nrand()}
	for true{
		for i := 0; i < len(ck.servers); i++ {
			replyy:=GetReply{Err:ErrNoKey, Value:""}
			var servertorequest int
			if (i ==0 ){
				//servertorequest = ck.lastSuccessfulServer
				servertorequest=i
			}else{
				servertorequest=i
			}

			ok := ck.servers[servertorequest].Call("KVServer.Get", &args, &replyy)
			if replyy.WrongLeader == false && ok == true {
				ck.lastSuccessfulServer = servertorequest
				if replyy.Err == OK {
					if replyy.Value==""{

					}

					fmt.Printf("server(%d) replied True %v and reply.value==%s...gonna return\n", servertorequest,replyy,replyy.Value)
					return replyy.Value
				}else {
					os.Exit(23)
				}

			} else{
				fmt.Printf("Get did not work for server (%d) .. Reply == %v!!!\n", servertorequest,replyy)
			}
		}
	}
	//todo need to check if we need to incorporate reply.Err in if condition too ;)?
	// You will have to modify this function.
	return ""
}

//
// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) PutAppend(key string, value string, op string) {
	// You will have to modify this function.
	ck.sequence++
	fmt.Println(value)
	args:=PutAppendArgs{Op:op, Key:key, Seq:ck.sequence, Value:value, Id:nrand()}

	//todo probably need to check the Err Value that I have set to ErrNoKey; Not a good approach though.

	for true{
		for i := 0; i < len(ck.servers); i++ {

			var servertorequest int
			if (i ==0 ){
				//servertorequest = ck.lastSuccessfulServer
				servertorequest = i

			}else{
				servertorequest=i
			}

			replyy:=PutAppendReply{ Err:ErrNoKey}
			ok := ck.servers[servertorequest].Call("KVServer.PutAppend", &args, &replyy)
			if replyy.WrongLeader == false && ok == true {
				ck.lastSuccessfulServer = servertorequest
				if replyy.Err == OK {
					fmt.Printf("server(%d) replied True and PutAppendReply=%v\n", servertorequest,replyy)
					return
				}else {
					os.Exit(22)
				}
			} else{
				fmt.Printf("PutAppend did not work for server (%d) .. Reply == %v!!!\n", servertorequest,replyy)
			}
		}
	}


}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}
