# Go限流器

Golang标准库自带限流器实现`golang.org/x/time/rate`，该限流器基于令牌桶算法，包含两个参数bucket和rate：
+ bucket参数控制令牌桶的大小，即令牌桶最多能够放多少个令牌
+ rate参数控制将令牌放入到令牌桶的速率，例如表示每秒放入令牌桶的令牌的数量

所谓令牌桶算法，也就是每来一个请求，都需要从令牌桶拿出一个令牌，如果令牌桶中没有令牌了，那么这个请求就不会被处理或者阻塞等待直到拿到一个令牌之后才会被处理。

令牌桶算法可以较好地应对突发流量，具体来说，如果系统大部分时间都没有请求，短时间内有大量请求，那么此时令牌桶内的所有令牌短时都会被消耗掉来应对突发流量。

## 源码剖析

Limiter的结构体定义如下。它是并发安全的，其中limit参数即令牌桶令牌的放入速率，burst参数即令牌桶的大小。tokens表示令牌桶目前剩余的令牌数量，可以为负数，负数表示某些原子请求了较大数量的令牌

```go
type Limiter struct {
	mu     sync.Mutex
	limit  Limit
	burst  int
	tokens float64
	// last is the last time the limiter's tokens field was updated
	last time.Time
	// lastEvent is the latest time of a rate-limited event (past or future)
	lastEvent time.Time
}
```

Limiter包含三个关键的方法，分别是`Allow`, `Reserve`和`Wait`，三个方法的共同点都是从令牌桶中消耗一个令牌，区别在于它们在令牌桶中没有令牌的处理方式：
+ `Allow`: 令牌消耗完后直接返回false
+ `Reserve`: 令牌消耗完后会返回一个`reservation`对象表示提前预定一个令牌，同时返回拿到令牌需要等待的时间
+ `Wait`：令牌消耗完后会阻塞直到拿到令牌

下面具体看一下这个三个方法的实现。

首先是`Allow`与`Reserve`方法。可以看到实际是调用`reserveN`方法，那么我们继续将目光投入到`reserveN`方法的实现。

```go
// Allow reports whether an event may happen now.
func (lim *Limiter) Allow() bool {
	return lim.AllowN(time.Now(), 1)
}

// AllowN reports whether n events may happen at time t.
// Use this method if you intend to drop / skip events that exceed the rate limit.
// Otherwise use Reserve or Wait.
func (lim *Limiter) AllowN(t time.Time, n int) bool {
	return lim.reserveN(t, n, 0).ok
}

// Reserve is shorthand for ReserveN(time.Now(), 1).
func (lim *Limiter) Reserve() *Reservation {
	return lim.ReserveN(time.Now(), 1)
}
```

在`ReserveN`方法中包含一个很重要的结构体`Reservation`，这里我们首先看一下它的定义：它表示允许事件在一段时间之后处理的一个凭证，或者说它给的是一个承诺，告诉请求者在当前令牌桶令牌消耗完的情况下，请求多久能够拿到令牌从而被处理。`tokens`参数表示请求的令牌数量，`ok`表示Limiter是否能在最大等待时间内返回请求数量的令牌，`timeToAct`表示拿到预计多久能拿到令牌。

```go
// A Reservation holds information about events that are permitted by a Limiter to happen after a delay.
// A Reservation may be canceled, which may enable the Limiter to permit additional events.
type Reservation struct {
	ok        bool
	lim       *Limiter
	tokens    int
	timeToAct time.Time
	// This is the Limit at reservation time, it can change later.
	limit Limit
}
```

关于`reserveN`方法的解析见代码中的注释，这里着重理解一下代码中对t小于last的处理逻辑。

可以看到这里的`reserveN`是并发安全的，考虑两个goroutine同时并发请求这个方法，此时goroutine 1拿到了limiter的锁，开始处理，gouroutine 2没拿到锁等待，此时两个goroutine传进去的时间戳都为t1；假设gouroutine 2处理完成之后更新了limiter的last字段表示更新tokens的时间为t2，此时t1 < t2。接下来gouroutine 2拿到锁，advance方法中last被更新为t1，此时计算出的delta为0，表示增量的令牌数为0（预支给了goroutine 1），返回的令牌数也就是当前时刻令牌桶的存量令牌数。

