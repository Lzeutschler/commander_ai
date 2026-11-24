# commander_ai

Schnelle, regelgetreue Simulation von Magic-Commander-Partien zu viert, als Grundlage für das Training eines Agenten, der beliebige Decks pilotiert.

Das Projekt baut auf [gorge](https://github.com/adams-shaun/gorge) auf, einer Magic-Engine in Go. Deren schneller Kern `dirac` spielt die Partien, die breitere Engine `gorge` dient als Referenz. Dieses Repository liefert, was darüber liegt: feste Testtische, einen reproduzierbaren Benchmark und später die Lernumgebung und das Training.

## Stand

Gemessen am 24. November 2026 auf einem Apple M3 Max (14 Kerne), vier heuristische Piloten (`hbot`), Seeds 1 bis 1000, ein erlaubter Mulligan. Ohne Modellinferenz und ohne Baumsuche.

| Tisch | Engine | Worker | Beendet / gestartet | Beendete Partien/s |
|---|---|---:|---:|---:|
| `original` (vier unveränderte Precons) | dirac | – | beim Laden abgelehnt | – |
| `original` | gorge | 1 | 100 / 100 | 7,8 |
| `adjusted` (23 Kartenplätze durch Standardländer ersetzt) | dirac | 1 | 998 / 1.000 | 94 |
| `adjusted` | dirac | 8 | 998 / 1.000 | 679 |
| `adjusted` | dirac | 14 | 998 / 1.000 | 965 |
| `mirror` (viermal unverändertes *Wretched Ranks*) | dirac | 14 | 938 / 1.000 | 2.251 |

Sieger, Zugzahlen und Abbrüche sind unabhängig von der Workerzahl identisch. Eine Partie des angepassten Tischs umfasst im Mittel 42 Züge und rund 300 Entscheidungen.

Was dem Originaltisch im schnellen Kern noch fehlt:

- **21 Kartenplätze** (19 verschiedene Karten) werden nicht gespielt, *War Room* allein in drei Decks. Die vollständige Liste mit Begründung steht in [results/baseline/cards-original.json](results/baseline/cards-original.json).
- **Eine Interaktionssperre** rund um Angriffskosten (*Archangel of Tithes*, *Propaganda* neben *Carnelian Orb of Dragonkind*).
- **62 von 1.000 Spiegelpartien** brechen ab, 58 davon an derselben Stelle (`exilerider`).

Der Plan dazu steht in [docs/ROADMAP.md](docs/ROADMAP.md).

## Schnellstart

Voraussetzungen: Git, Go ≥ 1.25.8, GNU Make, Bash und Netzwerkzugriff für den ersten Aufbau.

```sh
git clone --recurse-submodules https://github.com/Lzeutschler/commander_ai.git
cd commander_ai
make setup   # Engine am gepinnten Stand, Kartenskripte, Kartentabellen, bin/podbench (etwa 1 Minute)
make test    # die eingefrorenen Decklisten und die Baseline müssen sich bitgenau reproduzieren
```

Danach:

```sh
bin/podbench run -table adjusted -n 1000              # Durchsatz und Abschlussquote
bin/podbench run -table mirror -n 1000 -workers 1     # Spiegeltest mit einem Worker
bin/podbench run -table original                      # zeigt die Ablehnung beim Laden (Exit 3)
bin/podbench cards -table original                    # Kartenbericht für die vier Originaldecks
bin/podbench repro -table mirror -seed 19 -trace 20   # einen abgebrochenen Seed mit den letzten 20 Entscheidungen
make reference                                        # Originaltisch auf der breiten Engine, 100 Partien
```

## Tische und Decks

Die acht Listen unter [decks/](decks/) sind eingefroren, ihre Prüfsummen stehen in `decks/SHA256SUMS`. `decks/original/` enthält die vier Foundations-Commander-Precons unverändert, `decks/adjusted/` dieselben Decks mit 7, 3, 13 und 0 ersetzten Kartenplätzen. Die Sitzreihenfolge ist fest: *Calling All Angels*, *Keen Engineering*, *Reign of Dragons*, *Wretched Ranks*.

Die angepassten Listen bleiben auch dann unverändert, wenn die Engine später mehr Karten spielt. So bleibt jede Vorher-nachher-Messung am selben Tisch vergleichbar.

## Berichte

`podbench run -out <ordner>` schreibt zwei Dateien pro Tisch:

- `<tisch>.json` enthält alles, was die Partien bestimmt und wie sie endeten: Engine- und Kartenrevision, Deck-Prüfsummen, Pilot, Seeds, Ergebnis, jeden Abbruch mit Seed und Grund sowie eine Prüfsumme über alle Partien einschließlich Endstellung. Ein zweiter Checkout am selben Stand schreibt dieselben Bytes, unabhängig von Hardware und Workerzahl.
- `<tisch>.timing.json` enthält die hardwareabhängigen Werte: Rechner, Worker, Laufzeit, Durchsatz.

Die Referenz liegt unter [results/baseline/](results/baseline/). `make test` spielt sie nach und schlägt fehl, sobald sich ein Ergebnis ändert. Nach einer gewollten Änderung an Engine oder Decks schreibt `make baseline` sie neu, und der Diff zeigt, was sich am Tisch geändert hat.

## Engine und Lizenzen

Die Engine ist als Submodul unter `third_party/gorge` auf einen festen Commit gepinnt. Ihre Kartentabellen entstehen beim Aufbau im Checkout, deshalb lässt sie sich nicht als gewöhnliches Go-Modul einbinden.

| Bestandteil | Revision | Lizenz |
|---|---|---|
| Engine [adams-shaun/gorge](https://github.com/adams-shaun/gorge) | `16ec2936189a` | Apache-2.0 |
| Kartenskripte [adams-shaun/forge](https://github.com/adams-shaun/forge) | `ff076d443284` | GPL-3.0 |
| Dieses Repository | – | Apache-2.0 |

Die Forge-Kartenskripte werden beim Aufbau in das von Git ignorierte Verzeichnis `third_party/gorge/.cards/` geladen. Sie sind reine Build-Eingabe und werden hier nie eingecheckt, auch nicht in Auszügen.
