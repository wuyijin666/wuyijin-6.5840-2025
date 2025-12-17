package mr

import "log"
import "net"
import "os"
import "net/rpc"
import "net/http"
import "time"
import "sync"


// 事件驱动架构
// 心跳事件：Worker请求任务
// 报告事件：Worker完成任务
// 系统响应式地处理各种事件

// 核心设计理念
// 1. 单一goroutine处理原则
// 所有对Coordinator状态的修改都在一个goroutine中进行
// 通过channel通信避免直接共享内存访问
// 从根本上消除data race问题
// 2. 事件驱动架构
// Worker Request Task → HeartBeat Channel → Scheduler Goroutine → Process & Update State
// Worker Report Done  → Report Channel    → Scheduler Goroutine → Process & Update State
// 3. 同步机制
//  Worker端：发送请求并等待响应
// func (c *Coordinator) HeartBeat(args, reply) error {
//     msg := heartbeatMsg{reply, make(chan struct{})}
//     c.heartBeatCh <- msg  // 发送请求
//     <-msg.ok              // 等待处理完成
//     return nil
// }

//  Coordinator端：处理请求
// func (c *Coordinator) Schedule() {
//     for {
//         select {
//         case msg := <-c.heartBeatCh:
//             // 处理任务分配逻辑
//             // 更新内部状态
//             msg.ok <- struct{}{}  // 通知处理完成
//         }
//     }
// }
// 设计优势
// 1. 并发安全：只有一个goroutine修改共享数据
// 2. 简化同步：无需复杂的mutex锁机制
// 3. 清晰职责分离：
// RPC handlers只负责消息传递
// Schedule goroutine负责业务逻辑
// 4. 良好的扩展性：易于添加新类型的事件

// 需要注意的实现细节
// 1. Channel缓冲区：考虑是否需要带缓冲的channel以防阻塞
// 2. 超时机制：防止某个消息处理过久影响整体性能
// 3. 错误处理：RPC调用失败时的重试机制
// 4. 资源清理：程序结束时正确关闭channels
// 这是一个典型的"share by communicating"的设计模式，很好地利用了Go的并发特性。

type SchedulePhase int
const (
	MapPhase SchedulePhase = iota
	ReducePhase
	DonePhase
)

type TaskStatus int
const (
	Inited TaskStatus = iota
	Running
	Completed
)
type Coordinator struct {
	// Your definitions here.
	mu sync.Mutex
	files []string      // 输入文件列表
	nReduce int
	nMap int
	phase SchedulePhase // 当前任务调度阶段
	tasks []Task        // 任务列表
	
	// 通信通道
	heartBeatCh chan heartbeatMsg  // 心跳通道 （worker请求任务）
	reportCh chan reportMsg        // 报告通道  (worker完成任务)
	doneCh chan  struct{}          // 完成通道  （系统完成信号）
}

type Task struct {
	fileName string
	startTime time.Time // 用于超时检测
	id int              // 任务id
	status TaskStatus   // 任务状态
}
type heartbeatMsg struct {
	args *RequestTaskArgs
	reply *RequestTaskReply 
	ok chan struct{} 
}
type reportMsg struct {
	args *ReportTaskArgs
	reply *ReportTaskReply 
	ok chan struct{}  
}

// type RequestTaskArgs struct{}
// type RequestTaskReply struct {
//     TaskType string
//     FileName string
//     TaskID   int
//     NReduce  int
//     NMap     int
// }

// Your code here -- RPC handlers for the worker to call
// func (c *Coordinator) RPCHandler(args *FactArgs, reply *FactReply) error {
// }
// RPC handlers for the worker to call
func (c *Coordinator) RequestTask(args *RequestTaskArgs, reply *RequestTaskReply) error {
    msg := heartbeatMsg{
        args:  args,
        reply: reply,
        ok:    make(chan struct{}),
    }
    c.heartBeatCh <- msg
    <-msg.ok
    return nil
}

func (c *Coordinator) ReportTask(args *ReportTaskArgs, reply *ReportTaskReply) error {
    msg := reportMsg{
        args:  args,
        reply: reply,
        ok:    make(chan struct{}),
    }
    c.reportCh <- msg
    <-msg.ok
    return nil
}

