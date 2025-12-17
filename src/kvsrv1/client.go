package kvsrv

import (
	"6.5840/kvsrv1/rpc"
	"6.5840/kvtest1"
	"6.5840/tester1"
)


type Clerk struct {
	clnt   *tester.Clnt
	server string
}

func MakeClerk(clnt *tester.Clnt, server string) kvtest.IKVClerk {
	ck := &Clerk{clnt: clnt, server: server}
	// You may add code here.

	return ck
}

// Get fetches the current value and version for a key.  It returns
// ErrNoKey if the key does not exist. It keeps trying forever in the
// face of all other errors.
//
// You can send an RPC with code like this:
// ok := ck.clnt.Call(ck.server, "KVServer.Get", &args, &reply)
//
// The types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. Additionally, reply must be passed as a pointer.
func (ck *Clerk) Get(key string) (string, rpc.Tversion, rpc.Err) {
	// You will have to modify this function.
	args := rpc.GetArgs{Key: key}

	for {
		var reply rpc.GetReply
		ok := ck.clnt.Call(ck.server, "KVServer.Get", &args, &reply)
		if ok {
			// 成功收到服务器回复，无论成功还是失败，都返回结果
			// 根据协议，Get 只有在键不存在时才返回 ErrNoKey，其他错误需要重试
			// 但这里的任务是返回服务器的回复，所以直接返回 reply.Err
			return reply.Value, reply.Version, reply.Err
		} else {
			// 未收到回复 (可能是超时或网络问题)，记录日志并重试
			log.Printf("Get(%v) failed to receive reply, will retry...\n", key)
			// 根据提示，等待一段时间再重试
			time.Sleep(100 * time.Millisecond)
		}
		// 循环继续重试
	}
}

// Put updates key with value only if the version in the
// request matches the version of the key at the server.  If the
// versions numbers don't match, the server should return
// ErrVersion.  If Put receives an ErrVersion on its first RPC, Put
// should return ErrVersion, since the Put was definitely not
// performed at the server. If the server returns ErrVersion on a
// resend RPC, then Put must return ErrMaybe to the application, since
// its earlier RPC might have been processed by the server successfully
// but the response was lost, and the Clerk doesn't know if
// the Put was performed or not.
//
// You can send an RPC with code like this:
// ok := ck.clnt.Call(ck.server, "KVServer.Put", &args, &reply)
//
// The types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. Additionally, reply must be passed as a pointer.
func (ck *Clerk) Put(key, value string, version rpc.Tversion) rpc.Err {
	// You will have to modify this function.
	args := rpc.PutArgs{
		Key: key,
		Value: value,
		Version: version
	}
	isFirstAttempt := true // 标记是否为首次尝试
	for { 
		var reply rpc.PutReply 

		// 发送 RPC 调用
		ok := ck.clnt.Call(ck.server, "KVServer.Put", &args, &reply)
		if ok {
			if reply.Err == rpc.ErrVersion {
				// 服务器返回 ErrVersion
				if isFirstAttempt {
					// 首次调用就收到 ErrVersion -> 确定未执行
					return rpc.ErrVersion
				} else {
					// 重传时收到 ErrVersion -> 可能已执行 (响应丢失)
					return rpc.ErrMaybe
				}
			} else {
				// 服务器返回 OK 或其他非 ErrVersion 错误 -> 直接返回
				// (对于 Put 来说，主要是 OK，因为 ErrNoKey 是 Get 或特定 Put 情况下的)
				return reply.Err // 通常是 rpc.OK
			}
		} else {
			// 未收到回复 (可能是超时或网络问题)
			// 标记后续尝试为重传
			isFirstAttempt = false
			// 根据提示，等待一段时间再重试
			time.Sleep(100 * time.Millisecond)
		}
		// 循环继续重试
	}
}
