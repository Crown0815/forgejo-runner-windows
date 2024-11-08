// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package run

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"connectrpc.com/connect"
	"github.com/docker/docker/api/types/container"
	"github.com/nektos/act/pkg/artifactcache"
	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	"github.com/nektos/act/pkg/runner"
	log "github.com/sirupsen/logrus"

	"gitea.com/gitea/act_runner/internal/pkg/client"
	"gitea.com/gitea/act_runner/internal/pkg/config"
	"gitea.com/gitea/act_runner/internal/pkg/labels"
	"gitea.com/gitea/act_runner/internal/pkg/report"
	"gitea.com/gitea/act_runner/internal/pkg/ver"
)

// Runner runs the pipeline.
type Runner struct {
	name string

	cfg *config.Config

	client client.Client
	labels labels.Labels
	envs   map[string]string

	runningTasks sync.Map
}

type RunnerInterface interface {
	Run(ctx context.Context, task *runnerv1.Task) error
}

func NewRunner(cfg *config.Config, reg *config.Registration, cli client.Client) *Runner {
	ls := labels.Labels{}
	for _, v := range reg.Labels {
		if l, err := labels.Parse(v); err == nil {
			ls = append(ls, l)
		}
	}

	if cfg.Runner.Envs == nil {
		cfg.Runner.Envs = make(map[string]string, 10)
	}

	cfg.Runner.Envs["GITHUB_SERVER_URL"] = reg.Address

	envs := make(map[string]string, len(cfg.Runner.Envs))
	for k, v := range cfg.Runner.Envs {
		envs[k] = v
	}
	if cfg.Cache.Enabled == nil || *cfg.Cache.Enabled {
		if cfg.Cache.ExternalServer != "" {
			envs["ACTIONS_CACHE_URL"] = cfg.Cache.ExternalServer
		} else {
			cacheHandler, err := artifactcache.StartHandler(
				cfg.Cache.Dir,
				cfg.Cache.Host,
				cfg.Cache.Port,
				log.StandardLogger().WithField("module", "cache_request"),
			)
			if err != nil {
				log.Errorf("cannot init cache server, it will be disabled: %v", err)
				// go on
			} else {
				envs["ACTIONS_CACHE_URL"] = cacheHandler.ExternalURL() + "/"
			}
		}
	}

	// set artifact gitea api
	artifactGiteaAPI := strings.TrimSuffix(cli.Address(), "/") + "/api/actions_pipeline/"
	envs["ACTIONS_RUNTIME_URL"] = artifactGiteaAPI
	envs["ACTIONS_RESULTS_URL"] = strings.TrimSuffix(cli.Address(), "/")

	// Set specific environments to distinguish between Gitea and GitHub
	envs["GITEA_ACTIONS"] = "true"
	envs["GITEA_ACTIONS_RUNNER_VERSION"] = ver.Version()

	return &Runner{
		name:   reg.Name,
		cfg:    cfg,
		client: cli,
		labels: ls,
		envs:   envs,
	}
}

func (r *Runner) Run(ctx context.Context, task *runnerv1.Task) error {
	if _, ok := r.runningTasks.Load(task.Id); ok {
		return fmt.Errorf("task %d is already running", task.Id)
	}
	r.runningTasks.Store(task.Id, struct{}{})
	defer r.runningTasks.Delete(task.Id)

	ctx, cancel := context.WithTimeout(ctx, r.cfg.Runner.Timeout)
	defer cancel()
	reporter := report.NewReporter(ctx, cancel, r.client, task, r.cfg.Runner.ReportInterval)
	var runErr error
	defer func() {
		lastWords := ""
		if runErr != nil {
			lastWords = runErr.Error()
		}
		_ = reporter.Close(lastWords)
	}()
	reporter.RunDaemon()
	runErr = r.run(ctx, task, reporter)

	return nil
}

