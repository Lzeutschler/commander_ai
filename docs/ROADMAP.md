# Roadmap

Ziel: ein Agent, der in Viererpartien beliebige Commander-Decks pilotiert und aus sehr vielen simulierten Partien gelernt hat.

## Zuerst: trägt die Idee?

Bevor wir die Engine weiter ausbauen, klären wir am laufenden Tisch `adjusted`, ob ein Modell dort überhaupt besser spielen lernt als der Heuristik-Bot. Die Kartenarbeit aus Etappe 1 ruht so lange.

**Messlatte.** `podbench arena`: ein Kandidat gegen drei Heuristik-Bots, jedes Deck von jedem Platz, verglichen mit dem Heuristik-Bot auf denselben Partien. Erfolg heißt: deutlich über dessen 26 %.

**Was schon feststeht.**

- Die Engine ist schnell und regeltreu genug: rund 965 Partien pro Sekunde, 99,5 % der Partien verlaufen wie in der Referenz-Engine.
- Vorausdenken lohnt sich zu viert: Die Baumsuche der Engine gewinnt ohne gelerntes Netz 33 % (25 Simulationen) und 38 % (100 Simulationen).
- Upstream hat das Lernverfahren für Partien zu zweit mit 40-Karten-Decks bereits durchgespielt: Netze, die aus den Besuchszahlen der eigenen Suche lernen, schlugen dort die Suche ohne Netz mit rund 60 %. Nach einigen Generationen flachte der Zuwachs ab.

**Was die Engine dafür mitbringt.** Eine Codierung der Stellung aus Sicht eines Sitzes (nur was dieser Sitz sehen darf), eine Codierung der angebotenen Aktionen, die Baumsuche für mehr als zwei Sitze und das Laden eines extern trainierten Netzes im Format `.dzgw`. Das Trainingsprogramm selbst ist nicht veröffentlicht.

**Die nächsten Schritte.**

1. ✅ Messlatte bauen und mit Zufall, Heuristik-Bot und Baumsuche prüfen.
2. **Trainingsdaten erzeugen.** Partien mit Baumsuche spielen und pro Entscheidung festhalten: codierte Stellung, angebotene Aktionen, Besuchszahlen der Suche, Ausgang der Partie. Zu klären ist, ob die vorhandene Aufzeichnung der Engine dafür reicht oder ob wir sie ergänzen.
3. **Ein kleines Netz trainieren** (außerhalb von Go, in Python) und als `.dzgw` zurückgeben. Erst die Bewertung der Stellung, dann die Auswahl der Aktion.
4. **Messen.** Baumsuche mit Netz gegen Baumsuche ohne Netz bei gleicher Simulationszahl, und Netz ohne Suche gegen den Heuristik-Bot. Wird die Suche mit Netz stärker, trägt die Idee. Bleibt sie gleich, wissen wir es nach wenigen Tagen.
5. **Wiederholen.** Mit dem besseren Netz neue Partien erzeugen und prüfen, ob eine zweite Runde noch einmal zulegt.

**Bekannte Risiken.** Die Suche behandelt die drei Gegner bisher als gemeinsamen Gegenspieler, eine eigene Bewertung je Sitz braucht ein dafür trainiertes Netz. Die Stellungscodierung ist auf das Netz von Upstream zugeschnitten. Und vier Decks sind wenig: Ein Netz kann die Decks auswendig lernen, statt allgemein besser zu spielen. Ob es überträgt, zeigt erst ein Deck, das im Training nicht vorkam.

## Arbeitsweise

