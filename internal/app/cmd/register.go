// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	goruntime "runtime"
	"strings"
	"time"

	pingv1 "code.gitea.io/actions-proto-go/ping/v1"
	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/bufbuild/connect-go"
	"github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"gitea.com/gitea/act_runner/internal/pkg/client"
	"gitea.com/gitea/act_runner/internal/pkg/config"
	"gitea.com/gitea/act_runner/internal/pkg/labels"
	"gitea.com/gitea/act_runner/internal/pkg/ver"
)

// runRegister registers a runner to the server
func runRegister(ctx context.Context, regArgs *registerArgs, configFile *string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		log.SetReportCaller(false)
		isTerm := isatty.IsTerminal(os.Stdout.Fd())
		log.SetFormatter(&log.TextFormatter{
			DisableColors:    !isTerm,
			DisableTimestamp: true,
		})
		log.SetLevel(log.DebugLevel)

		log.Infof("Registering runner, arch=%s, os=%s, version=%s.",
			goruntime.GOARCH, goruntime.GOOS, ver.Version())

		// runner always needs root permission
		if os.Getuid() != 0 {
			// TODO: use a better way to check root permission
			log.Warnf("Runner in user-mode.")
		}

		if regArgs.NoInteractive {
			if err := registerNoInteractive(*configFile, regArgs); err != nil {
				return err
			}
		} else {
			go func() {
				if err := registerInteractive(*configFile); err != nil {
					log.Fatal(err)
					return
				}
				os.Exit(0)
			}()

			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			<-c
		}

		return nil
	}
}

// registerArgs represents the arguments for register command
type registerArgs struct {
	NoInteractive bool
	InstanceAddr  string
	Token         string
	RunnerName    string
	Labels        string
}

type registerStage int8

const (
	StageUnknown              registerStage = -1
	StageOverwriteLocalConfig registerStage = iota + 1
	StageInputInstance
	StageInputToken
	StageInputRunnerName
	StageInputLabels
	StageWaitingForRegistration
	StageExit
)

var defaultLabels = []string{
	"ubuntu-latest:docker://node:16-bullseye",
	"ubuntu-22.04:docker://node:16-bullseye", // There's no node:16-bookworm yet
	"ubuntu-20.04:docker://node:16-bullseye",
	"ubuntu-18.04:docker://node:16-buster",
}

type registerInputs struct {
	InstanceAddr string
	Token        string
	RunnerName   string
	Labels       []string
}

func (r *registerInputs) validate() error {
	if r.InstanceAddr == "" {
		return fmt.Errorf("instance address is empty")
	}
	if r.Token == "" {
		return fmt.Errorf("token is empty")
	}
	if len(r.Labels) > 0 {
		return validateLabels(r.Labels)
	}
	return nil
}

func validateLabels(ls []string) error {
	for _, label := range ls {
		if _, err := labels.Parse(label); err != nil {
			return err
		}
	}
	return nil
}

func (r *registerInputs) assignToNext(stage registerStage, value string, cfg *config.Config) registerStage {
	// must set instance address and token.
	// if empty, keep current stage.
	if stage == StageInputInstance || stage == StageInputToken {
		if value == "" {
			return stage
		}
	}

	// set hostname for runner name if empty
	if stage == StageInputRunnerName && value == "" {
		value, _ = os.Hostname()
	}

	switch stage {
	case StageOverwriteLocalConfig:
		if value == "Y" || value == "y" {
			return StageInputInstance
		}
		return StageExit
	case StageInputInstance:
		r.InstanceAddr = value
		return StageInputToken
	case StageInputToken:
		r.Token = value
		return StageInputRunnerName
	case StageInputRunnerName:
		r.RunnerName = value
		// if there are some labels configured in config file, skip input labels stage
		if len(cfg.Runner.Labels) > 0 {
			ls := make([]string, 0, len(cfg.Runner.Labels))
			for _, l := range cfg.Runner.Labels {
				_, err := labels.Parse(l)
				if err != nil {
					log.WithError(err).Warnf("ignored invalid label %q", l)
					continue
				}
				ls = append(ls, l)
			}
			if len(ls) == 0 {
				log.Warn("no valid labels configured in config file, runner may not be able to pick up jobs")
			}
			r.Labels = ls
			return StageWaitingForRegistration
		}
		return StageInputLabels
	case StageInputLabels:
		r.Labels = defaultLabels
		if value != "" {
			r.Labels = strings.Split(value, ",")
		}

		if validateLabels(r.Labels) != nil {
			log.Infoln("Invalid labels, please input again, leave blank to use the default labels (for example, ubuntu-20.04:docker://node:16-bullseye,ubuntu-18.04:docker://node:16-buster,linux_arm:host)")
			return StageInputLabels
		}
		return StageWaitingForRegistration
	}
	return StageUnknown
}

