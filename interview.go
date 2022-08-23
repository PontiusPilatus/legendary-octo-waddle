package main

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"sync"
	"time"
)

func main() {
	program()
	fmt.Println("Done.")
}
func program() {
	rand.Seed(time.Now().UnixNano())
	svc := NewService(5, 1*time.Second, 10*time.Second)

	t := time.NewTicker(45 * time.Second)
	id := 0
	for {
		select {
		case <-t.C:
			fmt.Println("Shut down")
			svc.Shutdown()
			return
		default:
			n := rand.Intn(2)
			time.Sleep(time.Duration(n) * time.Second)
			id++
			if err := svc.Add(Task{id: id}); err != nil {
				fmt.Println(err.Error())
			}
		}
	}
}

type Task struct {
	id int
}

type Service struct {
	sd   bool
	m    *sync.RWMutex
	cm   *sync.RWMutex
	wg   *sync.WaitGroup
	to   time.Duration
	ttl  time.Duration
	q    chan Task
	done map[int]time.Time
}

func NewService(wc int, to time.Duration, ttl time.Duration) *Service {
	s := &Service{
		q:    make(chan Task, wc),
		to:   to,
		m:    &sync.RWMutex{},
		cm:   &sync.RWMutex{},
		wg:   &sync.WaitGroup{},
		ttl:  ttl,
		done: make(map[int]time.Time),
	}

	for w := 0; w < wc; w++ {
		go s.worker()
	}

	go s.visualize()
	go s.cleaner()

	return s
}

func (s *Service) Add(t Task) error {
	if !s.process() {
		return nil
	}

	s.cm.RLock()
	last, ok := s.done[t.id]
	s.cm.RUnlock()
	if ok && time.Now().UTC().Add(-s.ttl).Before(last) {
		return errors.New("task already done")
	}

	tick := time.NewTicker(s.to)

	for {
		select {
		case <-tick.C:
			return errors.New("failed to add task: timeout reached")
		case s.q <- t:
			return nil
		}
	}
}

func (s *Service) Shutdown() {
	s.m.Lock()
	s.sd = true
	s.m.Unlock()

	s.wg.Wait()
	close(s.q)
	s.print()
	return
}

func (s *Service) worker() {
	fmt.Println("! worker is running...")
	for t := range s.q {
		s.wg.Add(1)

		s.cm.Lock()
		s.done[t.id] = time.Now().UTC()
		s.cm.Unlock()

		t.process()

		s.wg.Done()
	}
}

func (s *Service) cleaner() {
	for _ = range time.Tick(s.ttl) {
		if s.process() {
			return
		}

		s.cm.Lock()

		for k, v := range s.done {
			if !time.Now().UTC().Add(-s.ttl).After(v) {
				continue
			}

			delete(s.done, k)
		}

		s.cm.Unlock()
	}
}

func (t *Task) process() {
	time.Sleep(5 * time.Second)
}

func (s *Service) process() bool {
	s.m.RLock()
	defer s.m.RUnlock()
	return !s.sd
}

func (s *Service) visualize() {
	for {

		time.Sleep(1 * time.Second)
		s.print()
	}
}

func (s *Service) print() {
	clean()

	l := len(s.q)
	tmp := "[ %s]\n"
	q := ""
	for i := 0; i < cap(s.q); i++ {
		if i < l {
			q += "* "
			continue
		}
		q += "  "
	}
	fmt.Printf(tmp, q)
}

func clean() {
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	cmd.Run()
}
