# Release Notes

## 3.4.0

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
