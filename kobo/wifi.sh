# Brings Wi-Fi up if Nickel had not already done so. Sourced by start.sh; the
# steps follow KOReader's platform/kobo/enable-wifi.sh.
wifi_up() {
    interface=${INTERFACE:-eth0}
    echo "interface=$interface module=$WIFI_MODULE platform=$PLATFORM"
    if grep -q "^$interface[[:space:]]*00000000" /proc/net/route; then
        echo "already connected"
        return 0
    fi

    modules=/drivers/${PLATFORM:-ntx508}/wifi
    if ! grep -q "^sdio_wifi_pwr " /proc/modules; then
        if [ -e "$modules/sdio_wifi_pwr.ko" ]; then
            insmod "$modules/sdio_wifi_pwr.ko" && echo "power module loaded"
        else
            ./kobodash -wifi-power on
        fi
    fi
    sleep 1
    if [ -n "$WIFI_MODULE" ] && ! grep -q "^$WIFI_MODULE " /proc/modules; then
        insmod "$modules/$WIFI_MODULE.ko" && echo "driver loaded"
    fi
    wait_for 10 test -e "/sys/class/net/$interface" || echo "no $interface"
    ifconfig "$interface" up
    # The Broadcom chip of the Glo stays deaf until told to come up: without
    # this wpa_supplicant scans forever (KOReader does the same).
    broadcom up

    conf=/etc/wpa_supplicant/wpa_supplicant.conf
    [ -f /mnt/onboard/.kobo/wpa_supplicant.conf ] && conf=/mnt/onboard/.kobo/wpa_supplicant.conf
    # A supplicant left over from before would be tied to the old interface.
    killall -q wpa_supplicant
    sleep 1
    wpa_supplicant -D wext -i "$interface" -c "$conf" -C /var/run/wpa_supplicant -B
    wait_for 20 associated || echo "not associated"

    killall -q udhcpc dhcpcd
    sleep 1
    if [ -x /sbin/dhcpcd ]; then
        dhcpcd -t 30 -w "$interface"
    else
        udhcpc -S -i "$interface" -s /etc/udhcpc.d/default.script -t 15 -b -q
    fi
    if wait_for 15 grep -q "^$interface[[:space:]]*00000000" /proc/net/route; then
        echo "connected"
    else
        echo "no route"
        wpa_cli -i "$interface" status
        return 1
    fi
}

# broadcom up|down: only for the "dhd" driver.
broadcom() {
    [ "$WIFI_MODULE" = "dhd" ] || return 0
    wlarm_le -i "${INTERFACE:-eth0}" "$1" || echo "wlarm_le $1 failed"
}

# wait_for SECONDS COMMAND...: retries the command once a second.
wait_for() {
    tries=$1
    shift
    until "$@" > /dev/null 2>&1; do
        tries=$((tries - 1))
        [ "$tries" -gt 0 ] || return 1
        sleep 1
    done
}

associated() {
    wpa_cli -i "${INTERFACE:-eth0}" status | grep -q "^wpa_state=COMPLETED"
}

# Takes Wi-Fi down completely before a suspend. Refuses (and so cancels the
# suspend) if the driver cannot be unloaded: sleeping with it loaded can crash
# the kernel.
wifi_down() {
    interface=${INTERFACE:-eth0}
    if [ -z "$WIFI_MODULE" ]; then
        echo "unknown Wi-Fi driver"
        return 1
    fi
    killall -q udhcpc default.script dhcpcd
    wpa_cli -i "$interface" terminate
    killall -q wpa_supplicant
    broadcom down
    ifconfig "$interface" down
    sleep 1
    if grep -q "^$WIFI_MODULE " /proc/modules; then
        rmmod "$WIFI_MODULE" || return 1
    fi
    if grep -q "^sdio_wifi_pwr " /proc/modules; then
        rmmod sdio_wifi_pwr
    else
        ./kobodash -wifi-power off
    fi
    echo "down"
}

# Lets the radio doze between packets ("on") or not ("off").
wifi_powersave() {
    command -v iwconfig > /dev/null || { echo "no iwconfig"; return 1; }
    iwconfig "${INTERFACE:-eth0}" power "$1"
}
