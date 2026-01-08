package raft

// The file raftapi/raft.go defines the interface that raft must
// expose to servers (or the tester), but see comments below for each
// of these functions for more details.
//
// Make() creates a new raft peer that implements the raft interface.

import (
	//	"bytes"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
	//	"6.5840/labgob"
	"6.5840/labrpc"
	"6.5840/raftapi"
	"6.5840/tester1"
)

type State string
const (
	Leader State = "Leader"
	Candidate State = "Candidate"
	Follower State = "Follower"
)
// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers                        // 存储了集群中所有节点的 RPC 端点，用于节点间通信
	persister *tester.Persister   // Object to hold this peer's persisted state         // 用于保存当前Raft节点的持久化状态
	me        int                 // this peer's index into peers[]                     // 表示当前节点在 peers 数组中的索引位置
	dead      int32               // set by Kill()

	// Your data here (3A, 3B, 3C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	state State

	// Persistent state on all servers
	currentTerm int         // 服务器知道的最近任期，当服务器启动时初始化为0，单调递增
	votedFor   int          // 在当前任期中，该服务器给投过票的candidateId,若没有则-1
	logs map[int]LogEntry	// 日志条目；每一条包含了状态机指令以及该条目被leader收到时的任期号

	// Volatile（易失性）state on all servers
	commitIndex int     // 已知可以安全应用到状态机的最高日志索引
	lastApplied int     // 应用到状态机的最高日志条目索引号

	// Volatile state on leaders
	nextIndex   map[int]int  // 下次要发送给每个 Follower 的日志索引（初始化为leader的最高索引号+1）
	matchIndex map[int]int   // 每个 Follower 已成功复制的最高日志索引（初始化为0，单调递增）

	// 新添加这个字段用于跟踪上次收到领导者消息的时间 ？ 没想明白
	lastReceivedHeartbeat time.Time
}

type RequestVoteArgs struct {
	// Your data here (3A, 3B).
	Term        int  // candidate's term
	CandidateId int  // 发起投票的candidate的ID
	LastLogIndex int // candidate的最高日志条目索引
	LastLogTerm  int // candidate的最高日志条目的任期号
}

type RequestVoteReply struct {
	// Your data here (3A).
	Term        int  // currentTerm，用于candidate更新自己term
	VoteGranted bool // true表示candidate获得投票
}
type LogEntry struct {
	Term int               // 任期
	// 类型为 interface{}，意味着可以存储任意类型的数据；
	// 通常包含客户端请求的具体操作（如键值对存储的PUT/GET操作）；
	// 当日志条目被提交后，这个命令会被应用到状态机在Raft算法中的作用：
	Command interface{}   
} 

// AppendEntries RPC，同时leader发起，用来复制日志条目或者发送心跳
type AppendEntriesArgs struct { 
	Term        int      // leader任期号
	LeaderId    int      // 用来让follower把客户端请求定向到leader
	PrevLogIndex int     // 紧接新条目之前的日志条目索引
	PrevLogTerm  int
	Entries  []LogEntry  // 存储的日志条目（如果某条目是空的，它就是心跳；为了提效，可能会发出不止一条日志）
	LeaderCommit int     // leader's commitIndex
}

