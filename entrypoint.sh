#!/bin/sh

crond -f -d /tmp/cron.log &

/usr/local/bin/ms "$@"
