#!/bin/bash

CMD="pkgupd_cli -vct unix -p /run/pkgupd/pkgupd.sock"

if [[ "$#" -ge 1 ]]; then
  if [[ "$1" == "update" ]]; then
    ${CMD} --force-sync
  else
    echo 'Invalid argument; use `update` to force a server sync' 1>&2
    exit 1
  fi
else
  ${CMD}
fi