- **Erst messen, dann ändern.** Jede Änderung an Regeln oder Geschwindigkeit wird an den eingefrorenen Tischen beurteilt: Welche Decks lädt die Engine zusätzlich, welche Seeds enden jetzt korrekt, was kostet es an Durchsatz? `make test` hält den jeweils gültigen Stand fest.
- **Korrektheit vor Durchsatz.** Der schnelle Kern schafft mit heuristischen Piloten rund 965 Partien und 290.000 Entscheidungen pro Sekunde auf einem Laptop. Sobald ein Modell die Entscheidungen trifft, bestimmt dessen Inferenz die Geschwindigkeit. Der Engpass ist deshalb zuerst die Kartenabdeckung, danach die Schnittstelle zum Modell.
- **Engine-Änderungen gehen upstream.** gorge wird sehr aktiv entwickelt (38 neue Commits in den 35 Minuten nach unserem Pin). Kartenlücken schließen wir in einem Fork und reichen sie als Pull Request ein. In gorge gilt: Eine Karte, die `dirac` spielt, muss sich dort genauso verhalten wie in der breiten Engine. Diese Gegenprobe übernehmen wir.
- **Den Pin regelmäßig anheben.** Vor der Arbeit an einer Lücke prüfen, ob Upstream sie schon geschlossen hat. Fork auf Upstream rebasen, `make setup baseline lockstep`, Diff lesen. Stand 24.11.2026, Upstream `9ee19e82`: alle Lücken und Abbrüche unverändert.
- **Selbst belegen, was wir spielen.** Upstream prüft Karten in Partien zu zweit. Unser Maßstab ist `make lockstep`: unsere eigenen Decks zu viert, beide Engines im Gleichschritt.

## Etappe 0 · Baseline ✅

Fester Tisch, reproduzierbarer Runner, Kartenbericht, Reproduktion einzelner Seeds. Astras Ausgangsmessung ist auf einem frischen Aufbau bestätigt: identische Sieger, Zugzahlen und Abbrüche, die Referenz-Engine liefert für alle 100 Originalpartien dieselben Ereignisketten.

## Etappe 1 · Der Originaltisch spielt im schnellen Kern

Abnahme: Alle vier Originaldecks laden mit unveränderten Prüfsummen. Ein veröffentlichter Lauf über 1.000 Seeds weist Abschlussquote, Abbruchursachen und Durchsatz aus. `make lockstep` weist für den Originaltisch aus, wie viele Partien Zug für Zug mit der Referenz-Engine übereinstimmen.

Der Fortschritt ist am Tisch `frontier` ablesbar: 23 ersetzte Kartenplätze am Ausgangsstand, 20 seit *War Room*, 0 am Ziel.

