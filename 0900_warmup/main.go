// 已就位（AI 生成）· Ch09 并发热身 —— goroutine / channel / select / WaitGroup / Mutex
//
// 这不是「你来实现」，是一个读+跑的热身：全书只有第 9 章用到并发原语，
// 0903 用 Mutex 双锁、0904 用 channel/select/WaitGroup。没手感硬上会很虚，先在这建立直觉。
//
// 用法（build 三拍）：
//  1. 预测：每段函数末尾有「// 预测:」注释，先自己猜输出/答案，别急着跑。
//  2. 跑：  cd db_project && go run ./0900_warmup
//  3. 对照：看真实输出，和你的预测对不上的地方就是你之前的盲区。
//
// 每段末尾标了它直连 c09 的哪一关（→ 0903 / → 0904），跑完就去用。
package main

import (
	"fmt"
	"sync"
	"time"
)

// 1. goroutine：go f() 起一个轻量线程，它和主流程并发跑、互不等待。
func demoGoroutine() {
	done := make(chan struct{})
	go func() {
		fmt.Println("  [goroutine] 我在另一个 goroutine 里跑")
		close(done) // 用 close 当「我干完了」的信号
	}()
	// 预测:如果把下面这行 <-done 删掉,主函数会等这个 goroutine 吗?
	//       (goroutine 不阻塞主流程；不显式等,main 可能在它打印前就结束了)
	<-done
	fmt.Println("  [main] goroutine 已结束,我才继续")
}

// 2. channel：make(chan T, n) 是带缓冲队列。发/收会自然阻塞(不空转 CPU)；close 后收方 range 结束。
func demoChannel() {
	ch := make(chan int, 2) // 缓冲只有 2 个位子
	go func() {
		for i := 1; i <= 3; i++ {
			fmt.Printf("  [发] 准备发 %d\n", i)
			ch <- i // 前两个直接进缓冲；第 3 个缓冲满了,会阻塞等收方腾位子
		}
		close(ch) // 发完必须关,否则收方的 range 永远等下一个、死锁
	}()
	for v := range ch { // close 后循环自动退出
		fmt.Printf("  [收] 收到 %d\n", v)
	}
	// 预测:发到第 3 个时为什么会「卡一下」?如果发完不 close 会怎样?
}

// 3. select：同时等多个 channel，谁先就绪走谁。这就是 0904 后台 compact 线程的骨架。
func demoSelect() {
	data := make(chan int, 1)
	go func() {
		time.Sleep(20 * time.Millisecond) // 20ms 后才有数据
		data <- 42
	}()
	select {
	case v := <-data:
		fmt.Println("  [select] 数据先到:", v)
	case <-time.After(100 * time.Millisecond): // 100ms 超时兜底
		fmt.Println("  [select] 超时了")
	}
	// 预测:data 20ms 到、超时设在 100ms,select 会走哪个 case?
	//       (→ 0904 后台线程正是 select { case <-updated / case <-closing }:谁先来干谁)
}

// 4. WaitGroup：一个计数器。Add 登记、Done 减一、Wait 阻塞到归零——不用 sleep 猜时间。
func demoWaitGroup() {
	var wg sync.WaitGroup
	for i := 1; i <= 3; i++ {
		wg.Add(1) // 每起一个 worker 先登记
		go func(id int) {
			defer wg.Done() // 干完计数减一
			fmt.Printf("  [worker %d] 干活\n", id)
		}(i)
	}
	wg.Wait() // 阻塞,直到 3 个都 Done
	fmt.Println("  [main] 3 个 worker 都干完了")
	// 预测:去掉 wg.Wait() 会怎样?(→ 0904 Close 用 threads.Wait() 等所有后台线程退出)
}

// 5. Mutex：并发「读-改-写」同一个变量必须上锁，否则丢更新。这就是 0903 为什么要 Mutex。
func demoMutex() {
	// 无锁：10 个 goroutine 各 +1000,期望 10000
	var unsafe int
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				unsafe++ // 读→加→写三步不是原子的,并发下互相覆盖 = 丢更新
			}
		}()
	}
	wg.Wait()

	// 有锁：同样的活,Lock/Unlock 之间同一时刻只有一个 goroutine 在改
	var safe int
	var mu sync.Mutex
	var wg2 sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			for j := 0; j < 1000; j++ {
				mu.Lock()
				safe++
				mu.Unlock()
			}
		}()
	}
	wg2.Wait()

	fmt.Printf("  [无锁] unsafe=%d\n", unsafe)
	fmt.Printf("  [有锁] safe=%d\n", safe)
	// 预测:unsafe 会等于 10000 吗?每次跑一样吗?为什么 safe 稳定 10000?
	//       (→ 0903 applyTX 用 mu 锁保护共享的 MemTable,不锁就会像 unsafe 一样脏)
}

func main() {
	fmt.Println("=== 1. goroutine:go f() 起并发,不阻塞主流程 ===")
	demoGoroutine()
	fmt.Println("\n=== 2. channel:带缓冲队列,发/收自然阻塞,close 让收方结束 ===")
	demoChannel()
	fmt.Println("\n=== 3. select:同时等多个 channel,谁先就绪走谁 (→ 0904) ===")
	demoSelect()
	fmt.Println("\n=== 4. WaitGroup:Add 登记 / Done 减一 / Wait 等归零 ===")
	demoWaitGroup()
	fmt.Println("\n=== 5. Mutex:并发读改写要上锁,否则丢更新 (→ 0903) ===")
	demoMutex()
}
