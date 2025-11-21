package main

import "sync"

type Task func()

type Pool struct {
	taskCh chan Task
	wg     sync.WaitGroup
}

func NewPool(workerCount int) *Pool {
	p := &Pool{
		taskCh: make(chan Task, 100), // 缓冲区可调
	}

	for i := 0; i < workerCount; i++ {
		go func() {
			for task := range p.taskCh {
				task()
				p.wg.Done()
			}
		}()
	}

	return p
}

func (p *Pool) Submit(task Task) {
	//print(task)
	p.wg.Add(1)
	p.taskCh <- task
}

func (p *Pool) Wait() {
	p.wg.Wait()
}

func (p *Pool) Close() {
	close(p.taskCh)
}