```go
// reserveN is a helper method for AllowN, ReserveN, and WaitN.
// maxFutureReserve specifies the maximum reservation wait duration allowed.
// reserveN returns Reservation, not *Reservation, to avoid allocation in AllowN and WaitN.
func (lim *Limiter) reserveN(t time.Time, n int, maxFutureReserve time.Duration) Reservation {
	lim.mu.Lock()
	defer lim.mu.Unlock()

    // 处理边界条件，令牌桶的令牌生成速率为无穷大，默认允许所有的请求；令牌桶的令牌生成速率为0，令牌数量不够时拒绝所有的请求
	if lim.limit == Inf {
		return Reservation{
			ok:        true,
			lim:       lim,
			tokens:    n,
			timeToAct: t,
		}
	} else if lim.limit == 0 {
		var ok bool
		if lim.burst >= n {
			ok = true
			lim.burst -= n
		}
		return Reservation{
			ok:        ok,
			lim:       lim,
			tokens:    lim.burst,
			timeToAct: t,
		}
	}

	t, tokens := lim.advance(t)  // t传入的是当前时间戳，返回的tokens是当前系统的令牌总数

	// Calculate the remaining number of tokens resulting from the request.
	tokens -= float64(n) // 计算减去请求的令牌数后剩余的令牌数

	// Calculate the wait duration
	var waitDuration time.Duration
	if tokens < 0 {
        // 计算生成不够的令牌所需要的时间
		waitDuration = lim.limit.durationFromTokens(-tokens)
	}

	// Decide result
    // OK为true只有在请求令牌数小于令牌桶的size&&等待时间小于指定的最大等待时间maxFutureReserve
	ok := n <= lim.burst && waitDuration <= maxFutureReserve

	// Prepare reservation
	r := Reservation{
		ok:    ok,
		lim:   lim,
		limit: lim.limit,
	}
	if ok {
		r.tokens = n
		r.timeToAct = t.Add(waitDuration)

		// Update state
		lim.last = t
		lim.tokens = tokens
		lim.lastEvent = r.timeToAct
	}

	return r
}

// advance calculates and returns an updated state for lim resulting from the passage of time.
// lim is not changed.
// advance requires that lim.mu is held.
func (lim *Limiter) advance(t time.Time) (newT time.Time, newTokens float64) {
	last := lim.last  // tokens字段上一次被更新的时间
	if t.Before(last) { // 如果传入的时间戳是上一次更新的时间之前，设置last为传入的时间，后面生成的令牌数计算就为0；这里需要注意一下就是为什么会传入一个过去的时间&传入过去的时间会发生什么？
		last = t
	}

	// Calculate the new number of tokens, due to time that passed.
	elapsed := t.Sub(last) // 自从tokens字段更新经过了多长的时间
	delta := lim.limit.tokensFromDuration(elapsed) // 计算这段时间生成的令牌总量
	tokens := lim.tokens + delta // 更新当前总的令牌数
	if burst := float64(lim.burst); tokens > burst {  // 如果总令牌数量超过了令牌桶的大小，设置为令牌桶的size
		tokens = burst
	}
	return t, tokens  // 返回传入的时间戳，和当前的token总量
}
```

返回`Reservation`对象后，调用方可以调用`DelayFrom`方法获取拿到请求数量的令牌需要等待的时间，也可以通过Cancel方法取消请求，这里重点看看`CancelAt`方法的实现。这里不太清楚为什么需要减去最近一次reserve消费与当前timeToAct经历时间所预支的令牌数，感觉只需要归还当前请求的token数目即可，似乎是一个bug? 恢复令牌数后更新当前令牌桶的令牌数以及最近一次更新的时间为t，并更新lastEvent时间戳。

