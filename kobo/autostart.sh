#!/bin/sh
# Run at every boot by /usr/local/kobodash/boot.sh. Autostart is on while the
# file "autostart" exists next to this script: delete it to turn it off.
export PATH=/sbin:/usr/sbin:/bin:/usr/bin
cd /mnt/onboard/.adds/kobodash || exit 0
[ -f autostart ] || exit 0

if [ -f skip-once ]; then
    # The dashboard was closed: stay in the Kobo interface this time.
    rm -f skip-once
    exit 0
fi

n=0
until pidof nickel > /dev/null; do
    n=$((n + 1))
    [ $n -gt 120 ] && exit 0
    sleep 1
done
# Time for the Kobo to settle, and to connect a USB cable if needed.
sleep 30

# With USB storage connected these files are not reachable any more.
cd /mnt/onboard/.adds/kobodash 2> /dev/null || exit 0
[ -f autostart ] && [ -f start.sh ] && exec sh start.sh
