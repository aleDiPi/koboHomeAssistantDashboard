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
            insmod "$modules/sdio_wifi_pwr.ko"
        else
            ./kobodash -wifi-power on
        fi
    fi
    sleep 1
    if [ -n "$WIFI_MODULE" ] && ! grep -q "^$WIFI_MODULE " /proc/modules; then
        insmod "$modules/$WIFI_MODULE.ko"
    fi
    sleep 1
    ifconfig "$interface" up

    conf=/etc/wpa_supplicant/wpa_supplicant.conf
    [ -f /mnt/onboard/.kobo/wpa_supplicant.conf ] && conf=/mnt/onboard/.kobo/wpa_supplicant.conf
    pidof wpa_supplicant > /dev/null ||
        wpa_supplicant -D wext -i "$interface" -c "$conf" -C /var/run/wpa_supplicant -B
    sleep 3
    if [ -x /sbin/dhcpcd ]; then
        dhcpcd -d -t 30 -w "$interface"
    else
        udhcpc -S -i "$interface" -s /etc/udhcpc.d/default.script -b -q
    fi
}