// 初始化map阶段
func (c *Coordinator) initMapPhase() { 
	c.phase = MapPhase
	c.tasks = make([]Task, c.nMap)
	for i := 0; i < c.nMap; i++ {
		c.tasks[i] = Task{  // 使用结构体字面量初始化
			fileName:  c.files[i],
			startTime: time.Time{},
			id:        i,
			status:    Inited,
		}
	}
}
// 初始化reduce阶段
func (c *Coordinator) initReducePhase() { 
	c.phase = ReducePhase
	c.tasks = make([]Task, c.nReduce)
	for i := 0; i < c.nReduce; i++ {
		c.tasks[i] = Task{  // 使用结构体字面量初始化
			fileName:  "",
			startTime: time.Time{},
			id:        i,
			status:    Inited,
		}
	}
}
// 实现基于通道的事件驱动调度机制
func (c *Coordinator) Schedule() {
	c.initMapPhase() // 初始任务队列和状态
	// 主调度循环
	for {
		select { // go使用select语句监听多个通道，实现并发事件处理 无阻塞并发
		case msg := <-c.heartBeatCh:
			// 处理心跳请求
			c.handleHeartBeat(msg.args, msg.reply)
			msg.ok <- struct{}{}
		case msg := <-c.reportCh:
			// 处理任务完成报告
			c.handleReport(msg.args, msg.reply)
			msg.ok <- struct{}{}
		} 
	}
}
// 处理心跳请求（任务分配）
func (c *Coordinator) handleHeartBeat(args *RequestTaskArgs, reply *RequestTaskReply) { 
	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.phase {
    case MapPhase:
        // 查找空闲的map任务
        for i := 0; i < c.nMap; i++ {
            if c.tasks[i].status == Inited {
                c.tasks[i].status = Running
                c.tasks[i].startTime = time.Now()
                
                reply.TaskType = "map"
                reply.FileName = c.tasks[i].fileName
                reply.TaskID = c.tasks[i].id
                reply.NReduce = c.nReduce
                reply.NMap = c.nMap
                return
            }
			// 检查超时的Running任务
            if c.tasks[i].status == Running && time.Since(c.tasks[i].startTime) > 10*time.Second {
                c.tasks[i].status = Inited  // 重置为Inited以便重新分配
                // 递归调用重新查找任务
                c.handleHeartBeat(args, reply)
                return
            }
        }
		// 检查是否所有map任务都已完成
        allCompleted := true
        for i := 0; i < c.nMap; i++ {
            if c.tasks[i].status != Completed {
                allCompleted = false
                break
            }
        }
        if allCompleted {
            // 如果所有map任务已完成，初始化reduce阶段
            c.initReducePhase()
            // 递归调用自己处理reduce任务分配
            c.handleHeartBeat(args, reply)
            return
        }
        // 没有空闲任务，让worker等待
        reply.TaskType = "wait"
        
    case ReducePhase:
        // 查找空闲的reduce任务
        for i := 0; i < c.nReduce; i++ {
            if c.tasks[i].status == Inited {
                c.tasks[i].status = Running
                c.tasks[i].startTime = time.Now()
                
                reply.TaskType = "reduce"
                reply.TaskID = c.tasks[i].id
                reply.NReduce = c.nReduce
                reply.NMap = c.nMap
                return
            }
			// 检查超时的Running任务
            if c.tasks[i].status == Running && time.Since(c.tasks[i].startTime) > 10*time.Second {
                c.tasks[i].status = Inited  // 重置为Inited以便重新分配
                // 递归调用重新查找任务
                c.handleHeartBeat(args, reply)
                return
            }
        }
		allCompleted := true
        for i := 0; i < c.nReduce; i++ {
            if c.tasks[i].status != Completed {
                allCompleted = false
                break
            }
        }
        if allCompleted { 
            c.phase = DonePhase
            reply.TaskType = "done"
            return
        }
        // 没有空闲任务，让worker等待
        reply.TaskType = "wait"
        
    case DonePhase:
        reply.TaskType = "done"
    }

}
// 处理任务完成报告
func (c *Coordinator) handleReport(args *ReportTaskArgs, reply *ReportTaskReply) { 
	c.mu.Lock()
	defer c.mu.Unlock()

	switch args.TaskType {
    case "map":
		if  args.TaskID >= 0 && args.TaskID < c.nMap {
			c.tasks[args.TaskID].status = Completed
		}
		// 检查是否所有map任务完成
		allCompleted := true
		for i := 0; i < c.nMap; i++ {
			if c.tasks[i].status != Completed {
				allCompleted = false
				break
			}
		}
		// 若map任务都已完成，进入reduce阶段
		if allCompleted {
			c.initReducePhase()
		}
    case "reduce":
		if  args.TaskID >= 0 && args.TaskID < c.nReduce {
			c.tasks[args.TaskID].status = Completed
		}
		// 检查是否所有reduce任务完成
		allCompleted := true
		for i := 0; i < c.nReduce; i++ {
			if c.tasks[i].status != Completed {
				allCompleted = false
				break
			}
		}

		if allCompleted {
			c.phase = DonePhase
		}
	}
}

//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}
//
// start a thread that listens for RPCs from worker.go
// 启动一个线程来监听来自 worker.go 的 RPC 请求。
//
func (c *Coordinator) server() {
	rpc.Register(c)  // 注册RPC服务
	rpc.HandleHTTP() // 设置HTTP服务器
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()  // 使用 Unix Domain Socket 监听连接
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}
//
// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
// main/mrcoordinator.go 会定期调用 Done() 方法，以检查整个作业是否已经完成。

// 任务完成检查函数
func (c *Coordinator) Done() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	ret := false

	// Your code here.
	ret = c.phase == DonePhase
	return ret
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//

// 协调器创建函数，并启动服务
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	 cleanupOldFiles() // 添加这行

	c := Coordinator{
		files:      files,
		nMap:       len(files),
		nReduce:    nReduce,
		phase:      MapPhase,
		heartBeatCh: make(chan heartbeatMsg, 10),
		reportCh:    make(chan reportMsg, 10),
		doneCh:      make(chan struct{}),	
	}

	// Your code here.
	
	// 启动后台调度协程
	go c.Schedule()

	c.server()
	return &c
}
func cleanupOldFiles() {
    // 清理旧的中间文件
    os.RemoveAll("mr-*")
    os.RemoveAll("mr-out-*")
}
 