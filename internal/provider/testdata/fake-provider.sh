#!/bin/sh
if [ "$1" = "--version" ]; then
  printf 'fake-provider 0.0.1\n'
  exit 0
fi
printf 'fake provider completed\n'
exit 0