func registerInteractive(configFile string) error {
	var (
		reader = bufio.NewReader(os.Stdin)
		stage  = StageInputInstance
		inputs = new(registerInputs)
	)

	cfg, err := config.LoadDefault(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}
	if f, err := os.Stat(cfg.Runner.File); err == nil && !f.IsDir() {
		stage = StageOverwriteLocalConfig
	}

	for {
		printStageHelp(stage)

		cmdString, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		stage = inputs.assignToNext(stage, strings.TrimSpace(cmdString), cfg)

		if stage == StageWaitingForRegistration {
			log.Infof("Registering runner, name=%s, instance=%s, labels=%v.", inputs.RunnerName, inputs.InstanceAddr, inputs.Labels)
			if err := doRegister(cfg, inputs); err != nil {
				return fmt.Errorf("Failed to register runner: %w", err)
			} else {
				log.Infof("Runner registered successfully.")
			}
			return nil
		}

		if stage == StageExit {
			return nil
		}

		if stage <= StageUnknown {
			log.Errorf("Invalid input, please re-run act command.")
			return nil
		}
	}
}

func printStageHelp(stage registerStage) {
	switch stage {
	case StageOverwriteLocalConfig:
		log.Infoln("Runner is already registered, overwrite local config? [y/N]")
	case StageInputInstance:
		log.Infoln("Enter the Forgejo instance URL (for example, https://next.forgejo.org/):")
	case StageInputToken:
		log.Infoln("Enter the runner token:")
	case StageInputRunnerName:
		hostname, _ := os.Hostname()
		log.Infof("Enter the runner name (if set empty, use hostname: %s):\n", hostname)
	case StageInputLabels:
		log.Infoln("Enter the runner labels, leave blank to use the default labels (comma-separated, for example, ubuntu-20.04:docker://node:16-bullseye,ubuntu-18.04:docker://node:16-buster,linux_arm:host):")
	case StageWaitingForRegistration:
		log.Infoln("Waiting for registration...")
	}
}

func registerNoInteractive(configFile string, regArgs *registerArgs) error {
	cfg, err := config.LoadDefault(configFile)
	if err != nil {
		return err
	}
	inputs := &registerInputs{
		InstanceAddr: regArgs.InstanceAddr,
		Token:        regArgs.Token,
		RunnerName:   regArgs.RunnerName,
		Labels:       defaultLabels,
	}
	regArgs.Labels = strings.TrimSpace(regArgs.Labels)
	// command line flag.
	if regArgs.Labels != "" {
		inputs.Labels = strings.Split(regArgs.Labels, ",")
	}
	// specify labels in config file.
	if len(cfg.Runner.Labels) > 0 {
		if regArgs.Labels != "" {
			log.Warn("Labels from command will be ignored, use labels defined in config file.")
		}
		inputs.Labels = cfg.Runner.Labels
	}

	if inputs.RunnerName == "" {
		inputs.RunnerName, _ = os.Hostname()
		log.Infof("Runner name is empty, use hostname '%s'.", inputs.RunnerName)
	}
	if err := inputs.validate(); err != nil {
		log.WithError(err).Errorf("Invalid input, please re-run act command.")
		return nil
	}
	if err := doRegister(cfg, inputs); err != nil {
		return fmt.Errorf("Failed to register runner: %w", err)
	}
	log.Infof("Runner registered successfully.")
	return nil
}

func doRegister(cfg *config.Config, inputs *registerInputs) error {
	ctx := context.Background()

	// initial http client
	cli := client.New(
		inputs.InstanceAddr,
		cfg.Runner.Insecure,
		"",
		"",
		ver.Version(),
	)

	for {
		_, err := cli.Ping(ctx, connect.NewRequest(&pingv1.PingRequest{
			Data: inputs.RunnerName,
		}))
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		if ctx.Err() != nil {
			break
		}
		if err != nil {
			log.WithError(err).
				Errorln("Cannot ping the Forgejo instance server")
			// TODO: if ping failed, retry or exit
			time.Sleep(time.Second)
		} else {
			log.Debugln("Successfully pinged the Forgejo instance server")
			break
		}
	}

	reg := &config.Registration{
		Name:    inputs.RunnerName,
		Token:   inputs.Token,
		Address: inputs.InstanceAddr,
		Labels:  inputs.Labels,
	}

	ls := make([]string, len(reg.Labels))
	for i, v := range reg.Labels {
		l, _ := labels.Parse(v)
		ls[i] = l.Name
	}
	// register new runner.
	resp, err := cli.Register(ctx, connect.NewRequest(&runnerv1.RegisterRequest{
		Name:        reg.Name,
		Token:       reg.Token,
		Version:     ver.Version(),
		AgentLabels: ls, // Could be removed after Gitea 1.20
		Labels:      ls,
	}))
	if err != nil {
		log.WithError(err).Error("poller: cannot register new runner")
		return err
	}

	reg.ID = resp.Msg.Runner.Id
	reg.UUID = resp.Msg.Runner.Uuid
	reg.Name = resp.Msg.Runner.Name
	reg.Token = resp.Msg.Runner.Token

	if err := config.SaveRegistration(cfg.Runner.File, reg); err != nil {
		return fmt.Errorf("failed to save runner config: %w", err)
	}
	return nil
}
