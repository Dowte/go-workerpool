package go_workerpool

type Job interface {
	Run(worker Worker)
}

// execute process
type Worker struct {
	onSuccessChan chan<- Worker    // worker执行完成通知 Channel
	jobExitChan   chan<- Job       // job执行完成通知 Channel
	jobChannel    chan Job         // 新job分配 Channel
	quit          chan interface{} // 退出信号
	no            int              // worker编号
}

//创建一个新worker
func NewWorker(successChan chan Worker, no int, jobExitChan chan Job) Worker {
	return Worker{
		jobExitChan:   jobExitChan,
		onSuccessChan: successChan,
		jobChannel:    make(chan Job),
		quit:          make(chan interface{}),
		no:            no,
	}
}

//循环  监听任务和结束信号
func (w Worker) Start() {
	go func() {
		for {
			select {
			case job := <-w.jobChannel:
				// 收到新的job任务 开始执行
				job.Run(w)
				w.jobExitChan <- job
				w.onSuccessChan <- w
			case <-w.quit:
				// 收到退出信号
				return
			}
		}
	}()
}

// 停止信号
func (w Worker) Stop() {
	close(w.quit)
}

//调度中心
type Dispatcher struct {
	// on job exit
	OnJobExit chan Job
	// 空闲的worker
	FreeWorkers chan Worker
	// 所有worker实例
	Workers []Worker
	// 等待执行的job
	PendingJobs chan Job
}

//创建调度中心
func NewDispatcher(maxWorkers int, maxPendingJobs int, onJobExit chan Job) *Dispatcher {
	if onJobExit == nil {
		onJobExit = make(chan Job, maxWorkers)
		go func() {
			for {
				select {
				case <-onJobExit:
				}
			}
		}()
	}

	return &Dispatcher{
		FreeWorkers: make(chan Worker, maxWorkers),
		PendingJobs: make(chan Job, maxPendingJobs),
		OnJobExit:   onJobExit,
	}
}

//工作者池的初始化
func (d *Dispatcher) Run() {
	for i := 1; i < cap(d.FreeWorkers)+1; i++ {
		worker := NewWorker(d.FreeWorkers, i, d.OnJobExit)
		worker.Start()

		d.FreeWorkers <- worker
		d.Workers = append(d.Workers, worker)
	}
	go d.dispatch()
}

//调度
func (d *Dispatcher) dispatch() {
	for {
		worker := <-d.FreeWorkers
		// 得到 pending 中的job 和 空闲的worker， 并通知worker开始执行
		job := <-d.PendingJobs

		worker.jobChannel <- job
	}
}

func (d *Dispatcher) TryEnqueue(job Job) bool {
	select {
	case d.PendingJobs <- job:
		return true
	default:
		return false
	}
}
