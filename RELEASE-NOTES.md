# Release Notes

## 5.0.0

* [Add job_level logging option to config](https://code.forgejo.org/forgejo/runner/pulls/299) to make the logging level of jobs configurable. Change default from "trace" to "info".
* [Don't log job output when debug logging is not enabled](https://code.forgejo.org/forgejo/runner/pulls/303). This reduces the default amount of log output of the runner.

## 4.0.1

* Do not panic when [the number of arguments of a function evaluated in an expression is incorect](https://code.forgejo.org/forgejo/act/pulls/59/files).

## 4.0.0

* Breaking change: fix the default configuration for `docker_host` is changed to [not mounting the docker server socket](https://code.forgejo.org/forgejo/runner/pulls/305).
* [Remove debug information from the setup of a workflow](https://code.forgejo.org/forgejo/runner/pulls/297).
* Fix [crash in some cases when the YAML structure is not as expected](https://code.forgejo.org/forgejo/runner/issues/267).

## 3.5.1

* Fix [CVE-2024-24557](https://nvd.nist.gov/vuln/detail/CVE-2024-24557)
* [Add report_interval option to config](https://code.forgejo.org/forgejo/runner/pulls/220) to allow setting the interval of status and log reports

## 3.5.0

* [Allow graceful shutdowns](https://code.forgejo.org/forgejo/runner/pulls/202): when receiving a signal (INT or TERM) wait for running jobs to complete (up to shutdown_timeout).
* [Fix label declaration](https://code.forgejo.org/forgejo/runner/pulls/176): Runner in daemon mode now takes labels found in config.yml into account when declaration was successful.
* [Fix the docker compose example](https://code.forgejo.org/forgejo/runner/pulls/175) to workaround the race on labels.
* [Fix the kubernetes dind example](https://code.forgejo.org/forgejo/runner/pulls/169).
* [Rewrite ::group:: and ::endgroup:: commands like github](https://code.forgejo.org/forgejo/runner/pulls/183).
* [Added opencontainers labels to the image](https://code.forgejo.org/forgejo/runner/pulls/195)
* [Upgrade the default container to node:20](https://code.forgejo.org/forgejo/runner/pulls/203)

## 3.4.1

* Fixes a regression introduced in 3.4.0 by which a job with no image explicitly set would
  [be bound to the host](https://code.forgejo.org/forgejo/runner/issues/165)
  network instead of a custom network (empty string in the configuration file).

## 3.4.0

Although this version is able to run [actions/upload-artifact@v4](https://code.forgejo.org/actions/upload-artifact/src/tag/v4) and [actions/download-artifact@v4](https://code.forgejo.org/actions/download-artifact/src/tag/v4), these actions will fail because it does not run against GitHub.com. A fork of those two actions with this check disabled is made available at:

* https://code.forgejo.org/forgejo/upload-artifact/src/tag/v4
* https://code.forgejo.org/forgejo/download-artifact/src/tag/v4

and they can be used as shown in [an example from the end-to-end test suite](https://code.forgejo.org/forgejo/end-to-end/src/branch/main/actions/example-artifacts-v4/.forgejo/workflows/test.yml).

* When running against codeberg.org, the default poll frequency is 30s instead of 2s.
* Fix compatibility issue with actions/{upload,download}-artifact@v4.
* Upgrade ACT v1.20.0 which brings:
  * `[container].options` from the config file is exposed in containers created by the workflows
  * the expressions in the value of `jobs.<job-id>.runs-on` are evaluated
  * fix a bug causing the evaluated expression of `jobs.<job-id>.runs-on` to fail if it was an array
  * mount `act-toolcache:/opt/hostedtoolcache` instead of `act-toolcache:/toolcache`
  * a few improvements to the readability of the error messages displayed in the logs
  * `amd64` can be used instead of `x86_64` and `arm64` intead of `aarch64` when specifying the architecture
  * fixed YAML parsing bugs preventing dispatch workflows to be parsed correctly
  * add support for `runs-on.labels` which is equivalent to `runs-on` followed by a list of labels
  * the expressions in the service `ports` and `volumes` values are evaluated
  * network aliases are only supported when the network is user specified, not when it is provided by the runner
* If `[runner].insecure` is true in the configuration, insecure cloning actions is allowed

## 3.3.0

* Support IPv6 with addresses from a private range and NAT for
    docker:// with --enable-ipv6 and [container].enable_ipv6
    lxc:// always

## 3.2.0

* Support LXC container capabilities via `lxc:lxc://debian:bookworm:k8s` or  `lxc:lxc://debian:bookworm:docker lxc k8s`
* Update ACT v1.16.0 to resolve a [race condition when bootstraping LXC templates](https://code.forgejo.org/forgejo/act/pulls/23)

## 3.1.0

The `self-hosted` label that was hardwired to be a LXC container
running `debian:bullseye` was reworked and documented ([user guide](https://forgejo.org/docs/next/user/actions/#jobsjob_idruns-on) and [admin guide](https://forgejo.org/docs/next/admin/actions/#labels-and-runs-on)).

There now are two different schemes: `lxc://` for LXC containers and
`host://` for running directly on the host.

* Support the `host://` scheme for running directly on the host.
* Support the `lxc://` scheme in labels
* Update [code.forgejo.org/forgejo/act v1.14.0](https://code.forgejo.org/forgejo/act/pulls/19) to implement both self-hosted and LXC schemes

## 3.0.3

* Update [code.forgejo.org/forgejo/act v1.13.0](https://code.forgejo.org/forgejo/runner/pulls/106) to keep up with github.com/nektos/act

## 3.0.2

* Update [code.forgejo.org/forgejo/act v1.12.0](https://code.forgejo.org/forgejo/runner/pulls/106) to upgrade the node installed in the LXC container to node20

## 3.0.1

* Update [code.forgejo.org/forgejo/act v1.11.0](https://code.forgejo.org/forgejo/runner/pulls/86) to resolve a bug preventing actions based on node20 from running, such as [checkout@v4](https://code.forgejo.org/actions/checkout/src/tag/v4).

## 3.0.0

* Publish a rootless OCI image
* Refactor the release process

## 2.5.0

* Update [code.forgejo.org/forgejo/act v1.10.0](https://code.forgejo.org/forgejo/runner/pulls/71)

## 2.4.0

* Update [code.forgejo.org/forgejo/act v1.9.0](https://code.forgejo.org/forgejo/runner/pulls/64)

## 2.3.0

* Add support for [offline registration](https://forgejo.org/docs/next/admin/actions/#offline-registration).
