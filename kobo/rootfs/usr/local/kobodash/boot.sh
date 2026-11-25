#!/bin/sh
# Started by udev early in the boot. It only hands over to the script on the
# user storage, so everything can be changed or disabled over USB.
export PATH=/sbin:/usr/sbin:/bin:/usr/bin

if [ "$1" != "background" ]; then
    # udev must not wait for us.
    setsid /bin/sh "$0" background > /dev/null 2>&1 &
    exit 0
fi

script=/mnt/onboard/.adds/kobodash/autostart.sh
n=0
until [ -f "$script" ]; do
    n=$((n + 1))
    [ $n -gt 120 ] && exit 0
    sleep 1
done
mkdir /tmp/kobodash.lock 2> /dev/null || exit 0
exec /bin/sh "$script"
