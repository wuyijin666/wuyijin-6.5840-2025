package main

//
// start the coordinator process, which is implemented
// in ../mr/coordinator.go
//
// go run mrcoordinator.go pg*.txt
//
// Please do not change this file.
//

import "6.5840/mr"
import "time"
import "os"
import "fmt"
import "path/filepath"

func cleanupOldFiles() {
    // 使用filepath.Glob来正确匹配文件
    files, _ := filepath.Glob("mr-*")
    for _, f := range files {
        os.RemoveAll(f)
    }
}
func main() {
	 cleanupOldFiles() // 添加清理

	// 至少包含 输入文件
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: mrcoordinator inputfiles...\n")
		os.Exit(1)
	}

	// 创建协调器实例
	// os.Args[1:] - 传入所有命令行参数（除了程序名）作为输入文件列表
	// 10 - 指定每个worker的超时时间为10秒
	m := mr.MakeCoordinator(os.Args[1:], 10)
	// 轮询检查任务完成状态
	// 循环调用协调器的 Done() 方法检查是否所有任务都已完成。如果未完成，则休眠1秒钟后继续检查。
	for m.Done() == false {
		time.Sleep(time.Second)
	}

	time.Sleep(time.Second)
}

