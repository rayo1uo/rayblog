# Go实现生产者消费者模型

## 1、单生产者和单消费者

实现非常简单，建立一个无缓冲channel，生产者向channel发送数据，消费者读数据，写起来倒是容易出错，一不小心写成下面这样：
```go
pool := make(chan string)
done := make(chan bool)
producer(pool)
consumer(pool, done)
<-done
```
。。。看了半天还没发现问题出在哪里，原来是`producer`函数和`consumer`函数没写成协程的形式，`main`执行主斜程到生产者函数时向无缓冲channel写数据发现没人读取数据就给阻塞住了，最后就deadlock了。改成下面这样就好啦!
```go
go producer(pool)
go consumer(pool, done)
```

## 2、单生产者和多消费者

实现上的改动不大，创建多个消费者就可以啦。这里面又能引出来一个知识点——**多个协程对同一个channel的并发访问时，channel内部是会上锁的**（后面研究go结构源码时再去具体分析，挖坑挖坑wakuwaku~）。

## 3、多生产者和单消费者

创建多个生产者往channel写数据，channel并发写时同样是加锁的，这里需要注意的是一个生产者写完并不能直接关闭channel，否则consumer写成也会直接退出，正确的写法是啥呢？那当然是有主协程来close啦！
```go
wg.Wait()
close(pool)
<-done
```

## 4、多生产者和多消费者

这部分咱就是说代码有点复杂，解释一下。首先定义一个结构体作为生产者，包含三个成员——`myQ`是一个channel，存放消息；`quit`用于退出生产者协程；`id`就是标识啦。
```go
type producer struct {
	myQ  chan string
	quit chan bool
	id   int
}
```
`execute`函数可以看作是一个工作流，源源不断地向`jobQ`这个channel里面写数据，然后交给生产者，模拟生产者从外部获取数据、处理数据再发送给消费者的过程。发送完成之后会关闭这个channel。然后向worker协程发送结束信号，协程终止。
```go
func execute(jobQ chan<- string, workerPool chan *producer) {
	for _, j := range quotes {
		jobQ <- j
	}
	close(jobQ)
	for _, w := range workers {
		w.quit <- true
	}
	close(workerPool)
}
```
咱们继续看看具体的生产者和消费者的功能函数。

生产者监听到有消息了，就将producer指针加到workerPool中，然后将消息通过`myQ`传递给消费者。消费者从workerPool这个channel里面拿到当前消费者指针，从它的消息channel里面再拿到具体的消息进行处理。

另外需要注意的是，这里所有的channel都是无缓冲的，有的读者就会想，workerpool改成有缓冲的channel性能会不会有所提升呢？仔细想想，在多个producer几乎同时往workerpool里面写数据时，多个consumer同时从workerpool拿数据时，由于channel并发写时会加锁，只会有一对生产者消费者建立“握手”，所以性能是的确有优化空间的。

```go
func produce(jobQ <-chan string, p *producer, workerPool chan *producer) {
	for {
		select {
		case msg := <-jobQ:
			{
				workerPool <- p // 如果这个producer接收到了消息就把这个producer的指针加到workerpool里面，workerpool因为也是无缓冲的，因此也会阻塞等待消费者
				if len(msg) > 0 {
					fmt.Printf("Job \"%v\" produced by worker %v\n", msg, p.id)
				}
				p.myQ <- msg // 这里因为是一个无缓冲的channel，因此会阻塞住等待消费者来接受消息
			}
		case <-p.quit:
			return
		}
	}
}

func consume(cIdx int, workerPool <-chan *producer, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		worker, ok := <-workerPool
		if !ok {
			fmt.Printf("consumer %d exits\n", cIdx)
			return
		}
		if msg, ok := <-worker.myQ; ok {
			if len(msg) > 0 {
				fmt.Printf("Message \"%v\" is consumed by consumer %v from worker %v\n", msg, cIdx, worker.id)
			}
		}
	}
}

```
另一种实现多生产者消费者模型的方法是简单使用两个`sync.WaitGroup`，具体看见代码哦。 