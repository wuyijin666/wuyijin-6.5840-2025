package raftapi

// The Raft interface
type Raft interface {
	// Start agreement on a new log entry, and return the log index
	// for that entry, the term, and whether the peer is the leader.

// 功能：启动对新日志条目的共识过程
// 返回值：
// 第一个 int：该条目在日志中的索引
// 第二个 int：当前任期号
// bool：表示该节点是否为领导者
	Start(command interface{}) (int, int, bool)

	// Ask a Raft for its current term, and whether it thinks it is
	// leader
	
// 功能：获取 Raft 节点的当前状态
// 返回值：
// int：当前任期号
// bool：是否认为自己是领导者
	GetState() (int, bool)

	// For Snaphots (3D)
// 功能：处理快照相关操作（Lab 3D）
// 参数：
// index：快照包含的最新日志条目的索引
// snapshot：快照数据
	Snapshot(index int, snapshot []byte)
	PersistBytes() int

	// For the tester to indicate to your code that is should cleanup
	// any long-running go routines.
	// 功能：通知 Raft 节点清理长时间运行的 goroutine
	Kill()
}

// As each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the server (or
// tester), via the applyCh passed to Make(). Set CommandValid to true
// to indicate that the ApplyMsg contains a newly committed log entry.
//
// In Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
// 这个结构体用于从 Raft 层向应用层传递已提交的消息。
// 应用层根据 CommandValid 或 SnapshotValid 处理不同类型的已提交消息

// Raft 实现在达成共识后通过 applyCh 通道发送 ApplyMsg
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}
