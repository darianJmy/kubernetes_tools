#!/bin/bash
set -x
count=`ss -lntp | grep haproxy | wc -l`
if [ $count > 0 ]; then
    exit 0
else
    exit 1
fi
