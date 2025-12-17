package kvsrv

// 设计特点
// 可配置的网络环境：支持可靠和不可靠网络模式测试
// 客户端管理：提供创建和删除客户端的功能
// 测试隔离：每个测试可以独立创建和清理资源
// 模块化设计：将测试逻辑与具体实现分离

import (
	// "log"
	"testing"

	"6.5840/kvtest1"
	"6.5840/tester1"
)

type TestKV struct {
	*kvtest.Test         // 组合了 kvtest.Test 类型，继承其测试功能
	t        *testing.T  // 包含 *testing.T 用于测试控制
	reliable bool        // reliable 字段标识网络是否可靠
}

// 创建测试环境的工厂函数
// 使用 tester.MakeConfig 创建配置，支持指定服务器数量和网络可靠性
// 调用 StartKVServer 启动KV服务器
// 返回封装好的 TestKV 实例
func MakeTestKV(t *testing.T, reliable bool) *TestKV {
	cfg := tester.MakeConfig(t, 1, reliable, StartKVServer)
	ts := &TestKV{
		t:        t,
		reliable: reliable,
	}
	ts.Test = kvtest.MakeTest(t, cfg, false, ts)
	return ts
}

// 创建客户端 Clerk 用于与KV服务器交互
// 通过配置创建客户端连接
// 返回封装了实际 clerk 的 TestClerk 实例
func (ts *TestKV) MakeClerk() kvtest.IKVClerk {
	clnt := ts.Config.MakeClient()
	ck := MakeClerk(clnt, tester.ServerName(tester.GRP0, 0))
	return &kvtest.TestClerk{ck, clnt}
}

func (ts *TestKV) DeleteClerk(ck kvtest.IKVClerk) {
	tck := ck.(*kvtest.TestClerk)
	ts.DeleteClient(tck.Clnt)
}
