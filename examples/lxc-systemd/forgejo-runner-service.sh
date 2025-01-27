#!/bin/bash
# Copyright Forgejo Authors.
# SPDX-License-Identifier: MIT

set -o pipefail

: ${TMPDIR:=$(mktemp -d)}

export -n TMPDIR

if ! test -d "$TMPDIR"; then
  echo "TMPDIR=$TMPDIR is expected to be a directory"
  exit 1
fi

trap "rm -fr $TMPDIR" EXIT

: ${INPUTS_LXC_CONFIG:=docker libvirt lxc}
: ${INPUTS_SERIAL:=}
: ${INPUTS_TOKEN:=}
: ${INPUTS_FORGEJO:=https://code.forgejo.org}
: ${INPUTS_LIFETIME:=7d}
: ${INPUTS_RUNNER_VERSION:=6.2.0}

: ${KILL_AFTER:=21600} # 6h == 21600
NODEJS_VERSION=20
DEBIAN_RELEASE=bookworm
YQ_VERSION=v4.45.1
SELF=${BASH_SOURCE[0]}
SELF_FILENAME=$(basename "$SELF")
ETC=/etc/forgejo-runner
LIB=/var/lib/forgejo-runner
LOG=/var/log/forgejo-runner
: ${HOST:=$(hostname)}

LXC_IPV4_PREFIX="10.105.7"
LXC_IPV6_PREFIX="fd91"
LXC_USER_NAME=debian
LXC_USER_ID=1000

if ${VERBOSE:-false}; then
  set -ex
  PS4='${BASH_SOURCE[0]}:$LINENO: ${FUNCNAME[0]}:  '
  # export LXC_VERBOSE=true # use with caution, it will block .forgejo/workflows/example-lxc-systemd.yml
else
  set -e
fi

if test $(id -u) != 0; then
  SUDO=sudo
fi

function config_inotify() {
  if grep --quiet fs.inotify.max_user_instances=8192 /etc/sysctl.conf; then
    return
  fi
  echo fs.inotify.max_user_instances=8192 | $SUDO tee -a /etc/sysctl.conf
  $SUDO sysctl -p
}

function dependencies() {
  if ! which curl jq retry >/dev/null; then
    export DEBIAN_FRONTEND=noninteractive
    $SUDO apt-get update -qq
    $SUDO apt-get install -y -qq curl jq retry
  fi
  if ! which lxc-helpers.sh >/dev/null; then
    $SUDO curl --fail -sS -o /usr/local/bin/lxc-helpers-lib.sh https://code.forgejo.org/forgejo/lxc-helpers/raw/branch/main/lxc-helpers-lib.sh
    $SUDO curl --fail -sS -o /usr/local/bin/lxc-helpers.sh https://code.forgejo.org/forgejo/lxc-helpers/raw/branch/main/lxc-helpers.sh
    $SUDO chmod +x /usr/local/bin/lxc-helpers*.sh
  fi
  if ! which lxc-ls >/dev/null; then
    $SUDO lxc-helpers.sh lxc_install_lxc_inside $LXC_IPV4_PREFIX $LXC_IPV6_PREFIX
  fi
  if ! which yq >/dev/null; then
    $SUDO curl -L --fail -sS -o /usr/local/bin/yq https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/yq_linux_arm64
    $SUDO chmod +x /usr/local/bin/yq
  fi
  if ! cmp $SELF /usr/local/bin/$SELF_FILENAME; then
    cp -a $SELF /usr/local/bin/$SELF_FILENAME
  fi
}

function lxc_name() {
  echo runner-${INPUTS_SERIAL}-lxc
}

function lxc_destroy() {
  $SUDO lxc-destroy -f $(lxc_name) >/dev/null || true
}

function lxc_create() {
  local name=$(lxc_name)
  local lib=$LIB/$name
  local etc=$ETC/$INPUTS_SERIAL

  lxc-helpers.sh --config "$INPUTS_LXC_CONFIG" lxc_container_create $name
  echo "lxc.start.auto = 1" | sudo tee -a /var/lib/lxc/$name/config

  local bin=/var/lib/lxc/$name/rootfs/usr/local/bin
  $SUDO cp -a $SELF $bin/$SELF_FILENAME
  $SUDO cp -a /usr/local/bin/forgejo-runner-$INPUTS_RUNNER_VERSION $bin/forgejo-runner
  $SUDO cp -a /usr/local/bin/yq $bin/yq
  $SUDO cp -a $(which jq) $bin/jq

  $SUDO mkdir -p $lib/.cache/actcache
  $SUDO chown -R $LXC_USER_ID $lib
  lxc-helpers.sh lxc_container_mount $name $lib/.cache/actcache

  $SUDO mkdir -p $etc
  $SUDO chown -R $LXC_USER_ID $etc
  lxc-helpers.sh lxc_container_mount $name $etc

  lxc-helpers.sh lxc_container_start $name
  if echo $INPUTS_LXC_CONFIG | grep --quiet 'docker'; then
    lxc-helpers.sh lxc_install_docker $name
  fi
  if echo $INPUTS_LXC_CONFIG | grep --quiet 'lxc'; then
    local ipv4="10.48.$INPUTS_SERIAL"
    local ipv6="fd$INPUTS_SERIAL"
    lxc-helpers.sh lxc_install_lxc $name $ipv4 $ipv6
  fi
  lxc-helpers.sh lxc_container_user_install $name $LXC_USER_ID $LXC_USER_NAME
}

function service_create() {
  cat >$TMPDIR/forgejo-runner@.service <<EOF
[Unit]
Description=Forgejo runner %i
After=syslog.target
After=network.target

[Service]
Restart=on-success
ExecStart=/usr/local/bin/${SELF_FILENAME} run_in_copy start
ExecStop=/usr/local/bin/${SELF_FILENAME} stop
EnvironmentFile=/etc/forgejo-runner/%i/env

[Install]
WantedBy=multi-user.target
EOF

  local service=/etc/systemd/system/forgejo-runner@.service
  if test -f $service && cmp $TMPDIR/forgejo-runner@.service $service; then
    return
  fi

  $SUDO mkdir -p $ETC
  $SUDO chown -R $LXC_USER_ID $ETC

  $SUDO mkdir -p $LOG
  $SUDO chown -R $LXC_USER_ID $LOG

  $SUDO cp $TMPDIR/forgejo-runner@.service $service
  $SUDO systemctl daemon-reload
}

function inside() {
  local name=$(lxc_name)

  lxc-helpers.sh lxc_container_run $name -- sudo --user $LXC_USER_NAME \
    INPUTS_SERIAL="$INPUTS_SERIAL" \
    INPUTS_LXC_CONFIG="$INPUTS_LXC_CONFIG" \
    INPUTS_TOKEN="$INPUTS_TOKEN" \
    INPUTS_FORGEJO="$INPUTS_FORGEJO" \
    INPUTS_LIFETIME="$INPUTS_LIFETIME" \
    KILL_AFTER="$KILL_AFTER" \
    VERBOSE="$VERBOSE" \
    HOST="$HOST" \
    $SELF_FILENAME "$@"
}

function install_runner() {
  local runner=/usr/local/bin/forgejo-runner-$INPUTS_RUNNER_VERSION
  if test -f $runner; then
    return
  fi

  $SUDO curl --fail -sS -o $runner https://code.forgejo.org/forgejo/runner/releases/download/v$INPUTS_RUNNER_VERSION/forgejo-runner-$INPUTS_RUNNER_VERSION-linux-amd64
  $SUDO chmod +x $runner
}

function ensure_configuration() {
  if test -z "$INPUTS_SERIAL"; then
    echo "the INPUTS_SERIAL environment variable is not set"
    return 1
  fi

  local etc=$ETC/$INPUTS_SERIAL
  $SUDO mkdir -p $etc

  if test -f $etc/config; then
    INPUTS_LXC_CONFIG=$(cat $etc/config)
  else
    echo $INPUTS_LXC_CONFIG >$etc/config
  fi

  $SUDO mkdir -p $LIB/$(lxc_name)/.cache/actcache
}

function ensure_configuration_and_registration() {
  local etc=$ETC/$INPUTS_SERIAL

  if ! test -f $etc/config.yml; then
    forgejo-runner generate-config >$etc/config.yml
    cat >$TMPDIR/edit-config <<EOF
.runner.labels = ["docker:docker://data.forgejo.org/oci/node:${NODEJS_VERSION}-${DEBIAN_RELEASE}","lxc:lxc://debian:${DEBIAN_RELEASE}"]
EOF
    yq --inplace --from-file $TMPDIR/edit-config $etc/config.yml
    cat >$TMPDIR/edit-config <<EOF
.cache.dir = "/var/lib/forgejo-runner/runner-${INPUTS_SERIAL}-lxc/.cache/actcache"
EOF
    yq --inplace --from-file $TMPDIR/edit-config $etc/config.yml

  fi

  if ! test -f $etc/env; then
    cat >$etc/env <<EOF
INPUTS_LXC_CONFIG=$INPUTS_LXC_CONFIG
INPUTS_SERIAL=$INPUTS_SERIAL
INPUTS_LIFETIME=$INPUTS_LIFETIME
INPUTS_FORGEJO=$INPUTS_FORGEJO
EOF
  fi

  if test -f $etc/.runner; then
    return
  fi
  if test -z "$INPUTS_TOKEN"; then
    echo "the INPUTS_TOKEN environment variable is not set"
    return 1
  fi
  (
    cd $etc
    forgejo-runner register --config config.yml --no-interactive \
      --token "$INPUTS_TOKEN" \
      --name "$HOST-$INPUTS_SERIAL" \
      --instance $INPUTS_FORGEJO
  )
}

function daemon() {
  cd $ETC/$INPUTS_SERIAL
  rm -f stopped-* killed-*
  touch started-running
  set +e
  timeout --signal=SIGINT --kill-after=$KILL_AFTER $INPUTS_LIFETIME forgejo-runner --config config.yml daemon
  case $? in
  0) touch stopped-gracefully ;;
  124) touch stopped-timeout ;;
  137) touch stopped-forcefully ;;
  esac
  set -e
}

