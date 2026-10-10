#!/bin/sh
# Complete a capture while a helper retains the inherited stderr descriptor.
/bin/sleep 4 &
for destination in "$@"; do :; done
/bin/cp "$(/usr/bin/dirname "$0")/fixture.png" "$destination"
