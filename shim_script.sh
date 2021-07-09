#!/bin/bash

if [[ $# -lt 1 ]]; then
  echo 'you need to provide some parameters'
  exit 1
fi

echo $@ >> /var/log/cpbackuptrans.log

/usr/local/lp/bin/S3CompatibleMultithread $@