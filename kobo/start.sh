#!/bin/sh
# Starts the dashboard in place of the Kobo interface (Nickel), and reboots
# back into it when the dashboard closes. "start.sh test" closes by itself
# after two minutes and logs every touch event.
cd /mnt/onboard/.adds/kobodash || exit 1

# Let NickelMenu finish before Nickel is stopped. Wi-Fi stays as Nickel left it.
sleep 3
killall -q -TERM nickel hindenburg sickel fickel strickel fontickel adobehost foxitpdf iink fmon
sleep 2

if [ "$1" = "test" ]; then
    ./kobodash -max-runtime 2m -touchtest > test.log 2>&1
else
    ./kobodash > kobodash.log 2>&1
fi

sync
reboot
