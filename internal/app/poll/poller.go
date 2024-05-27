// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package poll

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/bufbuild/connect-go"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"gitea.com/gitea/act_runner/internal/app/run"
	"gitea.com/gitea/act_runner/internal/pkg/client"
	"gitea.com/gitea/act_runner/internal/pkg/config"
)

type Poller struct {
	client       client.Client
	runner       *run.Runner
	cfg          *config.Config
	tasksVersion atomic.Int64 // tasksVersion used to store the version of the last task fetched from the Gitea.

	pollingCtx      context.Context
	shutdownPolling context.CancelFunc

	jobsCtx      context.Context
	shutdownJobs context.CancelFunc

	done chan struct{}
}

func New(cfg *config.Config, client client.Client, runner *run.Runner) *Poller {
	pollingCtx, shutdownPolling := context.WithCancel(context.Background())

	jobsCtx, shutdownJobs := context.WithCancel(context.Background())

	done := make(chan struct{})

	return &Poller{
		client: client,
		runner: runner,
		cfg:    cfg,

		pollingCtx:      pollingCtx,
		shutdownPolling: shutdownPolling,

		jobsCtx:      jobsCtx,
		shutdownJobs: shutdownJobs,

		done: done,
	}
}

func (p *Poller) Poll() {
	limiter := rate.NewLimiter(rate.Every(p.cfg.Runner.FetchInterval), 1)
	wg := &sync.WaitGroup{}
	for i := 0; i < p.cfg.Runner.Capacity; i++ {
		wg.Add(1)
		go p.poll(wg, limiter)
	}
	wg.Wait()

	// signal that we shutdown
	close(p.done)
}

func (p *Poller) Shutdown(ctx context.Context) error {
	p.shutdownPolling()

	select {
	// graceful shutdown completed succesfully
	case <-p.done:
		return nil

	// our timeout for shutting down ran out
	case <-ctx.Done():
		// when both the timeout fires and the graceful shutdown
		// completed succsfully, this branch of the select may
		// fire. Do a non-blocking check here against the graceful
		// shutdown status to avoid sending an error if we don't need to.
		_, ok := <-p.done
		if !ok {
			return nil
		}

		// force a shutdown of all running jobs
		p.shutdownJobs()

		// wait for running jobs to report their status to Gitea
		_, _ = <-p.done

		return ctx.Err()
	}
}

func (p *Poller) poll(wg *sync.WaitGroup, limiter *rate.Limiter) {
	defer wg.Done()
	for {
		if err := limiter.Wait(p.pollingCtx); err != nil {
			if p.pollingCtx.Err() != nil {
				log.WithError(err).Debug("limiter wait failed")
			}
			return
		}
		task, ok := p.fetchTask(p.pollingCtx)
		if !ok {
			continue
		}

		p.runTaskWithRecover(p.jobsCtx, task)
	}
}

func (p *Poller) runTaskWithRecover(ctx context.Context, task *runnerv1.Task) {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic: %v", r)
			log.WithError(err).Error("panic in runTaskWithRecover")
		}
	}()

	if err := p.runner.Run(ctx, task); err != nil {
		log.WithError(err).Error("failed to run task")
	}
}

func (p *Poller) fetchTask(ctx context.Context) (*runnerv1.Task, bool) {
	reqCtx, cancel := context.WithTimeout(ctx, p.cfg.Runner.FetchTimeout)
	defer cancel()

	// Load the version value that was in the cache when the request was sent.
	v := p.tasksVersion.Load()
	resp, err := p.client.FetchTask(reqCtx, connect.NewRequest(&runnerv1.FetchTaskRequest{
		TasksVersion: v,
	}))
	if errors.Is(err, context.DeadlineExceeded) {
		err = nil
	}
	if err != nil {
		log.WithError(err).Error("failed to fetch task")
		return nil, false
	}

	if resp == nil || resp.Msg == nil {
		return nil, false
	}

	if resp.Msg.TasksVersion > v {
		p.tasksVersion.CompareAndSwap(v, resp.Msg.TasksVersion)
	}

	if resp.Msg.Task == nil {
		return nil, false
	}

	// got a task, set `tasksVersion` to zero to focre query db in next request.
	p.tasksVersion.CompareAndSwap(resp.Msg.TasksVersion, 0)

	return resp.Msg.Task, true
}