func (r *Runner) run(ctx context.Context, task *runnerv1.Task, reporter *report.Reporter) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	reporter.Logf("%s(version:%s) received task %v of job %v, be triggered by event: %s", r.name, ver.Version(), task.Id, task.Context.Fields["job"].GetStringValue(), task.Context.Fields["event_name"].GetStringValue())

	workflow, jobID, err := generateWorkflow(task)
	if err != nil {
		return err
	}

	plan, err := model.CombineWorkflowPlanner(workflow).PlanJob(jobID)
	if err != nil {
		return err
	}
	job := workflow.GetJob(jobID)
	reporter.ResetSteps(len(job.Steps))

	taskContext := task.Context.Fields

	log.Infof("task %v repo is %v %v %v", task.Id, taskContext["repository"].GetStringValue(),
		taskContext["gitea_default_actions_url"].GetStringValue(),
		r.client.Address())

	preset := &model.GithubContext{
		Event:           taskContext["event"].GetStructValue().AsMap(),
		RunID:           taskContext["run_id"].GetStringValue(),
		RunNumber:       taskContext["run_number"].GetStringValue(),
		Actor:           taskContext["actor"].GetStringValue(),
		Repository:      taskContext["repository"].GetStringValue(),
		EventName:       taskContext["event_name"].GetStringValue(),
		Sha:             taskContext["sha"].GetStringValue(),
		Ref:             taskContext["ref"].GetStringValue(),
		RefName:         taskContext["ref_name"].GetStringValue(),
		RefType:         taskContext["ref_type"].GetStringValue(),
		HeadRef:         taskContext["head_ref"].GetStringValue(),
		BaseRef:         taskContext["base_ref"].GetStringValue(),
		Token:           taskContext["token"].GetStringValue(),
		RepositoryOwner: taskContext["repository_owner"].GetStringValue(),
		RetentionDays:   taskContext["retention_days"].GetStringValue(),
	}
	if t := task.Secrets["GITEA_TOKEN"]; t != "" {
		preset.Token = t
	} else if t := task.Secrets["GITHUB_TOKEN"]; t != "" {
		preset.Token = t
	}

	giteaRuntimeToken := taskContext["gitea_runtime_token"].GetStringValue()
	if giteaRuntimeToken == "" {
		// use task token to action api token for previous Gitea Server Versions
		giteaRuntimeToken = preset.Token
	}
	r.envs["ACTIONS_RUNTIME_TOKEN"] = giteaRuntimeToken

	eventJSON, err := json.Marshal(preset.Event)
	if err != nil {
		return err
	}

	maxLifetime := 3 * time.Hour
	if deadline, ok := ctx.Deadline(); ok {
		maxLifetime = time.Until(deadline)
	}

	var inputs map[string]string
	if preset.EventName == "workflow_dispatch" {
		if inputsRaw, ok := preset.Event["inputs"]; ok {
			inputs, _ = inputsRaw.(map[string]string)
		}
	}

	runnerConfig := &runner.Config{
		// On Linux, Workdir will be like "/<parent_directory>/<owner>/<repo>"
		// On Windows, Workdir will be like "\<parent_directory>\<owner>\<repo>"
		Workdir:        filepath.FromSlash(filepath.Clean(fmt.Sprintf("/%s/%s", r.cfg.Container.WorkdirParent, preset.Repository))),
		BindWorkdir:    false,
		ActionCacheDir: filepath.FromSlash(r.cfg.Host.WorkdirParent),

		ReuseContainers:            false,
		ForcePull:                  r.cfg.Container.ForcePull,
		ForceRebuild:               false,
		LogOutput:                  true,
		JSONLogger:                 false,
		Env:                        r.envs,
		Secrets:                    task.Secrets,
		GitHubInstance:             strings.TrimSuffix(r.client.Address(), "/"),
		AutoRemove:                 true,
		NoSkipCheckout:             true,
		PresetGitHubContext:        preset,
		EventJSON:                  string(eventJSON),
		ContainerNamePrefix:        fmt.Sprintf("GITEA-ACTIONS-TASK-%d", task.Id),
		ContainerMaxLifetime:       maxLifetime,
		ContainerNetworkMode:       container.NetworkMode(r.cfg.Container.Network),
		ContainerNetworkEnableIPv6: r.cfg.Container.EnableIPv6,
		ContainerOptions:           r.cfg.Container.Options,
		ContainerDaemonSocket:      r.cfg.Container.DockerHost,
		Privileged:                 r.cfg.Container.Privileged,
		DefaultActionInstance:      taskContext["gitea_default_actions_url"].GetStringValue(),
		PlatformPicker:             r.labels.PickPlatform,
		Vars:                       task.Vars,
		ValidVolumes:               r.cfg.Container.ValidVolumes,
		InsecureSkipTLS:            r.cfg.Runner.Insecure,
		Inputs:                     inputs,
	}

	if r.cfg.Log.JobLevel != "" {
		level, err := log.ParseLevel(r.cfg.Log.JobLevel)
		if err != nil {
			return err
		}

		runnerConfig.JobLoggerLevel = &level
	}

	rr, err := runner.New(runnerConfig)
	if err != nil {
		return err
	}
	executor := rr.NewPlanExecutor(plan)

	reporter.Logf("workflow prepared")

	// add logger recorders
	ctx = common.WithLoggerHook(ctx, reporter)

	if !log.IsLevelEnabled(log.DebugLevel) {
		ctx = runner.WithJobLoggerFactory(ctx, NullLogger{})
	}

	execErr := executor(ctx)
	reporter.SetOutputs(job.Outputs)
	return execErr
}

func (r *Runner) Declare(ctx context.Context, labels []string) (*connect.Response[runnerv1.DeclareResponse], error) {
	return r.client.Declare(ctx, connect.NewRequest(&runnerv1.DeclareRequest{
		Version: ver.Version(),
		Labels:  labels,
	}))
}

func (r *Runner) Update(ctx context.Context, labels labels.Labels) {
	r.labels = labels
}
