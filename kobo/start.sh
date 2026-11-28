#!/bin/sh
# Starts the dashboard in place of the Kobo interface (Nickel), and reboots
# back into Nickel when the dashboard closes.
#
# Everything is inside main, so that the shell has read the whole script before
# an update replaces the file.
main() {
    export PATH=/sbin:/usr/sbin:/bin:/usr/bin
    cd /mnt/onboard/.adds/kobodash || exit 1

    # A version installed over the air that never proved to work (it hung and
    # the Kobo was switched off) is undone.
    [ "$1" != "updated" ] && [ -f update-pending ] && restore

    # Learn the Wi-Fi hardware from the running system, while Nickel is alive.
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
    code=$?

    # 42: a new version was installed, run it (and its own launcher).
    if [ $code -eq 42 ]; then
        exec /bin/sh ./start.sh updated
    fi
    # A new version that closes before proving to work is replaced by the
    # previous one, which starts at once.
    if [ -f update-pending ]; then
        restore
        echo "$(date) update undone, exit code $code" >> update.log
        exec /bin/sh ./start.sh
    fi

    # Whatever ended the dashboard, the next boot goes to the Kobo interface,
    # so that "Esci" really leaves and a crash cannot loop.
    touch skip-once
    sync
    reboot
}

import_env() {
    pid=$(pidof "$1" | cut -d' ' -f1)
    [ -n "$pid" ] || return
    for kv in $(tr '\0' '\n' < "/proc/$pid/environ" | grep -E "^($2)="); do
        export "$kv"
    done
}

restore() {
    for f in kobodash start.sh wifi.sh autostart.sh; do
        [ -f "$f.old" ] && mv -f "$f.old" "$f"
    done
    rm -f update-pending
}

main "$@"
exit
