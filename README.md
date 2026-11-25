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

## Schermate da Home Assistant

Con `dashboard = dashboard-kobo` in `[homeassistant]` le schermate si modificano
dall'interfaccia di Home Assistant, senza cavo:

- ogni **vista** della dashboard è una pagina (al massimo 6), nell'ordine in cui
  appare;
- di ogni **scheda** contano le entità, i nomi e le icone: sensori → grafico,
  luci/interruttori/scene → pulsante, climate → controllo clima,
  cover/binary_sensor → finestra o stato, calendar → calendario;
- i **badge** delle viste diventano le etichette di avviso.

Script, scene e pulsanti diventano pulsanti di avvio (gli script chiedono un
secondo tocco). Per mostrare il tempo rimanente di uno script con un'attesa,
aggiungi alla sua scheda, dall'editor YAML:

```yaml
kobo_timer: input_number.minuti_di_attesa   # oppure un numero di minuti
```

Il conto parte da quando lo script è stato avviato, anche se non dal Kobo.

Le modifiche arrivano al Kobo appena salvate. Se la dashboard non si può
leggere, valgono le liste di `dashboard.ini`.

## Schermate

I pulsanti in basso passano tra **Casa** (grafici delle temperature e
calendario), **Luci** e **Clima**. Dopo `return_home` senza tocchi si torna a
Casa. I calendari si elencano nella sezione `[calendar]`.

La pagina Clima mostra anche le finestre di `[windows]`; **Apri** chiede un
secondo tocco di conferma. Le luci usano le icone di Home Assistant (Material
Design Icons, vedi `internal/ui/mdi/LICENSE.txt`).

## Avvisi

Le entità in `[notifications]` compaiono come etichette nell'intestazione solo
quando sono `on` o `open`; quelle che non ci stanno sono contate in "+N".

## Standby

Con `standby = true` in `[power]`:

| Situazione | Comportamento |
| --- | --- |
| In casa, alimentato | sempre acceso |
| In casa, a batteria | dorme; si aggiorna ogni `home_refresh` |
| Nessuno in casa | dorme; si aggiorna ogni `away_refresh` |

Con `saving = true`, a batteria anche da sveglio il processore usa un regime
più parsimonioso e il Wi-Fi sonnecchia tra un pacchetto e l'altro.

Mentre dorme mostra una schermata di standby con temperature e avvisi; alla
pressione del tasto compare subito "Mi sto riattivando…".

Dorme dopo `idle` senza tocchi. Mentre dorme il touch non risponde: premi il
tasto di accensione. Lo schermo mostra "Standby" con l'ora dell'ultimo
aggiornamento.

## Sensori in Home Assistant

Con `report = true` compaiono `sensor.kobo_batteria`,
`binary_sensor.kobo_alimentazione`, `sensor.kobo_modalita` e
`sensor.kobo_luce`, più `sensor.kobo_tensione_batteria` se il dispositivo la
fornisce. Alimentato li aggiorna a ogni cambiamento (almeno ogni
minuto); a batteria solo ogni `report_on_battery`. Dopo un riavvio di Home
Assistant ricompaiono al primo invio.

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
