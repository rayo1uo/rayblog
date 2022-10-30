package main

import (
	"fmt"
	"sync"
)

var quotes = []string{
	"你要尽全力保护你的梦想。那些嘲笑你的人，他们必定会失败，他们想把你变成和他们一样的人。如果你有梦想的话，就要努力去实现。",
	"如果做人没有梦想，那跟咸鱼有什么分别？",
	"死亡不是生命的终点，遗忘才是。",
	"星星在哪里都是很亮的，就看你有没有抬头去看他们。",
	"不管前方的路有多苦，只要走的方向正确，不管多么崎岖不平，都比站在原地更加接近幸福。",
	"只要你肯领略，就会发现人生本是多么可爱，每个季节里有很多足以让你忘记所有烦恼的赏心乐趣。",
	"开拓视野，冲破艰险，看见世界，身临其境，贴近彼此，感受生活，这就是生活的目的。",
	"我们读诗写诗，非为它的灵巧。我们读诗写诗，因为我们是人类的一员。而人类充满了热情。医药，法律，商业，工程，这些都是高贵的理想，并且是维生的必需条件。但是诗，美，浪漫，爱，这些才是我们生存的原因。",
	"如果你不出去走走，你就会以为这就是全世界。",
	"如果我不顾一切发挥每一点潜能去做会怎样？我必须做到，我别无选择。",
	"你真正是谁并不重要，重要的是你的所做所为。",
	"有信心不一定会成功，没信心一定不会成功。",
	"决定我们成为什么样人的，不是我们的能力，而是我们的选择。",
	"有时候你只需要花二十秒，疯狂地一鼓作气。仅仅花上二十秒，鼓起勇气，即便有多尴尬。然后我向你保证，会有好事发生的。",
}

const producerCount int = 4
const consumerCount int = 4

var workers []*producer

type producer struct {
	myQ  chan string
	quit chan bool
	id   int
}

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

func main() {
	jobQ := make(chan string)
	workerPool := make(chan *producer) // 一个包含消费者的pool
	var wg sync.WaitGroup

	for i := 0; i < producerCount; i++ {
		workers = append(workers, &producer{
			myQ:  make(chan string),
			quit: make(chan bool),
			id:   i,
		})
		go produce(jobQ, workers[i], workerPool)
	}

	go execute(jobQ, workerPool)

	for i := 0; i < consumerCount; i++ {
		wg.Add(1)
		go consume(i, workerPool, &wg)
	}
	wg.Wait()
}