1. ✅ **War Room.** Lebenskosten in Höhe der Farbanzahl der Commander-Farbidentität, nur in Commander-Partien. Umgesetzt im Fork (Zweig `war-room`) mit Teststellungen im Kern, Gleichschritt-Tests zu zweit und zu viert und einem Gegentest. Offen: Pull Request bei Upstream.
2. **Regeltreue zu viert.** Der Gleichschritt-Lauf zeigt 14 bis 18 abweichende von 4.000 Viererpartien, auch ohne unsere Änderungen. Hauptursache ist die Reihenfolge gleichzeitiger Auslöser mehrerer Spieler, dazu einzelne Fälle rund um ausscheidende Spieler. Für ein Trainingsziel zu viert ist das die wichtigste Korrektheitsbaustelle, weil Upstream Karten nur in Partien zu zweit belegt.
3. **Abbrüche im Spiegeltest.** 58 von 62 entfallen auf `exilerider`. Seed 19 bricht schon nach 64 Entscheidungen in Zug 17 ab und eignet sich als Ausgangspunkt für die kleinste Stellung.
4. **Orb of Dragonkind und Herald's Horn.** Beide sind übersetzt, aber unbelegt. *Orb of Dragonkind* weicht im Gleichschritt ab und führt mit dem heuristischen Piloten zu Kernfehlern (`autopay found no plan`, meist wenn *Lathliss, Dragon Queen* ihre Fähigkeit mit dem Mana des Orbs bezahlen will). *Herald's Horn* fiel in 4.000 Partien nicht auf, braucht aber noch einen gezielten Test.
5. **Die übrigen 16 Kartenlücken, nach Mechanik gebündelt.** Zählausdrücke (*Endless Atlas*, *Padeem*, *Spinerock Knoll*, *Cut a Deal*), Filter und Parameter (*Haven of the Spirit Dragon*, *Cursed Mirror*, *Leyline Tyrant*, *Chandra's Ignition*, *Fateful Absence*), Entscheidungen, die der Kern noch nicht stellt (*Emeria Shepherd*, *The Elder Dragon War*, *Sephara*), Sonderfälle (*Hit the Mother Lode*, *Dragonhawk*, *Thundermane Dragon*, *Goddric*).
6. **Angriffskosten.** *Archangel of Tithes* und *Propaganda* zusammen mit Angriffspflichten und Manaquellen anderer Form.
7. **Kapazitätsabbrüche.** Über 10.000 Seeds am angepassten Tisch bleiben 7 Abbrüche (`board` 5, `prolif` 2). Prüfen, ob größere Kapazitäten sie ohne messbaren Durchsatzverlust beseitigen.

Jede Karte durchläuft dieselben Schritte wie *War Room*: Übersetzung im Generator, Verhalten im Kern, Teststellungen, ein Gleichschritt-Test mit eigenem kleinem Kartenpool samt Gegentest, danach `make lockstep` an unseren Tischen und der Eintrag in `decks/allow.txt`. In Streitfällen dient XMage als dritte Meinung, weil `dirac` und `gorge` denselben Kartencompiler teilen und eine falsche Lesart gemeinsam erben können.

Der formale Nachweis bei Upstream läuft über dessen eigene Automatik und über Partien zu zweit. Fähigkeiten, die es nur in Commander-Partien gibt, kann sie laut Upstream-Design derzeit noch nicht gutschreiben. Deshalb führen wir die Zulassungsliste selbst.

## Etappe 2 · Mehr Decks, Suche, Durchsatz

- **Deckbreite.** Den Kartenbericht auf weitere Precons und Community-Listen ausdehnen. Kennzahl: Anteil der Decks, die unverändert laden, und Anteil der Partien, die regulär enden.
- **Kosten einer Stellung.** Kopieren und Fortschalten einer Stellung mit vier vollen Boards und offenem Stapel messen. Das bestimmt, wie viel Baumsuche pro Entscheidung bezahlbar ist.
- **Profil des schnellen Kerns.** Erst wenn die Abdeckung steht. Das mitgelieferte Profil betrifft die breite Engine und sagt über `dirac` wenig.

## Etappe 3 · Lernumgebung

- **Beobachtung pro Sitz.** Nur was der Sitz wissen darf: eigene Hand, offene Zonen, Stapel, bekannte Karten. Fremde Hände und Bibliotheksreihenfolgen bleiben verborgen. Karten als Merkmale ihrer Regeln statt nur als Name, damit das Modell auf ungesehene Decks übertragen kann.
- **Aktionen.** Die Engine liefert zu jeder Entscheidung die legalen Aktionen. Das Modell bewertet diese Liste, statt aus einem festen Aktionsraum zu wählen.
- **Gebündelte Inferenz.** Viele Partien laufen parallel in Go, ihre offenen Entscheidungen gehen gesammelt an das Modell. Zu messen sind Partien, Modellabfragen und Suchaufwand pro Sekunde, jeweils getrennt.
- **Offene Entscheidung:** Training in Python mit Go-Simulatoren als Datenlieferanten oder Inferenz direkt im Go-Prozess. Das entscheiden die Messungen aus Etappe 2.

## Etappe 4 · Training und Bewertung

- Zuerst die Heuristik imitieren, danach Self-Play gegen ältere Modellstände und feste Bots.
- Die Suche mit Netzbewertung liefert auf dem Messrechner (Apple M3 Max) andere Prüfsummen als bei Upstream hinterlegt: Zwei Upstream-Tests schlagen dort auch am unveränderten Stand fehl. Vor dem Training klären, woran das liegt und ob Ergebnisse zwischen Rechnern bitgenau vergleichbar sein müssen.
- Bewertung mit rotierenden Sitzen, unveränderten Gegnern und zurückgehaltenen Decks. Am festen Tisch gewinnt *Keen Engineering* mit der Heuristik nur 28 von 1.000 Partien. Solche Schieflagen sagen etwas über den Piloten, erst in zweiter Linie über das Deck.
- Für vier Sitze steht der Nachweis aus: Der veröffentlichte Commander-Trainingslauf von Argentum P1 betrifft Partien zu zweit.
