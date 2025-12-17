1. lab1: 
mapreudece
官方的线性mapreduce通过官方测试脚本，无法生成mr-correct-wc.txt，那么将导致个人程序的测试无法与mr-correct-wc.txt进行比对，从而部分测试FATAL。 我通过主动让mrsequential.go生成mr-correct-wc.txt，依旧FATAL。 我怀疑是不是生成的是my-output.txt。 先搁置。
2. lab2：
Key/value server with reliable network (easy)

kolar2@Kolar:~/6.5840/src/kvsrv1$ go test -v -run Reliable
go: downloading github.com/anishathalye/porcupine v1.0.3
../models1/kv.go:3:8: github.com/anishathalye/porcupine@v1.0.3: Get "https://proxy.golang.org/github.com/anishathalye/porcupine/@v/v1.0.3.zip": dial tcp 142.250.204.49:443: i/o timeout
目前显示代理超时。。。。

依赖关系: Part C 严重依赖于 Part A/B。lock.go 中的代码直接调用 lk.ck.Get() 和 lk.ck.Put()。如果 Clerk 的 Get/Put 方法不能在不可靠网络下可靠工作（例如，不能正确处理重试和 ErrMaybe），那么基于它构建的锁也必然会在网络问题下失效或行为异常。

目的关系: Part A/B 是基础设施建设，旨在解决分布式系统中最基础的通信可靠性问题。Part C 是应用开发，展示了如何利用这个可靠的基础设施去解决更高级别的分布式协调问题。

验证关系: 通过 Part C 的测试（尤其是 Unreliable 测试），反过来验证了 Part A/B 的实现是正确的。如果锁能在不可靠网络下正常工作，说明底层的 Clerk 重试机制、版本控制和错误处理都按预期工作。

简单来说：先做好一个可靠的锤子和钉子（Part A/B），然后用这个锤子去造一个复杂的家具（Part C）。 Part C 的成功与否，直接反映了 Part A/B 的质量。

3. lab3:
raft1.server.go 
这个实现采用了装饰器模式，rfsrv 包装了实际的 Raft 实现，提供了额外的功能如：

日志验证
快照管理
错误处理
测试支持