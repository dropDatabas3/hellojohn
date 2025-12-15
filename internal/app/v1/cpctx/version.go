package cpctx

import "sync/atomic"

var globalVersion atomic.Uint64

func BumpVersion() uint64 { return globalVersion.Add(1) }
func Version() uint64     { return globalVersion.Load() }
