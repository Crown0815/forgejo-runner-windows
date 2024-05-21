# Forgejo Runner

**WARNING:** this is [alpha release quality](https://en.wikipedia.org/wiki/Software_release_life_cycle#Alpha) code and should not be considered secure enough to deploy in production.

A daemon that connects to a Forgejo instance and runs jobs for continous integration. The [installation and usage instructions](https://forgejo.org/docs/next/admin/actions/) are part of the Forgejo documentation.

# Hacking

The Forgejo runner depends on [a fork of ACT](https://code.forgejo.org/forgejo/act) and is a dependency of the [setup-forgejo action](https://code.forgejo.org/actions/setup-forgejo). See [the full dependency graph](https://code.forgeasdfjo.org/actions/cascading-pr/#forgejo-dependencies) for a global view.

## Local debug

The repositories are checked out in the same directory:

- **runner**: [Forgejo runner](https://code.forgejo.org/forgejo/runner)
- **act**: [ACT](https://code.forgejo.org/forgejo/act)
- **setup-forgejo**: [setup-forgejo](https://code.forgejo.org/actions/setup-forgejo)

### Install dependencies

The dependencies are installed manually or with:

```shell
setup-forgejo/forgejo-dependencies.sh
```

### Build the Forgejo runner with the local ACT

The Forgejo runner is rebuilt with the ACT directory by changing the `runner/go.mod` file to:

```
replace github.com/nektos/act => ../act
```

Running:

```
cd runner ; go mod tidy
```

Building:

```shell
cd runner ; rm -f forgejo-runner ; make forgejo-runner
```

### Launch Forgejo and the runner

A Forgejo instance is launched with:

```shell
cd setup-forgejo
./forgejo.sh setup
firefox $(cat forgejo-url)
```

The user is `root` with password `admin1234`. The runner is registered with:

```
cd setup-forgejo
docker exec --user 1000 forgejo forgejo actions generate-runner-token > forgejo-runner-token
../runner/forgejo-runner register --no-interactive --instance "$(cat forgejo-url)" --name runner --token $(cat forgejo-runner-token) --labels docker:docker://node:16-bullseye,self-hosted:host://-self-hosted,lxc:lxc://debian:bullseye
```

And launched with:

```shell
cd setup-forgejo ; ../runner/forgejo-runner --config runner-config.yml daemon
```

Note that the `runner-config.yml` is required in that particular case
to configure the network in `bridge` mode, otherwise the runner will
create a network that cannot reach the forgejo instance.

### Try a sample workflow

From the Forgejo web interface, create a repository and add the
following to `.forgejo/workflows/try.yaml`. It will launch the job and
the result can be observed from the `actions` tab.

```yaml
on: [push]
jobs:
  ls:
    runs-on: docker
    steps:
      - uses: actions/checkout@v3
      - run: |
          ls ${{ github.workspace }}
```
