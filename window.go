package circuit

import (
	"container/ring"
	"sync"
	"time"
)

var (
	DefaultWindowTime    = time.Millisecond * 10000
	DefaultWindowBuckets = 10
)

// bucket holds counts of failures and successes
type bucket struct {
	failure int64
	success int64
}

// Reset resets the counts to 0
func (b *bucket) Reset() {
	b.failure = 0
	b.success = 0
}

// Fail increments the failure count
func (b *bucket) Fail() {
	b.failure += 1
}

// Sucecss increments the success count
func (b *bucket) Success() {
	b.success += 1
}

// window maintains a ring of buckets and increments the failure and success
// counts of the current bucket. Once a specified time has elapsed, it will
// advance to the next bucket, reseting its counts. This allows the keeping of
// rolling statistics on the counts.
type window struct {
	buckets    *ring.Ring
	bucketTime time.Duration
	bucketLock sync.RWMutex
	lastAccess time.Time
}

// NewWindow creates a new window. windowTime is the time covering the entire
// window. windowBuckets is the number of buckets the window is divided into.
// An example: a 10 second window with 10 buckets will have 10 buckets covering
// 1 second each.
func NewWindow(windowTime time.Duration, windowBuckets int) *window {
	buckets := ring.New(windowBuckets)
	for i := 0; i < buckets.Len(); i++ {
		buckets.Value = &bucket{}
		buckets = buckets.Next()
	}

	bucketTime := time.Duration(windowTime.Nanoseconds() / int64(windowBuckets))
	return &window{buckets: buckets, bucketTime: bucketTime, lastAccess: time.Now()}
}

// Fail records a failure in the current bucket.
func (w *window) Fail() {
	var b *bucket
	w.bucketLock.Lock()
	defer w.bucketLock.Unlock()

	b = w.buckets.Value.(*bucket)

	if time.Since(w.lastAccess) > w.bucketTime {
		w.buckets = w.buckets.Next()
		b = w.buckets.Value.(*bucket)
		b.Reset()
	}
	w.lastAccess = time.Now()

	b.Fail()
}

// Success records a success in the current bucket.
func (w *window) Success() {
	var b *bucket
	w.bucketLock.Lock()
	defer w.bucketLock.Unlock()

	b = w.buckets.Value.(*bucket)

	if time.Since(w.lastAccess) > w.bucketTime {
		w.buckets = w.buckets.Next()
		b = w.buckets.Value.(*bucket)
		b.Reset()
	}
	w.lastAccess = time.Now()

	b.Success()
}

// Failures returns the total number of failures recorded in all buckets.
func (w *window) Failures() int64 {
	w.bucketLock.RLock()
	defer w.bucketLock.RUnlock()

	var failures int64
	w.buckets.Do(func(x interface{}) {
		b := x.(*bucket)
		failures += b.failure
	})
	return failures
}

// Successes returns the total number of successes recorded in all buckets.
func (w *window) Successes() int64 {
	w.bucketLock.RLock()
	defer w.bucketLock.RUnlock()

	var successes int64
	w.buckets.Do(func(x interface{}) {
		b := x.(*bucket)
		successes += b.success
	})
	return successes
}

// ErrorRate returns the error rate calculated over all buckets, expressed as
// a floating point number (e.g. 0.9 for 90%)
func (w *window) ErrorRate() float64 {
	w.bucketLock.RLock()
	defer w.bucketLock.RUnlock()

	var total int64
	var failures int64

	w.buckets.Do(func(x interface{}) {
		b := x.(*bucket)
		total += b.failure + b.success
		failures += b.failure
	})

	if total == 0 {
		return 0.0
	}

	return float64(failures) / float64(total)
}

// Reset resets the count of all buckets.
func (w *window) Reset() {
	w.bucketLock.Lock()
	defer w.bucketLock.Unlock()

	w.buckets.Do(func(x interface{}) {
		x.(*bucket).Reset()
	})
}
