package lock

import (
	"fmt"
	//	"log"
	"strconv"
	"testing"
	"time"

	"6.5840/kvsrv1"
	"6.5840/kvsrv1/rpc"
	"6.5840/kvtest1"
)
// 核心思想：

// 测试通过启动多个并发客户端，让它们都尝试获取同一个锁（key="l"），
// 然后在持有锁的情况下，对另一个共享资源（key="l0"）进行读-改-写操作。
// 如果锁实现了互斥，那么任何时候，最多只能有一个客户端成功读取到 "l0" 的旧值为空 ("")，然后写入自己的标识。
// 如果有两个客户端同时读到空值并写入，就说明锁失效了。

// 每个客户端都围绕一个共享锁 "l" 和一个共享变量 "l0" 工作。
// 客户端循环执行：获取锁 -> 检查 l0 是否空 -> 写入自己ID到 l0 -> 休眠 -> 清空 l0 -> 释放锁。

const (
	NACQUIRE = 10  // 每个客户端尝试获取锁的次数（实际上代码里是无限循环）
	NCLNT    = 10  // 并发测试的客户端数量
	NSEC     = 2   // 测试运行的总时间（秒）
)

// oneClient 是单个测试客户端执行的函数
// me: 客户端编号
// ck: 该客户端使用的 Clerk (IKVClerk 接口)
// done: 一个 channel，当测试结束时会被关闭，通知客户端停止
// 返回值: kvtest.ClntRes，包含客户端执行的操作计数等信息（这里主要关心操作次数）
func oneClient(t *testing.T, me int, ck kvtest.IKVClerk, done chan struct{}) kvtest.ClntRes {
	lk := MakeLock(ck, "l")
	ck.Put("l0", "", 0)
	for i := 1; true; i++ {
		select {
		case <-done:
			return kvtest.ClntRes{i, 0}
		default:
			lk.Acquire()

			// log.Printf("%d: acquired lock", me)

			b := strconv.Itoa(me)
			val, ver, err := ck.Get("l0")
			if err == rpc.OK {
				if val != "" {
					t.Fatalf("%d: two clients acquired lock %v", me, val)
				}
			} else {
				t.Fatalf("%d: get failed %v", me, err)
			}

			err = ck.Put("l0", string(b), ver)
			if !(err == rpc.OK || err == rpc.ErrMaybe) {
				t.Fatalf("%d: put failed %v", me, err)
			}

			time.Sleep(10 * time.Millisecond)

			err = ck.Put("l0", "", ver+1)
			if !(err == rpc.OK || err == rpc.ErrMaybe) {
				t.Fatalf("%d: put failed %v", me, err)
			}

			// log.Printf("%d: release lock", me)

			lk.Release()
		}
	}
	return kvtest.ClntRes{}
}

// Run test clients
func runClients(t *testing.T, nclnt int, reliable bool) {
	ts := kvsrv.MakeTestKV(t, reliable)
	defer ts.Cleanup()

	ts.Begin(fmt.Sprintf("Test: %d lock clients", nclnt))

	ts.SpawnClientsAndWait(nclnt, NSEC*time.Second, func(me int, myck kvtest.IKVClerk, done chan struct{}) kvtest.ClntRes {
		return oneClient(t, me, myck, done)
	})
}

func TestOneClientReliable(t *testing.T) {
	runClients(t, 1, true)
}

func TestManyClientsReliable(t *testing.T) {
	runClients(t, NCLNT, true)
}

func TestOneClientUnreliable(t *testing.T) {
	runClients(t, 1, false)
}

func TestManyClientsUnreliable(t *testing.T) {
	runClients(t, NCLNT, false)
}
