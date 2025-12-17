package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//
// 通信机制
// 使用 Unix domain socket 而不是 TCP socket：

// 1. 性能更好（本地通信无需网络协议栈）
// Unix Domain Socket 通信流程：
// 应用程序 → 文件系统 → 内核缓冲区 → 应用程序
//    ↓         ↓           ↓           ↓
//   数据    Unix socket   内存拷贝    接收数据
//           文件节点

// TCP Socket 通信流程：
// 应用程序 → TCP协议栈 → 网络接口 → 环回接口(lo) → TCP协议栈 → 应用程序
//    ↓           ↓           ↓           ↓           ↓           ↓
//   数据      封装TCP包    网络层处理   本地回环    解封TCP包     接收数据
//            添加IP头等                  转发

// 2. 避免端口冲突问题
// 具体优势
// Unix Domain Socket 的优势：
// - 无网络协议开销

// 不需要封装/解封装 TCP/IP 头部
// 不涉及网络路由和寻址
// 没有 checksum 计算等网络校验
// 更低的延迟

// 数据直接在内核中传递
// 避免网络协议栈处理时间
// 更高的吞吐量

// 减少了 CPU 使用
// 更少的内存拷贝操作

// --- 避免端口冲突问题：
// TCP Socket 的端口管理问题：
// TCP Socket 的端口管理问题：

// go
// // 如果使用 TCP，可能遇到的问题：
// listener1, err := net.Listen("tcp", ":8080") // 进程1占用8080
// listener2, err := net.Listen("tcp", ":8080") // 进程2也会尝试占用8080 - 冲突！

// // 或者需要手动管理端口分配：
// listener1, err := net.Listen("tcp", ":8080")
// listener2, err := net.Listen("tcp", ":8081") // 需要记住哪些端口可用
// listener3, err := net.Listen("tcp", ":8082")
// Unix Domain Socket 的解决方案：

// go
// // 使用文件路径作为唯一标识，避免端口冲突
// sockname1 := "/tmp/myapp-socket-1"
// sockname2 := "/tmp/myapp-socket-2"
// listener1, err := net.Listen("unix", sockname1) // 基于文件路径，不会冲突
// listener2, err := net.Listen("unix", sockname2)
import "os"
import "strconv"

//
// example to show how to declare the arguments
// and reply for an RPC.
//

type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

// Add your RPC definitions here.
type RequestTaskArgs struct {
}

type RequestTaskReply struct {
// in /var/tmp, for the coordinator.
	TaskType string  // map/reduce/done/wait
	FileName string
	NReduce int
	NMap int
	TaskID int
}

type ReportTaskArgs struct {
	TaskID int
	TaskType string
}
type ReportTaskReply struct {
}



// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.

// Socket路径生成函数 来确定唯一一条连接的
func coordinatorSock() string {
	// 使用文件路径作为唯一标识，避免端口冲突
	s := "/var/tmp/5840-mr-"
	s += strconv.Itoa(os.Getuid()) // 使用用户 ID 确保不同用户的进程不会冲突
	return s
}
