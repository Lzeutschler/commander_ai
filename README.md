# commander_ai

Schnelle, regelgetreue Simulation von Magic-Commander-Partien zu viert, als Grundlage für das Training eines Agenten, der beliebige Decks pilotiert.

Das Projekt baut auf [gorge](https://github.com/adams-shaun/gorge) auf, einer Magic-Engine in Go. Deren schneller Kern `dirac` spielt die Partien, die breitere Engine `gorge` dient als Referenz. Dieses Repository liefert, was darüber liegt: feste Testtische, einen reproduzierbaren Benchmark und später die Lernumgebung und das Training.

## Worum es gerade geht

Die erste Frage ist, ob die Idee trägt: Lernt ein Modell, an einem Commander-Tisch besser zu spielen als der eingebaute Heuristik-Bot?

### Die Messlatte

`podbench arena` setzt den Kandidaten auf einen Sitz und drei Heuristik-Bots auf die anderen. Jede Partie wird zweimal mit denselben Decks, derselben Sitzordnung und demselben Seed gespielt: einmal mit dem Heuristik-Bot auf dem Kandidatenplatz, einmal mit dem Kandidaten. Gezählt wird der Unterschied. Deckstärke, Sitzplatz und Kartenglück sind in beiden Partien gleich und fallen heraus.

Mit `-pool` zieht jede Partie einen neuen Tisch aus 46 Commander-Precons, die die Engine mitbringt: ein Deck für den Kandidaten, drei andere für die Gegner, zufällige Sitzordnung. Die Decks sind in 41 Trainings- und 5 Prüfdecks geteilt. Ein Modell, das auch mit Prüfdecks gewinnt, die es nie gesehen hat, spielt wirklich besser und hat nicht nur Listen gelernt.

### Erste Ergebnisse

Je 1.600 Partien ([results/arena/](results/arena/)), Siegquote des Kandidaten mit 95-%-Intervall:

| Kandidat | gezogene Tische (46 Precons) | fester Tisch `adjusted` |
|---|---:|---:|
| Zufall | 7,4 % (6,2 bis 8,8) | 8,3 % (7,1 bis 9,8) |
| Heuristik-Bot (Kontrolle) | 23,7 % (21,7 bis 25,8) | 26,0 % (23,9 bis 28,2) |
| Baumsuche, 25 Simulationen | 40,2 % (37,8 bis 42,6) | 33,0 % (30,7 bis 35,3) |
| Baumsuche, 100 Simulationen | 43,8 % (41,3 bis 46,2) | 38,4 % (36,1 bis 40,8) |

Die Baumsuche bringt die Engine mit. Sie bewertet Stellungen bisher mit einer Heuristik und ohne gelerntes Netz. Das Ergebnis zeigt zweierlei: Vorausdenken zahlt sich am Vierertisch deutlich aus, und die Messlatte unterscheidet Spielstärke zuverlässig. Offen ist der eigentliche Lernschritt, siehe [docs/ROADMAP.md](docs/ROADMAP.md).

### Einschränkungen des Deckpools

- Die Engine hat in diesen Listen Karten, die ihr Kern nicht spielt, schon durch Standardländer ersetzt. Zusätzlich ersetzen wir 25 Karten ohne Nachweis (111 Kartenplätze) und lassen 6 Decks weg, deren Commander selbst unbelegt ist.
- Etwa jeder sechste gezogene Tisch enthält eine Kartenkombination, die der Kern ablehnt. Dann wird neu gezogen.
- Rund 5 % der Partien brechen an einer Grenze des Kerns ab (am festen Tisch 0,2 %). Sie zählen als Niederlage.
- An gezogenen Tischen ist die Baumsuche etwa fünfmal langsamer als am festen Tisch (8 statt 48 Partien pro Sekunde bei 25 Simulationen). Die Ursache ist noch nicht untersucht.

## Stand der Engine

Gemessen am 24. November 2026 auf einem Apple M3 Max (14 Kerne), vier heuristische Piloten (`hbot`), Seeds 1 bis 1000, ein erlaubter Mulligan. Ohne Modellinferenz und ohne Baumsuche.

| Tisch | Engine | Worker | Beendet / gestartet | Beendete Partien/s |
|---|---|---:|---:|---:|
| `original` (vier unveränderte Precons) | dirac | – | beim Laden abgelehnt | – |
| `original` | gorge | 1 | 100 / 100 | 7,8 |
| `frontier` (Originale, 20 Kartenplätze ersetzt) | dirac | 14 | 1.000 / 1.000 | 929 |
| `adjusted` (eingefroren, 23 Kartenplätze ersetzt) | dirac | 1 | 998 / 1.000 | 94 |
| `adjusted` | dirac | 14 | 998 / 1.000 | 966 |
| `mirror` (viermal unverändertes *Wretched Ranks*) | dirac | 14 | 938 / 1.000 | 2.264 |

Sieger, Zugzahlen und Abbrüche sind unabhängig von der Workerzahl identisch. Eine Partie umfasst im Mittel 42 Züge und rund 300 Entscheidungen.

### Regeltreue zu viert

`make lockstep` spielt unsere Tische auf beiden Engines gleichzeitig: `gorge` führt, `dirac` folgt, und nach jeder Entscheidung werden die angebotenen Aktionen und der gesamte Spielzustand verglichen. Je 1.000 Viererpartien pro Tisch, Pilot und Bezahlweise ([results/baseline/lockstep.tsv](results/baseline/lockstep.tsv)):

| Tisch | Partien | identisch | abweichend | Kapazitätsgrenze | Regellücke |
|---|---:|---:|---:|---:|---:|
| `adjusted` | 4.000 | 3.980 | 18 | 2 | 0 |
| `frontier` | 4.000 | 3.978 | 14 | 6 | 2 |

Rund 99,5 % der Partien verlaufen Zug für Zug wie in der Referenz-Engine. Die Abweichungen betreffen überwiegend die Reihenfolge gleichzeitiger Auslöser bei mehr als zwei Spielern. Upstream belegt Karten in Partien zu zweit, dort tritt das nicht auf.

### Was dem Originaltisch noch fehlt

- **18 Kartenplätze** in drei Decks. 16 Karten übersetzt der schnelle Kern noch nicht. *Orb of Dragonkind* und *Herald's Horn* sind übersetzt, aber unbelegt. Die Liste mit Begründung steht in [results/baseline/cards-original.json](results/baseline/cards-original.json).
- ***Orb of Dragonkind* spielt nachweislich falsch**: Mit der Karte steigen die Abweichungen im Gleichschritt von 3 bis 5 auf 13 bis 32 je 1.000 Partien, und mit dem heuristischen Piloten brechen 4 von 1.000 Partien mit einem Kernfehler ab.
- **Eine Interaktionssperre** rund um Angriffskosten (*Archangel of Tithes*, *Propaganda* neben *Carnelian Orb of Dragonkind*) kostet zwei weitere Plätze.
- **62 von 1.000 Spiegelpartien** brechen ab, 58 davon an derselben Stelle (`exilerider`).

*War Room* ist seit dem aktuellen Engine-Stand spielbar (drei Decks, [Issue #1](https://github.com/Lzeutschler/commander_ai/issues/1)). Der Plan für den Rest steht in [docs/ROADMAP.md](docs/ROADMAP.md).

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
bin/podbench run -table adjusted -n 1000                        # Durchsatz und Abschlussquote am eingefrorenen Tisch
bin/podbench run -table frontier -allow decks/allow.txt         # die Originaldecks, soweit die Engine sie heute spielt
bin/podbench run -table original                                # zeigt die Ablehnung beim Laden (Exit 3)
bin/podbench cards -table original -allow decks/allow.txt       # Kartenbericht für die vier Originaldecks
bin/podbench repro -table mirror -seed 19 -trace 20             # einen abgebrochenen Seed mit den letzten 20 Entscheidungen
bin/podbench arena -pool all -candidate search:25 -n 1600       # Messlatte an gezogenen Tischen aus 46 Precons
bin/podbench arena -candidate search:25 -n 1600                 # Messlatte am festen Tisch (schneller Kontrolltest)
make lockstep                                                   # beide Engines im Gleichschritt, 8.000 Partien (etwa 7 Minuten)
make reference                                                  # Originaltisch auf der breiten Engine, 100 Partien
```

## Tische und Decks

Die acht Listen unter [decks/](decks/) sind eingefroren, ihre Prüfsummen stehen in `decks/SHA256SUMS`. `decks/original/` enthält die vier Foundations-Commander-Precons unverändert, `decks/adjusted/` dieselben Decks mit 7, 3, 13 und 0 ersetzten Kartenplätzen. Die Sitzreihenfolge ist fest: *Calling All Angels*, *Keen Engineering*, *Reign of Dragons*, *Wretched Ranks*.

| Tisch | Listen | Zweck |
|---|---|---|
| `original` | die vier Originale | das Ziel: lädt, sobald keine Karte mehr fehlt |
| `frontier` | aus den Originalen abgeleitet | der Fortschritt: nur was die laufende Engine nicht spielt, wird durch Standardländer ersetzt. Der Bericht nennt jede Ersetzung mit Grund |
| `adjusted` | eingefroren | der Vergleichsmaßstab: bleibt unverändert, auch wenn die Engine mehr Karten spielt |
| `mirror` | viermal *Wretched Ranks* | Abbrüche eines einzelnen Decks |

Ohne Zulassungsliste ergibt `frontier` am aktuellen Stand genau die Listen von `adjusted` und dieselben 1.000 Partien.

### Zulassungsliste

Die Engine spielt eine Karte erst, wenn saubere Gleichschritt-Partien sie belegen. Dieser Nachweis entsteht upstream in Partien zu zweit und folgt einer Änderung mit Verzögerung. [decks/allow.txt](decks/allow.txt) nennt Karten, die wir vorher selbst belegt haben: mit einem gezielten Gleichschritt-Test im Engine-Fork und mit `make lockstep` an unseren Tischen. Ungeprüft zulassen geht nicht ohne Risiko, wie *Orb of Dragonkind* zeigt.

## Berichte

`podbench run -out <ordner>` schreibt zwei Dateien pro Tisch:

- `<tisch>.json` enthält alles, was die Partien bestimmt und wie sie endeten: Engine- und Kartenrevision, Deck-Prüfsummen, Pilot, Seeds, Ergebnis, jeden Abbruch mit Seed und Grund sowie eine Prüfsumme über alle Partien einschließlich Endstellung. Ein zweiter Checkout am selben Stand schreibt dieselben Bytes, unabhängig von Hardware und Workerzahl.
- `<tisch>.timing.json` enthält die hardwareabhängigen Werte: Rechner, Worker, Laufzeit, Durchsatz.

Die Referenz liegt unter [results/baseline/](results/baseline/). `make test` spielt sie nach und schlägt fehl, sobald sich ein Ergebnis ändert. Nach einer gewollten Änderung an Engine oder Decks schreibt `make baseline` sie neu, und der Diff zeigt, was sich am Tisch geändert hat.

## Engine und Lizenzen

Die Engine ist als Submodul unter `third_party/gorge` auf einen festen Commit gepinnt. Ihre Kartentabellen entstehen beim Aufbau im Checkout, deshalb lässt sie sich nicht als gewöhnliches Go-Modul einbinden.

Das Submodul zeigt auf unseren Fork [Lzeutschler/gorge](https://github.com/Lzeutschler/gorge), Zweig `commander-ai`. Er liegt zwei Commits über Upstream `9ee19e8250`: *War Room* im schnellen Kern (Zweig `war-room`, für Upstream bestimmt) und die namentliche Zulassung ungeprüfter Karten in `dirac/lab`.

| Bestandteil | Revision | Lizenz |
|---|---|---|
| Engine [Lzeutschler/gorge](https://github.com/Lzeutschler/gorge), Fork von [adams-shaun/gorge](https://github.com/adams-shaun/gorge) | `558c52da7b95` | Apache-2.0 |
| Kartenskripte [adams-shaun/forge](https://github.com/adams-shaun/forge) | `ff076d443284` | GPL-3.0 |
| Dieses Repository | – | Apache-2.0 |

Die Forge-Kartenskripte werden beim Aufbau in das von Git ignorierte Verzeichnis `third_party/gorge/.cards/` geladen. Sie sind reine Build-Eingabe und werden hier nie eingecheckt, auch nicht in Auszügen.