type AppendEntriesReply struct {
	Term int          // currentTerm，用于leader更新自己term
	Success bool      // 如果follower包含的日志匹配参数汇总的prevLogIndex和prevLogTerm，返回true
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here (3A).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	term = rf.currentTerm
	isleader = (rf.state == Leader)
	
	return term, isleader
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved(恢复) after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent. 看figure2 ，哪些状态需要做持久化
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// 在你实现快照功能之前，你应该传递nil作为persister.Save()的第二个参数。
// after you've implemented snapshots, pass the current snapshot
// 在你实现快照功能之后，传递当前快照
// (or nil if there's not yet a snapshot).
// （如果没有快照则传递nil）。

// 将持久化状态保存到存储中
func (rf *Raft) persist() {
	// Your code here (3C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// raftstate := w.Bytes()
	// rf.persister.Save(raftstate, nil)
}


// restore previously persisted state.
// 恢复之前保存的状态
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (3C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

// how many bytes in Raft's persisted log?
// 返回持久化日志的大小
func (rf *Raft) PersistBytes() int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.persister.RaftStateSize()
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
// 服务端通知说已经创建了一个快照，该快照包含了直到并包括指定索引的所有信息。
// 这意味着服务端不再需要直到（并包括）该索引的日志。
// Raft现在应该尽可能多地修剪其日志。

// 根据快照索引修剪日志
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (3D).

}
// example RequestVote RPC handler.
// 处理候选者发来的投票请求 
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	
	// 设置回复任期
	reply.Term = rf.currentTerm

	// 如果参数中的任期小于当前任期，拒绝投票
	if args.Term < rf.currentTerm {
		reply.VoteGranted = false
		return 
	}

	// 如果参数中的任期大于当前任期，更新当前任期并转换为跟随者
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.state = Follower
		rf.votedFor = -1
		reply.Term = args.Term
	}

	// 检查是否可以投票给该候选人
	canVote := (rf.votedFor == -1 || rf.votedFor == args.CandidateId) && 
	           rf.isLogUpToDate(args.LastLogIndex, args.LastLogTerm)

	if canVote {
		rf.votedFor = args.CandidateId
		reply.VoteGranted = true
	} else {
		reply.VoteGranted = false
	}
}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[]. 参数 server 是目标服务器在 rf.peers 数组中的索引
// expects RPC arguments in args. 期望通过 args 参数传递 RPC 请求参数
// fills in *reply with RPC reply, so caller should
// pass &reply. 用 RPC 回复填充 reply 指针指向的变量，因此调用者应传递 &reply
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
// 传递给 Call() 的 args 和 reply 的类型必须与处理器函数中声明的参数类型相同（包括是否为指针）
//
// The labrpc package simulates a lossy network, in which servers     labrpc 包模拟了一个不稳定的网络环境，其中服务器可能无法访问，
// may be unreachable, and in which requests and replies may be lost. 请求和回复可能会丢失。
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
// 返回 false 的原因可能是服务器宕机、可访问但无法到达的服务器、丢失的请求或丢失的回复。
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
// Call() 保证会返回（可能会有延迟），除非服务器端的处理函数没有返回。
// 因此不需要在 Call() 周围实现自己的超时机制。
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
// 如果你在使用 RPC 时遇到问题，请检查你是否将通过 RPC 传递的结构体中的所有字段名都大写了，
// 并且调用者传递的是回复结构体的地址（使用 &），而不是结构体本身。

// 向其它服务器发送投票请求
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	reply.Term = rf.currentTerm

	if rf.currentTerm > args.Term {
		reply.Success = false
		return 
	}

	// 如果收到的任期比当前任期大，转换为跟随者
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.state = Follower
		rf.votedFor = -1
		reply.Term = args.Term
	}
	// 重置选举超时计时器，因为我们收到了来自leader的有效消息 
	rf.lastReceivedHeartbeat = time.Now()	
	
	// 2. 如果参数的term和prevLogTerm相等的的日志中不包含prevLogIndex的条目，返回false
	if args.PrevLogIndex > 0 {
		// 检查是否存在prevLogIndex对应的日志条目
		existingEntry, exists := rf.logs[args.PrevLogIndex]
		if !exists || existingEntry.Term != args.PrevLogTerm {
			reply.Success = false
			return 
		}
	}

	// 3. 现有条目与新条目（索引相同但任期不同）发生冲突，删除当前及以后的所有条目
	for i, entry := range args.Entries {
		index := args.PrevLogIndex + i + 1
		if existingEntry, exists := rf.logs[index]; exists {
			// 如果任期不同，删除当前及之后的所有条目
			if existingEntry.Term != entry.Term {
				// 删除从index开始的所有后续条目
				for j := index; ; j++ {
					if _, exists := rf.logs[j]; exists {
						delete(rf.logs, j)
					} else {
						break
					}
				}
			}
		}
		// 4. 添加不在日志中的新条目 
		rf.logs[index] = LogEntry{
			Term:  entry.Term,
			Command: entry.Command,
		}	
	}
	reply.Success = true

	// 5. 如果leaderCommit>commitIndex，设置commitIndex=min(leaderCommit, 上一个新条目的索引)
	if args.LeaderCommit > rf.commitIndex {
		lastNewIndex := args.PrevLogIndex + len(args.Entries)
		rf.commitIndex = min(args.LeaderCommit, lastNewIndex)
	}
}
func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
// 使用 Raft 的服务（例如一个键值服务器）想要开始就下一个要追加到 Raft 日志中的命令达成一致。
// 如果这个服务器不是领导者，则返回 false。否则立即开始达成一致并返回。
// 不能保证这个命令会被提交到 Raft 日志中，因为领导者可能会故障或在选举中失败。
// 即使 Raft 实例已经被关闭，这个函数也应该优雅地返回。

// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
// 第一个返回值是命令被提交时将出现的索引位置。
// 第二个返回值是当前任期。
// 第三个返回值表示这个服务器是否认为自己是领导者。

// 启动对新命令的共识(仅限领导者) - 客户端接口
// 启动共识：让 Raft 集群就一个新命令达成一致
// 仅限领导者：只有 Leader 节点才能接收这个请求，Follower 节点会拒绝
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (3B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// 检查当前服务器是否是领导者
	isLeader = rf.state == Leader
	if !isLeader {
		isLeader = false
		return index, term, isLeader
	}

	// 获取下一个日志索引
	index = len(rf.logs) + 1
	term = rf.currentTerm
	// 创建新的日志条目
	newEntry := LogEntry{
		Term: term,
		Command: command,
	}
	// 将日志添加到当前节点
	rf.logs[index] = newEntry
	rf.persist()

	return index, term, isLeader
}

// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
// 测试器在每次测试后不会停止由 Raft 创建的 goroutine，
// 但会调用 Kill() 方法。你的代码可以使用 killed() 来
// 检查是否调用了 Kill()。使用原子操作避免了使用锁的需要。
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
// 问题是长时间运行的 goroutine 会占用内存并可能消耗 CPU 时间，
// 这可能导致后续测试失败并产生令人困惑的调试输出。
// 任何具有长时间运行循环的 goroutine 都应该调用 killed() 来检查是否应该停止。

// 关闭服务器 - 客户端接口
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired（如果需要的话）. 
}

// 检查服务器是否已关闭 - 客户端接口
func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