function start() {
  stop
  lxc-helpers.sh lxc_container_destroy $(lxc_name)
  lxc_create

  local log=$LOG/$INPUTS_SERIAL.log
  if test -f $log; then
    mv $log $log.backup
  fi
  inside daemon >&$log
}

function kill_runner() {
  cd $ETC/$INPUTS_SERIAL
  rm -f killed-* started-running

  set +e
  pkill --exact forgejo-runner
  if test $? = 1; then
    touch killed-already
    return
  fi

  timeout $KILL_AFTER pidwait --exact forgejo-runner
  status=$?
  set -e

  # pidwait will exit 1 if the process is already gone
  # pidwait will exit 0 if the process terminated gracefully before the timeout
  if test $status = 0 || test $status = 1; then
    touch killed-gracefully
    echo "forgejo-runner stopped gracefully"
  else
    pkill --exact --signal=KILL forgejo-runner
    touch killed-forcefully
    echo "forgejo-runner stopped forcefully"
  fi
}

function stop() {
  inside kill_runner
}

function main() {
  config_inotify
  dependencies
  install_runner
  service_create
  lxc_create
  inside ensure_configuration_and_registration
}

#
# ensure an update of the current script does not break a long
# running function (such as `start`) by running from a copy instead
# of the script itself
#
function run_in_copy() {
  if test "$#" = 0; then
    echo "run_in_copy needs an argument"
    return 1
  fi

  export TMPDIR # otherwise it will not be removed by trap
  cp $SELF $TMPDIR/$SELF_FILENAME
  exec $TMPDIR/$SELF_FILENAME "$@"
}

"${@:-main}"
