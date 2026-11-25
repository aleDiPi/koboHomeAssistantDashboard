// Supporto a muro per Kobo Glo (N613) + adattatore per scatola 503 / BTicino Living Light
// Orientamento: landscape, micro-USB sul lato DESTRO, tasto power sul lato SINISTRO.
// Sistema di coordinate: x = destra, y = su, z = esce dal muro. Unità: mm.
//
// Due pezzi stampabili (ciascuno entra in un piatto 220x220):
//   tray()    = vano per il Kobo (aperto in alto: il Kobo si infila dall'alto)
//   adapter() = piastra che si avvita alla 503 sopra il supporto Living Light
// I due pezzi si uniscono con 2 viti M3x10 attraverso le linguette sovrapposte.
//
// !! Valori marcati [VERIFICARE] sono stime: misurali sul tuo hardware prima di stampare.

part = "assembly";   // "assembly" | "tray" | "adapter"

/* ---- Kobo Glo (fonti: MobileRead/Wikipedia 114 x 157 x 10) ---- */
kobo_w = 157;        // lato lungo (landscape)
kobo_h = 114;        // lato corto
kobo_t = 10;         // spessore
clr    = 0.6;        // gioco per lato
lip    = 2.0;        // sovrapposizione del bordo frontale: MAI di piu', sotto ci sono i sensori IR
usb_y     = 0;       // [VERIFICARE] offset verticale porta USB rispetto al centro (+ = su)
usb_w     = 18;      // larghezza apertura per spinotto (consigliato spinotto a 90 gradi)
power_y   = 0;       // [VERIFICARE] offset verticale tasto power rispetto al centro, lato sinistro
power_w   = 30;      // apertura per raggiungere il tasto

/* ---- Struttura ---- */
wall   = 2.4;        // pareti
back   = 3.0;        // fondo vano
lip_t  = 1.6;        // spessore del bordo frontale
tab_l  = 16;         // lunghezza linguette di unione
tab_t  = 2.5;        // spessore di ciascuna linguetta

/* ---- Scatola 503 / Living Light ---- */
hole_pitch = 83.5;   // interasse fori 503 (standard)
ad_w   = 106;        // [VERIFICARE] larghezza piastra adattatore
ad_h   = 76;         // [VERIFICARE] altezza piastra adattatore
sup_t  = 6;          // [VERIFICARE] spessore supporto Living Light che entra nella cava posteriore
sup_w  = 96;         // [VERIFICARE] ingombro supporto (cava posteriore)
sup_h  = 62;         // [VERIFICARE]
win_w  = 67.5;       // [VERIFICARE] finestra moduli (3 moduli)
win_h  = 49;         // [VERIFICARE]
win_dx = -10;        // spostamento finestra verso il Kobo (cavo corto)
screw_d = 4.5;       // asola per viti 503
head_d  = 8.5;

tw = kobo_w + 2*clr + 2*wall;
th = kobo_h + 2*clr + 2*wall;
pd = kobo_t + clr;
tdepth = back + pd + lip_t;
ad_t = sup_t + 2.5;

module tray() {
  difference() {
    union() {
      cube([tw, th, tdepth]);
      // linguetta di unione (lato muro)
      translate([tw, th/2-30, 0]) cube([tab_l, 60, tab_t]);
    }
    // vano Kobo, aperto in alto
    translate([wall, wall, back]) cube([kobo_w+2*clr, kobo_h+2*clr+wall+1, pd]);
    // finestra frontale (bordo che trattiene solo 'lip' mm)
    translate([wall+lip, wall+lip, back+pd-0.01]) cube([kobo_w+2*clr-2*lip, th, lip_t+1]);
    // apertura USB (destra) e tasto power (sinistra)
    translate([tw-wall-lip, th/2+usb_y-usb_w/2, back]) cube([wall+lip+1, usb_w, tdepth]);
    translate([-1, th/2+power_y-power_w/2, back]) cube([wall+lip+1, power_w, tdepth]);
    // viti M3 per unione
    for (s=[-1,1]) translate([tw+tab_l/2, th/2+s*22, -1]) cylinder(d=2.7, h=tab_t+2, $fn=24);
    // 2 fori opzionali per fissare il vano al muro (tassello)
    for (s=[-1,1]) translate([wall+14, th/2+s*(th/2-wall-10), -1]) {
      cylinder(d=4.2, h=back+2, $fn=24);
      translate([0,0,back-1.5]) cylinder(d1=4.2,d2=8.5,h=1.6,$fn=24);
    }
  }
}

module adapter() {
  difference() {
    union() {
      translate([tab_l, 0, 0]) cube([ad_w, ad_h, ad_t]);
      // linguetta sovrapposta (sopra quella del vano)
      translate([0, ad_h/2-30, tab_t]) cube([tab_l+1, 60, tab_t]);
    }
    // cava posteriore per il supporto Living Light
    translate([tab_l+(ad_w-sup_w)/2, (ad_h-sup_h)/2, -1]) cube([sup_w, sup_h, sup_t+1]);
    // finestra per i moduli
    translate([tab_l+(ad_w-win_w)/2+win_dx, (ad_h-win_h)/2, -1]) cube([win_w, win_h, ad_t+2]);
    // asole viti 503 con svasatura
    for (s=[-1,1]) translate([tab_l+ad_w/2+s*hole_pitch/2, ad_h/2, -1]) {
      cylinder(d=screw_d, h=ad_t+2, $fn=32);
      translate([0,0,sup_t+1.2]) cylinder(d=head_d, h=ad_t, $fn=32);
    }
    // viti M3 unione
    for (s=[-1,1]) translate([tab_l/2, ad_h/2+s*22, tab_t-1]) cylinder(d=3.4, h=tab_t+2, $fn=24);
  }
}

if (part == "tray") tray();
else if (part == "adapter") adapter();
else {
  color("#cfd8dc") tray();
  color("#90caf9") translate([tw, th/2-ad_h/2, 0]) adapter();
  // sagoma Kobo (solo anteprima)
  %translate([wall+clr, wall+clr, back]) cube([kobo_w, kobo_h, kobo_t]);
}
