#!/bin/sh
# Starts the dashboard in place of the Kobo interface (Nickel), and reboots
# back into Nickel when the dashboard closes.
export PATH=/sbin:/usr/sbin:/bin:/usr/bin
cd /mnt/onboard/.adds/kobodash || exit 1

# Learn the Wi-Fi hardware from the running system, while Nickel is alive.
import_env() {
    pid=$(pidof "$1" | cut -d' ' -f1)
    [ -n "$pid" ] || return
    for kv in $(tr '\0' '\n' < "/proc/$pid/environ" | grep -E "^($2)="); do
        export "$kv"
    done
}
import_env nickel 'INTERFACE|WIFI_MODULE'
import_env udevd 'PLATFORM'

# Let NickelMenu finish before Nickel is stopped.
sleep 3
killall -q -TERM nickel hindenburg sickel fickel strickel fontickel adobehost foxitpdf iink fmon
sleep 2

. ./wifi.sh
wifi_up > wifi.log 2>&1

[ -f kobodash.log ] && mv kobodash.log kobodash.previous.log
./kobodash > kobodash.log 2>&1

# Whatever ended the dashboard, the next boot goes to the Kobo interface, so
# that "Esci" really leaves and a crash cannot loop.
touch skip-once
sync
reboot
