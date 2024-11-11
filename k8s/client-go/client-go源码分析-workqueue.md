# client-go源码分析-workqueue

在前面我们整体分析了client-go的总体架构图，有这样的关键流程，informer从DeltaFIFO取出资源对象变更的事件，按照变更事件的类型执行注册的ResourceEventHandler方法，但是如果我们直接在回调函数中处理这些变更事件的话是比较慢的，为了提高事件处理性能，常用的做法是将资源对象变更事件放入到队列中，然后起若干个协程去并发地处理这些数据，这样可以大大加快变更事件的处理速度。这里其实类似于Go里面的channel，但是workqueue队列提供了更多的功能，比如限速功能、延时加入等。

client-go里面提供的workqueue是一个工具类，在client-go项目的`util/workqueue`目录下面。除了编写operator中可以引用workqueue包，在写其他的项目中我们也可以直接import使用，避免重复造轮子。

## 通用队列

看一下通用队列的接口定义，包括常见的队列操作：插入元素、取出元素、获取队列长度、关闭队列等，接口定义中也使用了Go1.18中引入的泛型新特性，队列元素可以是任意可比较的类型。

```go
// util/workqueue/queue.go
type TypedInterface[T comparable] interface {
	Add(item T)
	Len() int
	Get() (item T, shutdown bool)
	Done(item T)
	ShutDown()
	ShutDownWithDrain()
	ShuttingDown() bool
}

```

接下来看下通用队列是如何实现这些接口的，通用队列结构体的定义和其实现的Add/Get方法如下所示。通用队列包含两个集合dirty/processing和一个底层存储Queue，Queue是一个可自行实现的结构体、决定入队元素处理的先后顺序。Add添加元素时，先将元素加入到dirty集合中，同时，将元素加入到Queue中；Get获取元素时，从Queue中获取一个元素，并将该元素从dirty集合删除、加入到processing集合中。详细总结一下添加元素时的各种情形：
1. 如果dirty集合中包含该元素且processing集合不包含该元素，则调用Queue的Touch方法（比如说对于重复入队可以在Touch方法中提高该入队元素的优先级）
2. 如果dirty集合中包含该元素且processing集合包含该元素，直接返回
3. 如果dirty集合中不包含该元素且processing集合包含该元素，将入队元素加入到dirty集合中
4. 如果dirty集合不包含该元素且processing集合不包含该元素，将元素加入到底层Queue和dirty集合中
   
其中，情形2和情形3或许让人感到些许的疑惑，其实在后面看Done方法的实现时可以看到，processing集合中的元素在处理完成调用Done方法时，会检查dirty集合是否包含该元素，如果包含该元素，会直接将元素加入到Queue中。

```go
type Typed[t comparable] struct {
	// queue defines the order in which we will work on items. Every
	// element of queue should be in the dirty set and not in the
	// processing set.
	queue Queue[t] // queue定义入队元素的处理顺序，要求queue中的每个元素都在dirty集合、不在processing集合中(表示待处理的元素)

	// dirty defines all of the items that need to be processed.
	dirty set[t]

	// Things that are currently being processed are in the processing set.
	// These things may be simultaneously in the dirty set. When we finish
	// processing something and remove it from this set, we'll check if
	// it's in the dirty set, and if so, add it to the queue.
	processing set[t]

	cond *sync.Cond

	shuttingDown bool // 标识队列是否已关闭
	drain        bool

	metrics queueMetrics[t]  // 队列监控指标

	unfinishedWorkUpdatePeriod time.Duration
	clock                      clock.WithTicker
}

// Add marks item as needing processing.
func (q *Typed[T]) Add(item T) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	if q.shuttingDown { // 如果队列已经关闭了，直接返回，不再加入到队列中
		return
	}
	if q.dirty.has(item) { // 如果dirty集合中包含该元素，processing集合不包含该元素，则调用底层Queue的Touch方法（如重设优先级）
		// the same item is added again before it is processed, call the Touch
		// function if the queue cares about it (for e.g, reset its priority)
		if !q.processing.has(item) {
			q.queue.Touch(item)
		}
		return
	}

	q.metrics.add(item)

	q.dirty.insert(item) // 将新加元素放入到dirty集合中
	if q.processing.has(item) { // 如果新加元素在processing集合中，返回
		return
	}

	q.queue.Push(item) // 如果新加元素不在processing集合中，将元素放入到底层queue存储中
	q.cond.Signal()
}

// Get blocks until it can return an item to be processed. If shutdown = true,
// the caller should end their goroutine. You must call Done with item when you
// have finished processing it.
func (q *Typed[T]) Get() (item T, shutdown bool) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	for q.queue.Len() == 0 && !q.shuttingDown { // 如果底层的queue元素数量为0且队列没有关闭，Get方法会阻塞住直到被再次唤醒
		q.cond.Wait()
	}
	if q.queue.Len() == 0 { // 唤醒后如果满足条件，说明队列被关闭了，直接返回shutdown为true
		// We must be shutting down.
		return *new(T), true
	}

	item = q.queue.Pop() // 从底层queue获取一个元素

	q.metrics.get(item)

    // 将这个元素从dirty集合中删除并加入到processing集合中
	q.processing.insert(item)
	q.dirty.delete(item)

	return item, false
}
```

这里再看一下通用队列结构体定义里面queue的类型是Queue[T]，你可能会好奇为什么这里还要新定义一个类型。在client-go先前版本的实现中，queue的定义为`queue []T`，其实就是一个FIFO队列。这里在后续的[commit](https://github.com/kubernetes/kubernetes/pull/123347)对此进行了优化，扩展了底层的存储结构：workqueue的使用者可以提供自定义的数据结构从而自行决定入队元素的处理顺序。

```go
// Queue is the underlying storage for items. The functions below are always
// called from the same goroutine.
type Queue[T comparable] interface {
	// Touch can be hooked when an existing item is added again. This may be
	// useful if the implementation allows priority change for the given item.
	Touch(item T)
	// Push adds a new item.
	Push(item T)
	// Len tells the total number of items.
	Len() int
	// Pop retrieves an item.
	Pop() (item T)
}

// DefaultQueue is a slice based FIFO queue.
func DefaultQueue[T comparable]() Queue[T] { // 返回默认的Queue实现，FIFO队列
	return new(queue[T])
}

// queue is a slice which implements Queue.
type queue[T comparable] []T

func (q *queue[T]) Touch(item T) {}

func (q *queue[T]) Push(item T) {
	*q = append(*q, item)
}

func (q *queue[T]) Len() int {
	return len(*q)
}

func (q *queue[T]) Pop() (item T) {
	item = (*q)[0]

	// The underlying array still exists and reference this object, so the object will not be garbage collected.
	(*q)[0] = *new(T)
	*q = (*q)[1:]

	return item
}
```

接下来看一下通用队列的Done方法实现。

```go
// Done marks item as done processing, and if it has been marked as dirty again
// while it was being processed, it will be re-added to the queue for
// re-processing.
func (q *Typed[T]) Done(item T) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	q.metrics.done(item)

	q.processing.delete(item)
	if q.dirty.has(item) {
		q.queue.Push(item)
		q.cond.Signal()
	} else if q.processing.len() == 0 {
		q.cond.Signal()
	}
}

```

## 小任务

阅读完workqueue的源代码，修复client-go workqueue子目录单测里面的deprecated函数。