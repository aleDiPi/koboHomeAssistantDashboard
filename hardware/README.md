# Supporto a muro Kobo Glo + scatola 503 (BTicino Living Light)

Bozza v1 parametrica (OpenSCAD) per stampa FDM (PLA/PETG, senza supporti, stampare a faccia piatta sul piatto).

- `kobo_wall_mount.scad`: sorgente parametrico (`part = "tray" | "adapter" | "assembly"`)
- `kobo_tray.stl`: vano per il Kobo (landscape, USB a destra, power a sinistra, si infila dall'alto)
- `kobo_adapter.stl`: piastra per la scatola 503 (fori a 83,5 mm) con finestra per il caricatore USB
- `preview_assembly.png`: anteprima (grigio = vano, blu = adattatore)

I due pezzi si uniscono con 2 viti M3x10 sulle linguette sovrapposte. Il vano ha anche 2 fori opzionali per tasselli a muro.

## Misure verificate
- Kobo Glo: 114 x 157 x 10 mm (MobileRead, Wikipedia). Interasse fori 503: 83,5 mm.

## Da verificare / misurare prima di stampare (valori `[VERIFICARE]` nel .scad)
- Posizione verticale della porta micro-USB e del tasto power sul bordo del Kobo (`usb_y`, `power_y`).
- Larghezza del bordo cornice del Kobo (il bordo frontale trattiene solo `lip` = 2 mm per non coprire i sensori IR del touch).
- Supporto Living Light: ingombro, spessore, finestra moduli (`sup_*`, `win_*`, `ad_w`, `ad_h`); il codice esatto del caricatore (N4285C2 / N4286C2...) e' da confermare sulla confezione.
- Il pezzo sostituisce la placca Living Light: la finestra espone i moduli, il supporto entra nella cava posteriore.