// 定期检查选举超时  
func (rf *Raft) ticker() {
	// 初始化选举超时时间
	rf.mu.Lock()
	lastElectionTime := time.Now()
	rf.mu.Unlock()
	
	for rf.killed() == false {
		rf.mu.Lock()
		
		// 检查是否需要开始领导者选举
		elapsed := time.Since(lastElectionTime)
		electionTimeout := time.Duration(300+rand.Intn(300)) * time.Millisecond // 300-600ms
		
		// 如果当前不是Leader，并且已经过了选举超时时间，则开始选举
		if rf.state != Leader && elapsed >= electionTimeout {
			// 转换为候选人并开始选举
			rf.state = Candidate
			rf.currentTerm++
			rf.votedFor = rf.me
			rf.persist()
			
			// 记录开始选举的时间
			lastElectionTime = time.Now()
			
			// 获取当前状态副本
			currentTerm := rf.currentTerm
			lastLogIndex := len(rf.logs)
			var lastLogTerm int
			if lastLogIndex > 0 {
				if entry, ok := rf.logs[lastLogIndex]; ok {
					lastLogTerm = entry.Term
				}
			}
			
			rf.mu.Unlock()
			
			// 发起选举
			rf.requestVotes(currentTerm, lastLogIndex, lastLogTerm)
			
			rf.mu.Lock()
		}
		
		currentState := rf.state
		peersCount := len(rf.peers)
		currentTerm := rf.currentTerm
		commitIndex := rf.commitIndex
		
		if currentState == Leader {
			// 作为Leader发送心跳
			for i := 0; i < peersCount; i++ {
				if i == rf.me {
					continue
				}
				
				// 准备心跳参数
				args := AppendEntriesArgs{
					Term:         currentTerm,
					LeaderId:     rf.me,
					PrevLogIndex: 0,
					PrevLogTerm:  0,
					Entries:     []LogEntry{}, // 空条目代表心跳
					LeaderCommit: commitIndex,
				}
				
				// 异步发送心跳
				go func(server int) {
					var reply AppendEntriesReply
					ok := rf.sendAppendEntries(server, &args, &reply)
					if ok {
						rf.mu.Lock()
						defer rf.mu.Unlock()
						
						// 如果发现更高任期，转换为跟随者
						if reply.Term > rf.currentTerm {
							rf.currentTerm = reply.Term
							rf.state = Follower
							rf.votedFor = -1
							rf.persist()
						}
					}
				}(i)
			}
			
			// 更新 lastElectionTime，以避免 Leader 转换为候选人
			lastElectionTime = time.Now()
		}
		rf.mu.Unlock()

		// pause for a random amount of time between 50 and 350
		// milliseconds.
		// 暂停 50 到 350 毫秒之间的随机时间。
		
		ms := 50 + (rand.Int63() % 300)
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
}

// 候选者发起选举
func (rf *Raft) requestVotes(currentTerm, lastLogIndex, lastLogTerm int) {
	votesReceived := 1 // 投给自己的一票
	peersCount := len(rf.peers)
	
	args := RequestVoteArgs{
		Term:         currentTerm,
		CandidateId:  rf.me,
		LastLogIndex: lastLogIndex,
		LastLogTerm:  lastLogTerm,
	}
	
	for i := 0; i < peersCount; i++ {
		if i == rf.me {
			continue
		}
		
		go func(server int) {
			var reply RequestVoteReply
			ok := rf.sendRequestVote(server, &args, &reply)
			if ok {
				rf.mu.Lock()
				defer rf.mu.Unlock()
				
				if rf.state != Candidate || rf.currentTerm != args.Term {
					return
				}
				
				if reply.Term > rf.currentTerm {
					rf.currentTerm = reply.Term
					rf.state = Follower
					rf.votedFor = -1
					rf.persist()
					return
				}
				
				if reply.VoteGranted {
					votesReceived++
					if votesReceived > peersCount/2 {
						// 成为领导者
						rf.state = Leader
						// 初始化 nextIndex 和 matchIndex
						for j := 0; j < peersCount; j++ {
							if j != rf.me {
								rf.nextIndex[j] = len(rf.logs) + 1
								rf.matchIndex[j] = 0
							}
						}
					}
				}
			}
		}(i)
	}
}

// 重置选举计时器
func (rf *Raft) resetElectionTimer() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.lastReceivedHeartbeat = time.Now()
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
// 服务或测试器想要创建一个 Raft 服务器。所有 Raft 服务器（包括这个）的端口都在 peers[] 中。
// 这个服务器的端口是 peers[me]。所有服务器的 peers[] 数组顺序都相同。
// persister 是这个服务器用来保存其持久化状态的地方，并且最初也保存着最近保存的状态（如果有的话）。
// applyCh 是一个通道，测试器或服务期望 Raft 通过它发送 ApplyMsg 消息。
// Make() 必须快速返回，所以它应该为任何长时间运行的工作启动 goroutine（长时间运行的任务应该使用 goroutine 异步执行）。

// 构造函数，初始化新的Raft节点 - 内部操作 
func Make(peers []*labrpc.ClientEnd, me int,
	persister *tester.Persister, applyCh chan raftapi.ApplyMsg) raftapi.Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (3A, 3B, 3C).
	// 初始化 Raft 结构体的字段
	rf.state = Follower
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.logs = make(map[int]LogEntry)
	rf.logs[0] = LogEntry{Term: 0} // 使用索引0作为起始哨兵
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.nextIndex = make(map[int]int)
	rf.matchIndex = make(map[int]int)
	rf.lastReceivedHeartbeat = time.Now()

	
	// initialize from state persisted before a crash  从崩溃前持久化的状态中初始化
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()

	return rf
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
// 判断日志是否更新到最新
func (rf *Raft) isLogUpToDate(lastLogIndex, lastLogTerm int) bool {
    lastEntryIndex := len(rf.logs)
    var lastEntryTerm int
    if lastEntryIndex > 0 {
        if entry, ok := rf.logs[lastEntryIndex]; ok {
            lastEntryTerm = entry.Term
        }
    }
    
    // 如果任期号更大，或者任期号相同但日志更长
    if lastLogTerm > lastEntryTerm {
        return true
    }
    if lastLogTerm == lastEntryTerm && lastLogIndex >= lastEntryIndex {
        return true
    }
    return false
}