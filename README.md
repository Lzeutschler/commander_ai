# commander_ai

Schnelle, regelgetreue Simulation von Magic-Commander-Partien zu viert, als Grundlage für das Training eines Agenten, der beliebige Decks pilotiert.

Das Projekt baut auf [gorge](https://github.com/adams-shaun/gorge) auf, einer Magic-Engine in Go. Deren schneller Kern `dirac` spielt die Partien, die breitere Engine `gorge` dient als Referenz. Dieses Repository liefert, was darüber liegt: feste Testtische, einen reproduzierbaren Benchmark und später die Lernumgebung und das Training.

## Worum es gerade geht

Die erste Frage ist, ob die Idee trägt: Lernt ein Modell, an einem Commander-Tisch besser zu spielen als der eingebaute Heuristik-Bot?

### Die Messlatte

`podbench arena` setzt den Kandidaten auf einen Sitz und drei Heuristik-Bots auf die anderen. Jede Partie wird zweimal mit denselben Decks, derselben Sitzordnung und demselben Seed gespielt: einmal mit dem Heuristik-Bot auf dem Kandidatenplatz, einmal mit dem Kandidaten. Gezählt wird der Unterschied. Deckstärke, Sitzplatz und Kartenglück sind in beiden Partien gleich und fallen heraus.

Mit `-pool` zieht jede Partie einen neuen Tisch aus 45 Commander-Precons, die die Engine mitbringt: ein Deck für den Kandidaten, drei andere für die Gegner, zufällige Sitzordnung. Die Decks sind in 40 Trainings- und 5 Prüfdecks geteilt. Ein Modell, das auch mit Prüfdecks gewinnt, die es nie gesehen hat, spielt wirklich besser und hat nicht nur Listen gelernt.

### Erste Ergebnisse

Je 1.600 Partien ([results/arena/](results/arena/)), Siegquote des Kandidaten mit 95-%-Intervall:

| Kandidat | gezogene Tische (45 Precons) | fester Tisch `adjusted` |
|---|---:|---:|
| Zufall | 6,3 % (5,2 bis 7,6) | 8,3 % (7,1 bis 9,8) |
| Heuristik-Bot (Kontrolle) | 24,8 % (22,7 bis 26,9) | 26,0 % (23,9 bis 28,2) |
| Baumsuche, 25 Simulationen | 38,8 % (36,5 bis 41,2) | 33,0 % (30,7 bis 35,3) |
| Baumsuche, 100 Simulationen | 43,4 % (41,0 bis 45,8) | 38,4 % (36,1 bis 40,8) |

Die Baumsuche bringt die Engine mit. Sie bewertet Stellungen hier mit einer Heuristik und ohne gelerntes Netz. Das Ergebnis zeigt zweierlei: Vorausdenken zahlt sich am Vierertisch deutlich aus, und die Messlatte unterscheidet Spielstärke zuverlässig. Der Lernschritt folgt im nächsten Abschnitt, der Plan steht in [docs/ROADMAP.md](docs/ROADMAP.md).

### Die Lernversuche

Die Lernkette steht von Anfang bis Ende: Partien spielen und Stellungen aufzeichnen (`podbench record`), in Python ein Netz trainieren ([train/train_value.py](train/train_value.py)), es in Go laden und in der Baumsuche einsetzen (`podbench arena -candidate net:<datei>:<simulationen>`). Go berechnet dabei dieselben Werte wie PyTorch, das prüft ein Test.

**Ergebnis:** Die Baumsuche mit einem gelernten Netz schlägt die Baumsuche mit der handgebauten Bewertung der Engine, an gezogenen Tischen mit 51,4 % gegenüber 43,4 % bei 100 Simulationen.

Das Netz lernt bisher nur, Stellungen zu bewerten: Wie wahrscheinlich gewinnt dieser Sitz? Es ersetzt in der Baumsuche die handgebaute Bewertung, alles andere bleibt gleich. Trainiert wird an den 40 Trainingsdecks. Je 1.600 Partien an gezogenen Tischen, 25 Simulationen, Siegquote mit 95-%-Intervall:

| Bewertung in der Baumsuche | Trainingspartien | Siegquote |
|---|---|---:|
| keine Suche (Heuristik-Bot) | – | 24,8 % |
| Netz v0 | 20.000 des Heuristik-Bots | 33,1 % (30,8 bis 35,4) |
| Netz v1 | 100.000 des Heuristik-Bots | 33,2 % (30,9 bis 35,5) |
| Netz v2 | wie v1, sieht zusätzlich die Handbewertung | 34,1 % (31,8 bis 36,4) |
| Netz v3 | 100.000 mit 15 % zufälligen Zügen | 38,7 % (36,3 bis 41,1) |
| handgebaute Bewertung der Engine | – | 38,8 % (36,5 bis 41,2) |
| Netz v4 | wie v3, dazu 60.000, in denen die Suche mit Netz v3 selbst spielt | 42,2 % (39,8 bis 44,6) |
| Netz v5 | wie v4, dazu 60.000, in denen die Suche mit Netz v4 selbst spielt | 45,3 % (42,9 bis 47,8) |

**Was den Unterschied machte.** Mehr Partien (v1) und die Handbewertung als Eingabe (v2) halfen kaum. Zwei Schritte brachten je einen Sprung:

- **v3: zufällige Züge in den Trainingspartien.** Die Piloten spielen in 15 % der Entscheidungen einen zufälligen Zug. Ein Netz, das nur Partien des Heuristik-Bots kennt, hat nie gesehen, was nach einem Zug passiert, den der Bot nicht machen würde. Genau solche Züge muss es in der Suche aber bewerten.
- **v4 und v5: Partien der eigenen Suche.** Auf einem Sitz spielt die Baumsuche mit dem jeweils letzten Netz, auf den anderen drei der Heuristik-Bot, wie in der Arena. Aufgezeichnet wird aus Sicht des suchenden Sitzes. Das Netz lernt so an den Stellungen, die sein eigenes Spiel erzeugt. Jede der beiden Runden brachte bei 25 Simulationen rund 3 Punkte.

Nicht geholfen hat, die Netzausgabe für die Suche zu spreizen, und auch nicht, v4 nur auf den 60.000 neuen Partien zu trainieren (39,4 %).

**Netz gegen Handbewertung.** `podbench compare` stellt zwei Kandidaten auf denselben 1.600 Partien gegenüber. Siegquote, in Klammern der Abstand zur Handbewertung in Punkten mit 95-%-Intervall:

| | 25 Simulationen | 100 Simulationen |
|---|---:|---:|
| **gezogene Tische, alle 45 Decks** | | |
| Handbewertung | 38,8 % | 43,4 % |
| Netz v3 | 38,7 % (−0,1; −2,8 bis +2,6) | 47,3 % (+3,9; +1,2 bis +6,7) |
| Netz v4 | 42,2 % (+3,4; +0,6 bis +6,1) | 50,0 % (+6,6; +3,9 bis +9,4) |
| Netz v5 | 45,3 % (+6,5; +3,7 bis +9,3) | 51,4 % (+8,0; +5,3 bis +10,7) |
| **die 5 Prüfdecks** | | |
| Handbewertung | 29,1 % | 33,3 % |
| Netz v3 | 25,8 % (−3,4; −5,9 bis −0,9) | 35,4 % (+2,1; −0,6 bis +4,8) |
| Netz v4 | 26,4 % (−2,8; −5,3 bis −0,2) | 35,8 % (+2,5; −0,2 bis +5,2) |
| Netz v5 | 29,3 % (+0,2; −2,3 bis +2,7) | 39,1 % (+5,8; +3,2 bis +8,5) |

Der Heuristik-Bot gewinnt auf denselben Partien 24,8 % an allen Decks und 13,9 % an den Prüfdecks.

**Einordnung.**

- **Die Suche mit gelerntem Netz schlägt die Suche mit handgebauter Bewertung** bei gleicher Simulationszahl, und der Abstand wächst mit jeder Selbstspiel-Runde. Mit 25 Simulationen ist Netz v5 stärker als die Handbewertung mit 100.
- **An den Prüfdecks**, die im Training nie vorkamen, liegt Netz v5 mit 100 Simulationen ebenfalls klar vorn. Mit 25 Simulationen ist es dort gleichauf.
- **Das Netz ist an bekannten Decks stärker als an unbekannten.** An den 40 Trainingsdecks allein beträgt der Vorsprung 7 bis 8 Punkte (47,4 % gegenüber 40,1 % bei 25 Simulationen, 53,0 % gegenüber 44,8 % bei 100). Es hat also zum Teil gelernt, was für diese Decks gilt, und nicht nur, was allgemein gilt.
- **Die Gegner sind schwach.** Gemessen wird gegen drei Heuristik-Bots. Wie das Netz gegen Gegner abschneidet, die selbst vorausdenken, ist offen.

Niemand hat dem Netz gesagt, was eine gute Stellung ist. Es lernt allein aus dem Ausgang der Partien.

**Wie das Netz spielt** untersucht [docs/ANALYSE.md](docs/ANALYSE.md). Der Vorsprung zeigt sich als seltenes Ausscheiden in langen Partien. Das Netz lässt Angriffe öfter aus, meidet das größte Brett und behält mehr Karten und Leben. Diesen Stil hatte schon v3, das nur gleichauf lag. Die Siege kamen erst, als das Selbstspiel das Tempo zurückbrachte. An unbekannten Decks fehlt der Vorsprung bei 25 Simulationen noch. Dort stehen auch die daraus abgeleiteten nächsten Schritte.

Das Netz sieht keine Kartennamen, nur zusammengefasste Eigenschaften der Karten je Zone und Spieler, so wie ein Sitz sie sehen darf. Dafür nutzt es die Stellungscodierung der Engine für Vierertische.

### Gegen stärkere Gegner

Bisher saßen drei Heuristik-Bots am Tisch. `podbench arena -opponents` setzt andere Gegner ein. Je 1.600 Partien an gezogenen Tischen, 25 Simulationen, Siegquote auf dem Kandidatenplatz. In Klammern steht der Abstand von v5 zur Handbewertung auf denselben Partien:

| drei Gegner | Handbewertung auf dem Platz | v5 auf dem Platz | v5 minus Handbewertung |
|---|---:|---:|---:|
| Heuristik-Bot | 38,8 % | 45,3 % | +6,5 [+3,7; +9,3] |
| Suche mit Handbewertung | 23,7 % | 30,6 % | +6,9 [+4,2; +9,6] |
| Suche mit Netz v5 | 22,5 % | 23,7 % | +1,2 [−1,4; +3,8] |

- **Gegen Gegner, die mit der Handbewertung vorausdenken, hält der Vorsprung voll.** Er ist so groß wie gegen Heuristik-Bots.
- **Gegen Kopien von sich selbst verschwindet er.** Hier ist v5 nicht messbar besser als die Handbewertung. Der Unterschied zu den beiden anderen Gegnerarten ist gesichert (5,7 ± 1,9 Punkte).
- **Die Spiegeltische liegen wie erwartet bei einem Viertel** der beendeten Partien (24,3 % und 24,9 %), die Messung ist also unverzerrt.
- **Netz-Gegner sind die stärksten.** Der Heuristik-Bot gewinnt gegen sie 12,9 %, gegen Gegner mit Handbewertung 17,6 %.
- **Abbrüche:** Mit Netz-Piloten am Tisch brechen mehr Partien ab (4,6 % gegenüber 2,6 % bei vier Handbewertungs-Suchen). Sie zählen für den Kandidaten als Niederlage.

**Einordnung.** Ein Teil dessen, was v5 gelernt hat, ist gut gegen Gegner, die so spielen wie der Heuristik-Bot. Das gilt auch für die Suche mit Handbewertung, die den Heuristik-Bot als Zuggenerator nutzt. Gegen einen Tisch voller Netze hilft dieses Wissen nicht. Kein Wunder: Trainiert wurde bisher nur gegen Heuristik-Bots. Das ist kein Rückschlag für den PoC, aber ein klarer Auftrag für die nächste Runde: Selbstspiel, in dem auch die Gegner Netze sind.

### Tempo an gezogenen Tischen

Auf einem Apple M3 Max mit 14 Workern:

| Kandidat | Partien pro Sekunde |
|---|---:|
| Heuristik-Bot (je Partie zusätzlich die Kontrollpartie) | 232 |
| Baumsuche, 25 Simulationen | 26 |
| Baumsuche, 100 Simulationen | 4,2 |

Die Engine bereitet jeden Tisch einmal vor und schaltet dabei allgemeinere, langsamere Rechenwege ein, sobald irgendeine Karte der vier Decks sie braucht. Zwei Maßnahmen haben das Tempo an gezogenen Tischen etwa verdreifacht:

- Ein Schalter ging fälschlich an fast jedem Tisch an, weil die Engine die Kartenliste des ganzen Pools prüfte statt der vier Decks. Das ist im Engine-Fork korrigiert und spielt nachweislich dieselben Partien.
- 19 Karten, die die übrigen teuren Rechenwege einschalten, sind durch Standardländer ersetzt ([decks/pool-skip.txt](decks/pool-skip.txt), 28 von 4.500 Kartenplätzen).

### Einschränkungen des Deckpools

- **Die Decks sind stark verdünnt.** Im Mittel sind 15 der 100 Karten eines Decks durch Standardländer ersetzt (zwischen 2 und 22). Rund 12 davon hat schon die Engine ersetzt, weil ihr Kern die Karte nicht spielt, 3 kommen von uns (25 Karten ohne Nachweis und die 19 langsamen Karten). Ein Deck hat dadurch im Mittel 51 Länder statt der gedruckten 36 bis 38. Die Partien sind also manareicher und ärmer an Zaubern als echtes Commander, und es fehlen bevorzugt die komplizierten Karten.
- 7 Decks fallen ganz weg, weil ihr Commander ohne Nachweis ist oder zu den langsamen Karten gehört.
- Etwa jeder achte gezogene Tisch enthält eine Kartenkombination, die der Kern ablehnt. Dann wird neu gezogen.
- Rund 4 % der Partien brechen an einer Grenze des Kerns ab (am festen Tisch 0,2 %). Sie zählen als Niederlage.
- Die Regeltreue ist geringer als am festen Tisch: In einer Stichprobe von 509 Viererpartien im Gleichschritt mit der Referenz-Engine wichen 13 ab (2,6 % gegenüber 0,4 %).

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
bin/podbench arena -pool all -candidate search:25 -n 1600       # Messlatte an gezogenen Tischen aus 45 Precons
bin/podbench arena -candidate search:25 -n 1600                 # Messlatte am festen Tisch (schneller Kontrolltest)
make venv                                                       # Python-Umgebung fürs Training
bin/podbench record -pilot ehbot:0.15 -n 100000 -every 160 -out data/v3   # Stellungen mit Ausgang aufzeichnen
.venv/bin/python train/train_value.py data/v3 models/v3.caiv              # Bewertungsnetz trainieren
bin/podbench arena -pool all -candidate net:models/v3.caiv:25 -out results/arena   # Baumsuche mit diesem Netz messen
# eine Selbstspiel-Runde: die Suche mit dem Netz auf einem Sitz, aufgezeichnet aus dessen Sicht
bin/podbench record -pilot explore:0.15:net:models/v3.caiv:25,hbot,hbot,hbot -keep 0 -n 60000 -out data/v4
.venv/bin/python train/train_value.py data/v3 data/v4 models/v4b.caiv --mark data/v4
bin/podbench arena -pool all -candidate net:models/v4b.caiv:25 -out results/arena
bin/podbench compare results/arena/arena-pool-all-net-models_v4b_caiv-25.json results/arena/arena-pool-all-hsearch-25.json
bin/podbench arena -pool all -candidate net:models/v5.caiv:25 -opponents hsearch:25 -out results/arena   # gegen drei suchende Gegner
# beobachten, wie Piloten spielen: dieselben Tische wie die Arena, Aufzeichnung je Partie (docs/ANALYSE.md)
bin/podbench study -pilots hsearch:25,net:models/v5.caiv:25 -probe net:models/v5.caiv:25 -value models/v5.caiv -n 400 -out data/study/s.jsonl
.venv/bin/python analysis/study.py data/study/s.jsonl --out results/local/s.json
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

Das Submodul zeigt auf unseren Fork [Lzeutschler/gorge](https://github.com/Lzeutschler/gorge), Zweig `commander-ai`. Er liegt drei Commits über Upstream `9ee19e8250`: *War Room* im schnellen Kern (Zweig `war-room`, bei Upstream eingereicht), die namentliche Zulassung ungeprüfter Karten in `dirac/lab` und die Tempokorrektur für Tische ohne Landtyp-Verleiher.

| Bestandteil | Revision | Lizenz |
|---|---|---|
| Engine [Lzeutschler/gorge](https://github.com/Lzeutschler/gorge), Fork von [adams-shaun/gorge](https://github.com/adams-shaun/gorge) | `8cbe24342763` | Apache-2.0 |
| Kartenskripte [adams-shaun/forge](https://github.com/adams-shaun/forge) | `ff076d443284` | GPL-3.0 |
| Dieses Repository | – | Apache-2.0 |

Die Forge-Kartenskripte werden beim Aufbau in das von Git ignorierte Verzeichnis `third_party/gorge/.cards/` geladen. Sie sind reine Build-Eingabe und werden hier nie eingecheckt, auch nicht in Auszügen.
