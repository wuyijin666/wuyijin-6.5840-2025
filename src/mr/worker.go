package mr

import (
	"fmt"
	"net/rpc"
	"hash/fnv"
	"io/ioutil"
	"os"
	"encoding/json"
	"sort"
	"time"
	"strconv"
)


//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}
// for sorting by key.
type ByKey []KeyValue

// for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
// 分区策略：通过 ihash() 函数确保相同的键总是被分配到同一个 Reduce 任务
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}
//
// main/mrworker.go calls this function.
//
 
// 实现提示：
// 1. 循环请求任务（通过RPC调用coordinator)
// 2. 执行分配的map / reduce 函数
// 3. 处理中间文件读写
// 4. 向coordinator 报告任务完成状态
// 5. 处理失败/错误情况
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {
	// Your worker implementation here.
	// uncomment to send the Example RPC to the coordinator.
	// CallExample()
	for { 
		// 1. 向协调器去请求任务
		requestArgs := RequestTaskArgs{}
		var reply RequestTaskReply

		ok := call("Coordinator.RequestTask", &requestArgs, &reply)
	    if !ok  {
			time.Sleep(time.Second) // 等待coordinator重启
            continue // 继续尝试连接
		} 
		// 根据coordinator 的返回参数任务类型，处理任务
		// 2. 根据任务类型去处理任务（Map / Reduce）
		switch  reply.TaskType {
		case "map":
			// map task 
			doMapTask(mapf, &reply)
			reportTaskCompletion("map", reply.TaskID)
		case "reduce":
			// reduce task
			doReduceTask(reducef, &reply)
			reportTaskCompletion("reduce", reply.TaskID)
	    case "done": 
			return
	 	case "wait":	
			time.Sleep(time.Second)
        default:
			fmt.Printf("Unexpected task type: %v\n", reply.TaskType)
			time.Sleep(time.Second)
		}
	}
    return
}

//
// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
//
// CallExample()：演示如何向 coordinator 发送 RPC 请求的示例函数
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.Example", &args, &reply)
	if ok {
		// reply.Y should be 100.
		fmt.Printf("reply.Y %v\n", reply.Y)
	} else {
		fmt.Printf("call failed!\n")
	}
}

//
// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
// call()：底层的 RPC 调用函数，负责与 coordinator 通信。使用 Unix domain socket 连接 coordinator
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		// log.Fatal("dialing:", err)
		return false
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}

func doMapTask(mapf func(string, string) []KeyValue, reply *RequestTaskReply) {
	intermediate := []KeyValue{}

	// 打开并读取文件
	file, err := os.Open(reply.FileName)
	if err != nil {
		fmt.Printf("Cannot open %v, skipping task\n", reply.FileName)
        return
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Printf("Cannot read %v, skipping task\n", reply.FileName)
        file.Close()
        return
	}
	file.Close() 

	// call mapf
	kva := mapf(reply.FileName, string(content))
	intermediate = append(intermediate, kva...)

	// Partition intermediate keys into buckets on hash
	buckets := make([][]KeyValue, reply.NReduce)
	for i := range buckets {
		buckets[i] = []KeyValue{}
    }
	for _, kva := range intermediate {
		bucketIndex := ihash(kva.Key) % reply.NReduce
		buckets[bucketIndex] = append(buckets[bucketIndex], kva)
	}
	// 写入中间文件 "mr-{mapID}-{reduceID}"
	for i := range buckets{
		oname := "mr-" + strconv.Itoa(reply.TaskID) + "-" + strconv.Itoa(i)
		ofile, _ := ioutil.TempFile("", oname+"*")
		enc := json.NewEncoder(ofile)
		for _, kva := range buckets[i] {
			err := enc.Encode(&kva)
			if err != nil {
				fmt.Printf("Cannot write into: %v, skipping\n", oname)
                ofile.Close()
                os.Remove(ofile.Name())
                break
			}
		}
		os.Rename(ofile.Name(), oname) 
		ofile.Close()
	}	
	fmt.Printf("Map task %d completed\n", reply.TaskID)
}

func doReduceTask(reducef func(string, []string) string, reply *RequestTaskReply) {
	// collect k-v from intermediate files "mr-*-{reduceID}"
	intermediate := []KeyValue{}
	for i := 0; i < reply.NMap; i++ {
		iname := "mr-" + strconv.Itoa(i) + "-" + strconv.Itoa(reply.TaskID)
		// open and read files
		ifile, err := os.Open(iname)
		if err != nil { //文件可能不存在，正常的，继续处理下一个文件
			 continue
		}
		dec := json.NewDecoder(ifile)
		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				break
			}
			intermediate = append(intermediate, kv)
		}
		ifile.Close()
	}
	// sort by key
	sort.Sort(ByKey(intermediate))

	// output file
    oname := "mr-out-" + strconv.Itoa(reply.TaskID)
    ofile, _ := ioutil.TempFile("", oname+"*")

        i := 0
        for i < len(intermediate) { 
            // find all values for the same key
            j := i + 1
            for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
                j++
            }
            // collect all values for this key
            values := []string{}
            for k := i; k < j; k++ {
                values = append(values, intermediate[k].Value)
            }
            output := reducef(intermediate[i].Key, values)
            fmt.Fprintf(ofile, "%v %v\n", intermediate[i].Key, output)
            i = j
        }
    
	os.Rename(ofile.Name(), oname)
	ofile.Close()
	fmt.Printf("Reduce task %d completed\n", reply.TaskID)
}

func reportTaskCompletion(taskType string, taskID int) {
	for {
        reply := ReportTaskReply{}
        args := ReportTaskArgs{
            TaskType: taskType,
            TaskID:   taskID,
        }
        ok := call("Coordinator.ReportTask", &args, &reply)
        if ok {
            break // 成功报告，退出循环
        }
        fmt.Printf("Failed to report completion of %s task %d, retrying...\n", taskType, taskID)
        time.Sleep(time.Second) // 等待coordinator重启后再重试
    }
    fmt.Printf("Successfully reported completion of %s task %d\n", taskType, taskID)

}
