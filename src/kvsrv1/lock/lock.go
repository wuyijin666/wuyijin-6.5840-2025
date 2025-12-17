package lock

import (
	"6.5840/kvtest1"
)

type Lock struct {
	// IKVClerk is a go interface for k/v clerks: the interface hides
	// the specific Clerk type of ck but promises that ck supports
	// Put and Get.  The tester passes the clerk in when calling
	// MakeLock().
	ck kvtest.IKVClerk
	// You may add code here
	l string // lock state
	clientId string // 客户端唯一标识
}

// The tester calls MakeLock() and passes in a k/v clerk; your code can
// perform a Put or Get by calling lk.ck.Put() or lk.ck.Get().
//
// Use l as the key to store the "lock state" (you would have to decide
// precisely what the lock state is).
func MakeLock(ck kvtest.IKVClerk, l string) *Lock {
	lk := &Lock{
		ck: ck,
		l: l,
		clientId: kvtest.RandValue(8),
	}
	// You may add code here
	
	return lk
}

func (lk *Lock) Acquire() {
	// Your code here
	for {
		// 1. Read the current lock state (value and version)
		currentValue, currentVersion, getErr := lk.ck.Get(lk.l)

		// 2. Check if the lock is free or already held by us
		if currentValue == "" || currentValue == lk.clientId {
			// Lock is free OR we already hold it (e.g., due to retry after lost response)
			// Attempt to acquire/take ownership by putting our clientId

			// 3. Try to Put our clientId with the version we just read
			putErr := lk.ck.Put(lk.l, lk.clientId, currentVersion)

			if putErr == kvtest.OK {
				// Successfully acquired the lock
				return
			}
		}
		// If currentValue is someone else's clientId, we just need to wait and retry.

		// 4. Wait for a short interval before retrying to avoid busy-waiting
		time.Sleep(100 * time.Millisecond)
		// Loop continues to retry getting and putting
	}
}

func (lk *Lock) Release() {
	// Your code here
	// 1. Read the current lock state
	currentValue, currentVersion, getErr := lk.ck.Get(lk.l)
	// Again, Get should handle retries internally.

	// 2. Check if we are the owner
	if currentValue == lk.clientId {
		// 3. We own the lock, release it by setting the value to empty string ("")
		_ = lk.ck.Put(lk.l, "", currentVersion) //将锁 key 的值设置为空字符串 ("")，表示释放锁
	}
	// If currentValue != lk.clientId, we don't own the lock, releasing is a no-op
	// according to typical lock semantics if misused.
}
