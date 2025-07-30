package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAdditionalFunctions(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
on: push
jobs:
  job-with-condition:
    runs-on: self-hosted
    if: success() || success('joba', 'jobb') || failure() || failure('joba', 'jobb') || always() || cancelled()
    steps:
    - run: exit 0
`), &node)
	if !assert.NoError(t, err) {
		return
	}
	err = (&Node{
		Definition: "workflow-root-strict",
		Schema:     GetWorkflowSchema(),
	}).UnmarshalYAML(&node)
	assert.NoError(t, err)
}

func TestAdditionalFunctionsFailure(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
on: push
jobs:
  job-with-condition:
    runs-on: self-hosted
    if: success() || success('joba', 'jobb') || failure() || failure('joba', 'jobb') || always('error')
    steps:
    - run: exit 0
`), &node)
	if !assert.NoError(t, err) {
		return
	}
	err = (&Node{
		Definition: "workflow-root-strict",
		Schema:     GetWorkflowSchema(),
	}).UnmarshalYAML(&node)
	assert.Error(t, err)
}

func TestAdditionalFunctionsSteps(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
on: push
jobs:
  job-with-condition:
    runs-on: self-hosted
    steps:
    - run: exit 0
      if: success() || failure() || always()
    - uses: https://${{ secrets.PAT }}@example.com/action/here@v1
`), &node)
	if !assert.NoError(t, err) {
		return
	}
	err = (&Node{
		Definition: "workflow-root-strict",
		Schema:     GetWorkflowSchema(),
	}).UnmarshalYAML(&node)
	assert.NoError(t, err)
}

func TestAdditionalFunctionsStepsExprSyntax(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
on: push
jobs:
  job-with-condition:
    runs-on: self-hosted
    steps:
    - run: exit 0
      if: ${{ success() || failure() || always() }}
`), &node)
	if !assert.NoError(t, err) {
		return
	}
	err = (&Node{
		Definition: "workflow-root-strict",
		Schema:     GetWorkflowSchema(),
	}).UnmarshalYAML(&node)
	assert.NoError(t, err)
}

func TestWorkflowCallRunsOn(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
name: Build Silo Frontend DEV
on:
  push:
    branches:
      - dev
      - dev-*
jobs:
  build_frontend_dev:
    name: Build Silo Frontend DEV
    runs-on: ubuntu-latest
    container:
      image: code.forgejo.org/oci/node:22-bookworm
    uses: ./.forgejo/workflows/${{ vars.PATHNAME }}
    with:
      STAGE: dev
    secrets:
      PACKAGE_WRITER_TOKEN: ${{ secrets.PACKAGE_WRITER_TOKEN }}
`), &node)
	require.NoError(t, err)
	n := &Node{
		Definition: "workflow-root",
		Schema:     GetWorkflowSchema(),
	}
	require.NoError(t, n.UnmarshalYAML(&node))
}

func TestActionSchema(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
name: 'action name'
author: 'action authors'
description: |
  action description
inputs:
  url:
    description: 'url description'
    default: '${{ env.GITHUB_SERVER_URL }}'
  repo:
    description: 'repo description'
    default: '${{ github.repository }}'
runs:
  using: "composite"
  steps:
    - run: |
        echo "${{ github.action_path }}"
      env:
          MYVAR: ${{ vars.VARIABLE }}
`), &node)
	if !assert.NoError(t, err) {
		return
	}
	err = (&Node{
		Definition: "action-root",
		Schema:     GetActionSchema(),
	}).UnmarshalYAML(&node)
	assert.NoError(t, err)
}
