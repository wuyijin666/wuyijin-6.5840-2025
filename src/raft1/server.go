package raft

import (
	"bytes"
	"fmt"
	"log"
	"sync"

	"6.5840/labgob"
	"6.5840/labrpc"
	"6.5840/raftapi"
	"6.5840/tester1"

)

const (
	SnapShotInterval = 10
)

var useRaftStateMachine bool // to plug in another raft besided raft1


type rfsrv struct {
	ts          *Test   // 测试实例
	me          int     // 服务器ID
	applyErr    string  // from apply channel readers // 应用通道读取错误
	lastApplied int     // 最后应用的日志索引
	persister   *tester.Persister // 持久化存储

	mu   sync.Mutex
	raft raftapi.Raft   // Raft实例
	logs map[int]any    // copy of each server's committed entries // 已提交条目副本
}

func newRfsrv(ts *Test, srv int, ends []*labrpc.ClientEnd, persister *tester.Persister, snapshot bool) *rfsrv {
	//log.Printf("mksrv %d", srv)
	s := &rfsrv{
		ts:        ts,
		me:        srv,
		logs:      map[int]any{},
		persister: persister,
	}
	applyCh := make(chan raftapi.ApplyMsg)
	if !useRaftStateMachine {
		s.raft = Make(ends, srv, persister, applyCh)
	}
	// 支持两种模式 ：普通与快照模式 ？
	if snapshot {
		snapshot := persister.ReadSnapshot()
		if snapshot != nil && len(snapshot) > 0 {
			// mimic KV server and process snapshot now.
			// ideally Raft should send it up on applyCh...
			err := s.ingestSnap(snapshot, -1)
			if err != "" {
				tester.AnnotateCheckerFailureBeforeExit("failed to ingest snapshot", err)
				ts.t.Fatal(err)
			}
		}
		go s.applierSnap(applyCh)
	} else {
		go s.applier(applyCh)
	}
	return s
}

func (rs *rfsrv) Kill() {
	//log.Printf("rs kill %d", rs.me)
	rs.mu.Lock()
	rs.raft = nil // tester will call Kill() on rs.raft
	rs.mu.Unlock()
	if rs.persister != nil {
		// mimic KV server that saves its persistent state in case it
		// restarts.
		// 正确处理 Raft 实例的清理和持久化状态的保存
		raftlog := rs.persister.ReadRaftState()
		snapshot := rs.persister.ReadSnapshot()
		rs.persister.Save(raftlog, snapshot)
	}
}

func (rs *rfsrv) GetState() (int, bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.raft.GetState()
}

// 返回底层的 Raft 接口实例
func (rs *rfsrv) Raft() raftapi.Raft {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.raft
}

// 提交对已提交日志条目的访问
func (rs *rfsrv) Logs(i int) (any, bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	v, ok := rs.logs[i]
	return v, ok
}

// applier reads message from apply ch and checks that they match the log
// contents
// 处理普通的ApplyMsg消息
func (rs *rfsrv) applier(applyCh chan raftapi.ApplyMsg) {
	for m := range applyCh {
		if m.CommandValid == false {
			// ignore other types of ApplyMsg
		} else {
			err_msg, prevok := rs.ts.checkLogs(rs.me, m)
			if m.CommandIndex > 1 && prevok == false {
				err_msg = fmt.Sprintf("server %v apply out of order %v", rs.me, m.CommandIndex)
			}
			if err_msg != "" {
				tester.AnnotateCheckerFailureBeforeExit("apply error", err_msg)
				log.Fatalf("apply error: %v", err_msg)
				rs.applyErr = err_msg
				// keep reading after error so that Raft doesn't block
				// holding locks...
			}
		}
	}
}

// periodically snapshot raft state
// 处理快照的ApplyMsg消息
func (rs *rfsrv) applierSnap(applyCh chan raftapi.ApplyMsg) {
	if rs.raft == nil {
		return // ???
	}

	for m := range applyCh {
		err_msg := ""
		if m.SnapshotValid {
			err_msg = rs.ingestSnap(m.Snapshot, m.SnapshotIndex)
		} else if m.CommandValid {
			if m.CommandIndex != rs.lastApplied+1 {
				err_msg = fmt.Sprintf("server %v apply out of order, expected index %v, got %v", rs.me, rs.lastApplied+1, m.CommandIndex)
			}

			if err_msg == "" {
				var prevok bool
				err_msg, prevok = rs.ts.checkLogs(rs.me, m)
				if m.CommandIndex > 1 && prevok == false {
					err_msg = fmt.Sprintf("server %v apply out of order %v", rs.me, m.CommandIndex)
				}
			}

			rs.lastApplied = m.CommandIndex

			if (m.CommandIndex+1)%SnapShotInterval == 0 {
				w := new(bytes.Buffer)
				e := labgob.NewEncoder(w)
				e.Encode(m.CommandIndex)
				var xlog []any
				for j := 0; j <= m.CommandIndex; j++ {
					xlog = append(xlog, rs.logs[j])
				}
				e.Encode(xlog)
				start := tester.GetAnnotateTimestamp()
				rs.raft.Snapshot(m.CommandIndex, w.Bytes())
				details := fmt.Sprintf(
					"snapshot created after applying the command at index %v",
					m.CommandIndex)
				tester.AnnotateInfoInterval(start, "snapshot created", details)
			}
		} else {
			// Ignore other types of ApplyMsg.
		}
		if err_msg != "" {
			tester.AnnotateCheckerFailureBeforeExit("apply error", err_msg)
			log.Fatalf("apply error: %v", err_msg)
			rs.applyErr = err_msg
			// keep reading after error so that Raft doesn't block
			// holding locks...
		}
	}
}

// returns "" or error string
// 快照管理
func (rs *rfsrv) ingestSnap(snapshot []byte, index int) string {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if snapshot == nil {
		tester.AnnotateCheckerFailureBeforeExit("failed to ingest snapshot", "nil snapshot")
		log.Fatalf("nil snapshot")
		return "nil snapshot"
	}
	r := bytes.NewBuffer(snapshot)
	d := labgob.NewDecoder(r)
	var lastIncludedIndex int
	var xlog []any
	if d.Decode(&lastIncludedIndex) != nil ||
		d.Decode(&xlog) != nil {
		text := "failed to decode snapshot"
		tester.AnnotateCheckerFailureBeforeExit(text, text)
		log.Fatalf("snapshot decode error")
		return "snapshot Decode() error"
	}
	if index != -1 && index != lastIncludedIndex {
		err := fmt.Sprintf("server %v snapshot doesn't match m.SnapshotIndex", rs.me)
		return err
	}
	rs.logs = map[int]any{}
	for j := 0; j < len(xlog); j++ {
		rs.logs[j] = xlog[j]
	}
	rs.lastApplied = lastIncludedIndex
	return ""
}