```go
// DelayFrom returns the duration for which the reservation holder must wait
// before taking the reserved action.  Zero duration means act immediately.
// InfDuration means the limiter cannot grant the tokens requested in this
// Reservation within the maximum wait time.
func (r *Reservation) DelayFrom(t time.Time) time.Duration {
	if !r.ok {
		return InfDuration
	}
	delay := r.timeToAct.Sub(t)
	if delay < 0 {
		return 0
	}
	return delay
}

// CancelAt indicates that the reservation holder will not perform the reserved action
// and reverses the effects of this Reservation on the rate limit as much as possible,
// considering that other reservations may have already been made.
func (r *Reservation) CancelAt(t time.Time) {
	if !r.ok { // 如果ok为false，啥也不做
		return
	}

	r.lim.mu.Lock()
	defer r.lim.mu.Unlock()

    // 处理边界条件，没请求token
	if r.lim.limit == Inf || r.tokens == 0 || r.timeToAct.Before(t) {
		return
	}

	// calculate tokens to restore
	// The duration between lim.lastEvent and r.timeToAct tells us how many tokens were reserved
	// after r was obtained. These tokens should not be restored.
    // r.tokens表示此次消费的token数，r.timeToAct表示令牌桶可以满足本次消费数目的时刻，r.lim.lastEvent表示最近一次调用reserve的timeToAct值
	restoreTokens := float64(r.tokens) - r.limit.tokensFromDuration(r.lim.lastEvent.Sub(r.timeToAct))
	if restoreTokens <= 0 {
		return
	}
	// advance time to now
	t, tokens := r.lim.advance(t)
	// calculate new number of tokens
	tokens += restoreTokens
	if burst := float64(r.lim.burst); tokens > burst {
		tokens = burst
	}
	// update state
	r.lim.last = t
	r.lim.tokens = tokens
	if r.timeToAct == r.lim.lastEvent {
		prevEvent := r.timeToAct.Add(r.limit.durationFromTokens(float64(-r.tokens)))
		if !prevEvent.Before(t) {
			r.lim.lastEvent = prevEvent
		}
	}
}
```

最后是`WaitN`和`Wait`方法了，可以看到这两个方法都是调用`wait`方法。
```go
// Wait is shorthand for WaitN(ctx, 1).
func (lim *Limiter) Wait(ctx context.Context) (err error) {
	return lim.WaitN(ctx, 1)  // 实际调用WaitN方法
}

// WaitN blocks until lim permits n events to happen.
// It returns an error if n exceeds the Limiter's burst size, the Context is
// canceled, or the expected wait time exceeds the Context's Deadline.
// The burst limit is ignored if the rate limit is Inf.
func (lim *Limiter) WaitN(ctx context.Context, n int) (err error) {
	// The test code calls lim.wait with a fake timer generator.
	// This is the real timer generator.
	newTimer := func(d time.Duration) (<-chan time.Time, func() bool, func()) {
		timer := time.NewTimer(d)
		return timer.C, timer.Stop, func() {}
	}
    // 调用lim.Wait
	return lim.wait(ctx, n, time.Now(), newTimer)
}

// wait is the internal implementation of WaitN.
func (lim *Limiter) wait(ctx context.Context, n int, t time.Time, newTimer func(d time.Duration) (<-chan time.Time, func() bool, func())) error {
	lim.mu.Lock()
	burst := lim.burst
	limit := lim.limit
	lim.mu.Unlock()

	if n > burst && limit != Inf { // 处理边界条件，也就是请求令牌数大于令牌桶令牌数并且生成令牌的速率不是无穷大的
		return fmt.Errorf("rate: Wait(n=%d) exceeds limiter's burst %d", n, burst)
	}
	// Check if ctx is already cancelled
	select {
	case <-ctx.Done(): // 如果context取消了或超时了则直接退出
		return ctx.Err()
	default:
	}
	// Determine wait limit
	waitLimit := InfDuration
	if deadline, ok := ctx.Deadline(); ok {
		waitLimit = deadline.Sub(t)
	}
	// Reserve
    // 如果预留失败，则返回Error；否则执行后面的逻辑，阻塞等待
	r := lim.reserveN(t, n, waitLimit)
	if !r.ok {
		return fmt.Errorf("rate: Wait(n=%d) would exceed context deadline", n)
	}
	// Wait if necessary
	delay := r.DelayFrom(t)
	if delay == 0 {
		return nil
	}
	ch, stop, advance := newTimer(delay)
	defer stop()
	advance() // only has an effect when testing
	select {
	case <-ch:
		// We can proceed.
		return nil
	case <-ctx.Done():
		// Context was canceled before we could proceed.  Cancel the
		// reservation, which may permit other events to proceed sooner.
		r.Cancel()
		return ctx.Err()
	}
}
```



