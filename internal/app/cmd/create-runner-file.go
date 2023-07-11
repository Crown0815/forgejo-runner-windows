// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"

	pingv1 "code.gitea.io/actions-proto-go/ping/v1"
	"github.com/bufbuild/connect-go"
	gouuid "github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"gitea.com/gitea/act_runner/internal/app/run"
	"gitea.com/gitea/act_runner/internal/pkg/client"
	"gitea.com/gitea/act_runner/internal/pkg/config"
	"gitea.com/gitea/act_runner/internal/pkg/ver"
)

type createRunnerFileArgs struct {
	Connect      bool
	InstanceAddr string
	Secret       string
	Name         string
}

func createRunnerFileCmd(ctx context.Context, configFile *string) *cobra.Command {
	var argsVar createRunnerFileArgs
	cmd := &cobra.Command{
		Use:   "create-runner-file",
		Short: "Create a runner file using a shared secret used to pre-register the runner on the Forgejo instance",
		Args:  cobra.MaximumNArgs(0),
		RunE:  runCreateRunnerFile(ctx, &argsVar, configFile),
	}
	cmd.Flags().BoolVar(&argsVar.Connect, "connect", false, "tries to connect to the instance using the secret (Forgejo v1.21 instance or greater)")
	cmd.Flags().StringVar(&argsVar.InstanceAddr, "instance", "", "Forgejo instance address")
	cmd.MarkFlagRequired("instance")
	cmd.Flags().StringVar(&argsVar.Secret, "secret", "", "secret shared with the Frogejo instance via forgejo-cli actions register")
	cmd.MarkFlagRequired("secret")
	cmd.Flags().StringVar(&argsVar.Name, "name", "", "Runner name")

	return cmd
}

// must be exactly the same as fogejo/models/actions/forgejo.go
func uuidFromSecret(secret string) (string, error) {
	uuid, err := gouuid.FromBytes([]byte(secret[:16]))
	if err != nil {
		return "", fmt.Errorf("gouuid.FromBytes %v", err)
	}
	return uuid.String(), nil
}

// should be exactly the same as forgejo/cmd/forgejo/actions.go
func validateSecret(secret string) error {
	secretLen := len(secret)
	if secretLen != 40 {
		return fmt.Errorf("the secret must be exactly 40 characters long, not %d", secretLen)
	}
	if _, err := hex.DecodeString(secret); err != nil {
		return fmt.Errorf("the secret must be an hexadecimal string: %w", err)
	}
	return nil
}

func ping(cfg *config.Config, reg *config.Registration) error {
	// initial http client
	cli := client.New(
		reg.Address,
		cfg.Runner.Insecure,
		"",
		"",
		ver.Version(),
	)

	_, err := cli.Ping(context.Background(), connect.NewRequest(&pingv1.PingRequest{
		Data: reg.UUID,
	}))
	if err != nil {
		return fmt.Errorf("ping %s failed %w", reg.Address, err)
	}
	return nil
}

func runCreateRunnerFile(ctx context.Context, args *createRunnerFileArgs, configFile *string) func(cmd *cobra.Command, args []string) error {
	return func(*cobra.Command, []string) error {
		log.SetLevel(log.DebugLevel)
		log.Info("Creating runner file")

		//
		// Prepare the registration data
		//
		cfg, err := config.LoadDefault(*configFile)
		if err != nil {
			return fmt.Errorf("invalid configuration: %w", err)
		}

		if err := validateSecret(args.Secret); err != nil {
			return err
		}

		uuid, err := uuidFromSecret(args.Secret)
		if err != nil {
			return err
		}

		name := args.Name
		if name == "" {
			name, _ = os.Hostname()
			log.Infof("Runner name is empty, use hostname '%s'.", name)
		}

		reg := &config.Registration{
			Name:    name,
			UUID:    uuid,
			Token:   args.Secret,
			Address: args.InstanceAddr,
		}

		//
		// Verify the Forgejo instance is reachable
		//
		if err := ping(cfg, reg); err != nil {
			return err
		}

		//
		// Save the registration file
		//
		if err := config.SaveRegistration(cfg.Runner.File, reg); err != nil {
			return fmt.Errorf("failed to save runner config to %s: %w", cfg.Runner.File, err)
		}

		//
		// Verify the secret works
		//
		if args.Connect {
			cli := client.New(
				reg.Address,
				cfg.Runner.Insecure,
				reg.UUID,
				reg.Token,
				ver.Version(),
			)

			runner := run.NewRunner(cfg, reg, cli)
			resp, err := runner.Declare(ctx, cfg.Runner.Labels)

			if err != nil && connect.CodeOf(err) == connect.CodeUnimplemented {
				log.Warn("Cannot verify the connection because the Forgejo instance is lower than v1.21")
			} else if err != nil {
				log.WithError(err).Error("fail to invoke Declare")
				return err
			} else {
				log.Infof("connection successful: %s, with version: %s, with labels: %v",
					resp.Msg.Runner.Name, resp.Msg.Runner.Version, resp.Msg.Runner.Labels)
			}
		}
		return nil
	}
}
