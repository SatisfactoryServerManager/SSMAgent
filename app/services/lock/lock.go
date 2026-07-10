package lock

import "sync"

// Server guards the Satisfactory install directory and its process.
//
// The task executor holds it for the duration of every task. Background tickers
// (AutoRestart, backup, save restore) must TryLock and skip when it is held:
// an auto-restart during an install, or a backup of a half-written install dir,
// is exactly the class of bug this exists to prevent.
//
// It is a plain sync.Mutex and therefore NOT reentrant. Nothing reachable from a
// task handler may take it, because the executor already holds it.
var Server = &sync.Mutex{}

// TryServer reports whether the lock was acquired. The caller must Unlock on true.
func TryServer() bool {
	return Server.TryLock()
}
