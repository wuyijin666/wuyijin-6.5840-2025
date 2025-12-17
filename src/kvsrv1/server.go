package kvsrv

import (
	"log"
	"sync"

	"6.5840/kvsrv1/rpc"
	"6.5840/labrpc"
	"6.5840/tester1"
)

const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

// KeyValue 结构体用于存储值和版本号
type KeyValue struct {
	Value   string
	Version rpc.Tversion
}

type KVServer struct {
	mu sync.Mutex

	// Your definitions here.
	data map[string]KeyValue // 存储键值对及其版本号
}

func MakeKVServer() *KVServer {
	kv := &KVServer{
		data: make(map[string]KeyValue), // 初始化 map
	}
	// Your code here.
	return kv
}

// Get returns the value and version for args.Key, if args.Key
// exists. Otherwise, Get returns ErrNoKey.
func (kv *KVServer) Get(args *rpc.GetArgs, reply *rpc.GetReply) {
	// Your code here.
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if kvdata, ok := kv.data[args.Key] {
		if ok {
			reply.Value = kvdata.Value
			reply.Version = kvdata.Version
			reply.Err = rpc.OK
		} else {
			reply.Value = ""
			reply.Version = 0
			reply.Err = rpc.ErrNoKey
		}
	} 
}

// Update the value for a key if args.Version matches the version of
// the key on the server. If versions don't match, return ErrVersion.
// If the key doesn't exist, Put installs the value if the
// args.Version is 0, and returns ErrNoKey otherwise.
func (kv *KVServer) Put(args *rpc.PutArgs, reply *rpc.PutReply) {
	// Your code here.
	kv.mu.Lock()
	defer kv.mu.Unlock()

	currentKV, exists := kv.data[args.Key]

	if exists {
		// 键已存在
		if args.Version == currentKV.Version {
			// 版本号匹配，执行更新
			kv.data[args.Key] = KeyValue{
				Value:   args.Value,
				Version: currentKV.Version + 1, // 版本号递增
			}
			reply.Err = rpc.OK
		} else {
			// 版本号不匹配，返回 ErrVersion
			reply.Err = rpc.ErrVersion
		}
	} else {
		// 键不存在
		if args.Version == 0 {
			// 版本号为 0，允许创建新键
			kv.data[args.Key] = KeyValue{
				Value:   args.Value,
				Version: 1, // 新键的初始版本号为 1
			}
			reply.Err = rpc.OK
		} else {
			// 版本号不为 0，但键不存在，返回 ErrNoKey
			reply.Err = rpc.ErrNoKey
		}
	}
}

// You can ignore Kill() for this lab
func (kv *KVServer) Kill() {
}


// You can ignore all arguments; they are for replicated(复制) KVservers
func StartKVServer(ends []*labrpc.ClientEnd, gid tester.Tgid, srv int, persister *tester.Persister) []tester.IService {
	kv := MakeKVServer()
	return []tester.IService{kv}
}
