package tinybiome

import (
	"fmt"
	"log"
	"runtime/debug"
	"sync"
	"time"
)

type RetryGroup struct {
	errs chan error
	wg   sync.WaitGroup
}

func NewRetryGroup() *RetryGroup {
	return &RetryGroup{errs: make(chan error, 2)}
}

func (r *RetryGroup) Wait() error {
	go func() {
		r.wg.Wait()
		close(r.errs)
	}()

	if err, ok := <-r.errs; ok {
		return err
	}
	return nil
}

func (r *RetryGroup) Add(name string, failer func() error) {
	r.wg.Add(1)
	go func() {
		if err := r.Retry(name, failer); err != nil {
			r.errs <- err
		}
		r.wg.Done()
	}()
}

func (r *RetryGroup) Retry(name string, failer func() error) error {
	attempt := 0
	lastFail := time.Now()
	for {
		log.Printf("service(%s) attempt %d", name, attempt)
		err := func() (err error) {
			defer func() {
				rec := recover()
				if rec != nil {
					err = fmt.Errorf("panic:%#v", rec)
					log.Println(string(debug.Stack()))
				}
			}()
			err = failer()
			return
		}()

		log.Printf("service(%s) error: %s", name, err)

		attempt += 1
		if time.Since(lastFail) > time.Duration(attempt)*time.Second {
			attempt = 0
		}
		lastFail = time.Now()

		if attempt > 5 {
			return err
		}
	}
}
