# Kobo Home Assistant Dashboard

Pannello touch per Home Assistant su un Kobo Glo (N613). Un programma Go statico
disegna direttamente sul framebuffer e-ink, legge il touch e riceve gli stati in
tempo reale dalla WebSocket API.

Stato: prima versione (luci, condizionatori/termostati, sensori), provata su un
Glo con firmware 4.20.14622: schermo, touch e comandi funzionano.

## Installazione sul Kobo

1. Installa [NickelMenu](https://github.com/pgaskin/NickelMenu).
2. Copia in `.adds/kobodash/`: `dist/kobodash`, `kobo/start.sh`, `kobo/wifi.sh`,
   `kobo/autostart.sh` e `dashboard.ini`.
3. Copia `kobo/nickelmenu` come `.adds/nm/kobodash`.
4. Apri **Dashboard casa** dal menu. **Esci** (due tocchi) riavvia il Kobo.

### Avvio automatico (facoltativo)

Copia `dist/KoboRoot.tgz` in `.kobo/` ed espelli il Kobo: installa una regola
udev che a ogni accensione esegue `autostart.sh`. Controlla prima che in `.kobo/`
non ci sia già un `KoboRoot.tgz` (un aggiornamento in attesa).

- Si attiva creando il file vuoto `.adds/kobodash/autostart`, si disattiva
  cancellandolo.
- La dashboard parte circa 30 secondi dopo l'accensione.
- Dopo **Esci**, o dopo un errore, l'avvio successivo resta nell'interfaccia Kobo.

## Avvisi

Le entità in `[notifications]` compaiono come etichette nell'intestazione solo
quando sono `on` o `open`; quelle che non ci stanno sono contate in "+N".

## Luce dello schermo

Sezione `[frontlight]` di `dashboard.ini`: spenta di giorno; dopo il tramonto al
minimo se le luci indicate sono spente, più forte per qualche secondo dopo un
tocco.

## Provarla sul PC

Con dati finti:

```powershell
go run ./cmd/kobodash -config dashboard.example.ini -demo -sim 127.0.0.1:8080
```

Poi apri <http://127.0.0.1:8080>: un clic vale come un tocco. Senza `-demo` usa
l'istanza indicata nel file di configurazione e **comanda davvero i dispositivi**.

## Compilare per il Kobo

```powershell
.\build.ps1
```

Serve **Go 1.23**: dalla 1.24 il kernel minimo è Linux 3.2, più recente di quello
del Glo.

## Configurazione

Copia `dashboard.example.ini` in `dashboard.ini` accanto al programma. Il token
resta solo in quel file, escluso da git.

## Struttura

| Cartella | Contenuto |
| --- | --- |
| `cmd/kobodash` | avvio e opzioni |
| `internal/ha` | client WebSocket di Home Assistant |
| `internal/ui` | disposizione e disegno |
| `internal/app` | tocchi, aggiornamenti parziali, comandi |
| `internal/screen` | framebuffer ed EPDC (i.MX508) |
| `internal/touch` | eventi di input |
| `internal/sim` | simulatore nel browser |

`kobo/` contiene lo script di avvio e le voci per NickelMenu. Sul dispositivo
vanno in `.adds/kobodash/` (con il binario e `dashboard.ini`) e `.adds/nm/`.

La batteria viene annotata ogni 10 minuti in `kobodash.log`.

Opzione `-touchtest`: registra gli eventi grezzi del touch, per la calibrazione.
