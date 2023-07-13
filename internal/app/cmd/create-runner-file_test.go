// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"context"
	"os"
	"testing"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"gitea.com/gitea/act_runner/internal/pkg/client"
	"gitea.com/gitea/act_runner/internal/pkg/config"
	"gitea.com/gitea/act_runner/internal/pkg/ver"
	"github.com/bufbuild/connect-go"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func executeCommand(ctx context.Context, cmd *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(ctx)

	return buf.String(), err
}

func Test_createRunnerFileCmd(t *testing.T) {
	configFile := "config.yml"
	ctx := context.Background()
	cmd := createRunnerFileCmd(ctx, &configFile)
	output, err := executeCommand(ctx, cmd)
	assert.ErrorContains(t, err, `required flag(s) "instance", "secret" not set`)
	assert.Contains(t, output, "Usage:")
}

func Test_validateSecret(t *testing.T) {
	assert.ErrorContains(t, validateSecret("abc"), "exactly 40 characters")
	assert.ErrorContains(t, validateSecret("ZAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"), "must be an hexadecimal")
}

func Test_uuidFromSecret(t *testing.T) {
	uuid, err := uuidFromSecret("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	assert.NoError(t, err)
	assert.EqualValues(t, uuid, "41414141-4141-4141-4141-414141414141")
}

func Test_ping(t *testing.T) {
	cfg := &config.Config{}
	address := os.Getenv("FORGEJO_URL")
	if address == "" {
		address = "https://code.forgejo.org"
	}
	reg := &config.Registration{
		Address: address,
		UUID:    "create-runner-file_test.go",
	}
	assert.NoError(t, ping(cfg, reg))
}

func Test_runCreateRunnerFile(t *testing.T) {
	//
	// Set the .runner file to be in a temporary directory
	//
	dir := t.TempDir()
	configFile := dir + "/config.yml"
	runnerFile := dir + "/.runner"
	cfg, err := config.LoadDefault("")
	cfg.Runner.File = runnerFile
	yamlData, err := yaml.Marshal(cfg)
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(configFile, yamlData, 0o666))

	instance, has := os.LookupEnv("FORGEJO_URL")
	if !has {
		instance = "https://code.forgejo.org"
	}
	secret, has := os.LookupEnv("FORGEJO_RUNNER_SECRET")
	assert.True(t, has)
	name := "testrunner"

	//
	// Run create-runner-file
	//
	ctx := context.Background()
	cmd := createRunnerFileCmd(ctx, &configFile)
	output, err := executeCommand(ctx, cmd, "--connect", "--secret", secret, "--instance", instance, "--name", name)
	assert.NoError(t, err)
	assert.EqualValues(t, "", output)

	//
	// Read back the runner file and verify its content
	//
	reg, err := config.LoadRegistration(runnerFile)
	assert.NoError(t, err)
	assert.EqualValues(t, secret, reg.Token)
	assert.EqualValues(t, instance, reg.Address)

	//
	// Verify that fetching a task successfully returns there is
	// no task for this runner
	//
	cli := client.New(
		reg.Address,
		cfg.Runner.Insecure,
		reg.UUID,
		reg.Token,
		ver.Version(),
	)
	resp, err := cli.FetchTask(ctx, connect.NewRequest(&runnerv1.FetchTaskRequest{}))
	assert.NoError(t, err)
	assert.Nil(t, resp.Msg.Task)
}
