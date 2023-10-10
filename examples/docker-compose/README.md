## Docker compose with docker-in-docker

The `compose-forgejo-and-runner.yml` compose file runs a Forgejo
instance and registers a `Forgejo runner`. A docker server is also
launched within a container (using
[dind](https://hub.docker.com/_/docker/tags?name=dind)) and will be
used by the `Forgejo runner` to execute the workflows.

### Quick start

```sh
rm -fr /srv/runner-data /srv/forgejo-data
secret=$(openssl rand -hex 20)
sed -i -e "s/{SHARED_SECRET}/$secret/" compose-forgejo-and-runner.yml
docker compose -f compose-forgejo-and-runner.yml up -d
docker compose -f compose-forgejo-and-runner.yml -f compose-demo-workflow.yml up demo-workflow
firefox http://0.0.0.0:8080/root/test/actions/runs/1 # login root, password {ROOT_PASSWORD}
```

### Running

Create a shared secret with:

```sh
openssl rand -hex 20
```

Replace all occurences of {SHARED_SECRET} in
[compose-forgejo-and-runner.yml](compose-forgejo-and-runner.yml).

> **NOTE:** a token obtained from the Forgejo web interface cannot be used as a shared secret.

Replace {ROOT_PASSWORD} with a secure password in
[compose-forgejo-and-runner.yml](compose-forgejo-and-runner.yml).

```sh
docker-compose -f compose-forgejo-and-runner.yml up
Creating docker-compose_docker-in-docker_1 ... done
Creating docker-compose_forgejo_1          ... done
Creating docker-compose_runner-register_1  ... done
...
docker-in-docker_1  | time="2023-08-24T10:22:15.023338461Z" level=warning msg="WARNING: API is accessible on http://0.0.0.0:2375
...
forgejo_1           | 2023/08/24 10:22:14 ...s/graceful/server.go:75:func1() [D] Starting server on tcp:0.0.0.0:3000 (PID: 19)
...
runner-daemon_1     | time="2023-08-24T10:22:16Z" level=info msg="Starting runner daemon"
```

### Manual testing

To login the Forgejo instance:

* URL: http://0.0.0.0:8080
* user: root
* password: {ROOT_PASSWORD}

`Forgejo Actions` is enabled by default when creating a repository.

## Tests workflow

The `compose-demo-workflow.yml` compose file runs a demo workflow to
verify the `Forgejo runner` can pick up a task from the Forgejo instance
and run it to completion.

A new repository is created in root/test with the following workflow
in `.forgejo/workflows/demo.yml`:

```yaml
on: [push]
jobs:
  test:
    runs-on: docker
    steps:
      - run: echo All Good
```

A wait loop expects the status of the check associated with the
commit in Forgejo to show "success" to assert the workflow was run.

### Running

```sh
$ docker-compose -f compose-forgejo-and-runner.yml -f compose-demo-workflow.yml up demo-workflow
...
demo-workflow_1     | To http://forgejo:3000/root/test
demo-workflow_1     |  + 5ce134e...261cc79 main -> main (forced update)
demo-workflow_1     | branch 'main' set up to track 'http://root:admin1234@forgejo:3000/root/test/main'.
...
demo-workflow_1     | running
...
```
