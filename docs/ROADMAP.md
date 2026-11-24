# Roadmap

Ziel: ein Agent, der in Viererpartien beliebige Commander-Decks pilotiert und aus sehr vielen simulierten Partien gelernt hat.

## Zuerst: trägt die Idee?

Bevor wir die Engine weiter ausbauen, klären wir, ob ein Modell überhaupt besser spielen lernt als der Heuristik-Bot. Die Kartenarbeit aus Etappe 1 ruht so lange.

**Messlatte.** `podbench arena -pool`: ein Kandidat gegen drei Heuristik-Bots an zufällig gezogenen Tischen aus 45 Precons. Jede Partie wird auf denselben Karten auch mit dem Heuristik-Bot auf dem Kandidatenplatz gespielt, gezählt wird der Unterschied. Erfolg heißt: deutlich über dessen 25 %.

**Training und Prüfung getrennt.** 40 Decks dienen dem Training, 5 nur der Prüfung. Die Aufteilung stammt aus der Engine. Der feste Tisch `adjusted` bleibt als schneller Kontrolltest.

**Was schon feststeht.**

- Die Engine ist schnell und regeltreu genug: rund 965 Partien pro Sekunde, 99,5 % der Partien verlaufen wie in der Referenz-Engine.
- Vorausdenken lohnt sich zu viert: Die Baumsuche der Engine gewinnt an gezogenen Tischen ohne gelerntes Netz 39 % (25 Simulationen) und 43 % (100 Simulationen).
- Upstream hat das Lernverfahren für Partien zu zweit mit 40-Karten-Decks bereits durchgespielt: Netze, die aus den Besuchszahlen der eigenen Suche lernen, schlugen dort die Suche ohne Netz mit rund 60 %. Nach einigen Generationen flachte der Zuwachs ab.

**Was die Engine dafür mitbringt.** Eine Codierung der Stellung aus Sicht eines Sitzes (nur was dieser Sitz sehen darf), eine Codierung der angebotenen Aktionen, die Baumsuche für mehr als zwei Sitze und das Laden eines extern trainierten Netzes im Format `.dzgw`. Das Trainingsprogramm selbst ist nicht veröffentlicht.

**Die nächsten Schritte.**

1. ✅ Messlatte bauen und mit Zufall, Heuristik-Bot und Baumsuche prüfen, am festen Tisch und an gezogenen Tischen.
2. ✅ Lernkette bauen: Stellungen aufzeichnen, in Python trainieren, Netz in Go laden und in der Baumsuche einsetzen.
3. ✅ Bewertungsnetz aus Partien des Heuristik-Bots: 33 % Siegquote in der Suche, besser als der Heuristik-Bot (25 %), schlechter als die handgebaute Bewertung (39 %). Mehr Partien und die Handbewertung als Eingabe ändern daran wenig.
4. ✅ Ursache gefunden: Das Netz kannte nur Stellungen, die der Heuristik-Bot selbst herbeiführt. Mit Trainingspartien, in denen 15 % der Züge zufällig sind, erreicht es 38,7 % und damit die handgebaute Bewertung (38,8 %). An den 5 Prüfdecks liegt es noch zurück (25,8 % gegenüber 29,1 %).
5. **Über die Handbewertung hinauskommen.** In dieser Reihenfolge:
    - auf Partien trainieren, in denen die Baumsuche mit dem aktuellen Netz selbst spielt, und das wiederholen. Jede Runde zeigt dem Netz die Stellungen, die sein eigenes Spiel erzeugt;
    - mehr unterschiedliche Abweichungen in den Trainingspartien (Anteil zufälliger Züge variieren);
    - Kartennamen ins Netz geben, damit es einzelne Karten unterscheiden lernt. Das sollte auch den Rückstand an unbekannten Decks zeigen oder schließen helfen;
    - prüfen, wie sich Netz und Handbewertung bei 100 Simulationen verhalten.
6. **Zugauswahl lernen.** Das Netz schlägt der Suche vor, welche Züge sie zuerst prüft. Bisher behandelt die Suche alle Kandidaten gleich.

**Woran der PoC entschieden wird.** Schlägt die Suche mit Netz die Suche mit Handbewertung bei gleicher Simulationszahl, an den Trainingsdecks und an den 5 Prüfdecks, dann trägt die Idee. Gelingt das nach den Versuchen aus Schritt 4 und 5 nicht, ist das ebenfalls ein Ergebnis.

**Bekannte Risiken.**

- Die Suche behandelt die drei Gegner bisher als gemeinsamen Gegenspieler. Eine eigene Bewertung je Sitz braucht ein dafür trainiertes Netz.
- Die Stellungscodierung ist auf das Netz von Upstream zugeschnitten.
- Fünf Prüfdecks sind wenig. Ein Zuwachs dort ist ein Hinweis auf Übertragbarkeit, kein Beweis.
- Tempo: An gezogenen Tischen schafft die Baumsuche mit 25 Simulationen 26 Partien pro Sekunde auf einem Laptop, mit 100 Simulationen 4. Für eine Trainingsrunde in der Größe, die Upstream nutzte (rund 30.000 Partien), sind das Stunden, keine Tage. Reicht das nicht, liegt der nächste Hebel im Kern selbst: Er berechnet die Manaquellen jeder bleibenden Karte bei jeder Anfrage neu.
- Regeltreue: Am Deckpool wichen in einer Stichprobe 2,6 % der Viererpartien von der Referenz-Engine ab, am festen Tisch 0,4 %. Für den PoC ist das hinnehmbar, für ein ernsthaftes Training nicht.
- Rund 4 % der Partien im Deckpool brechen an einer Grenze des Kerns ab. Ein Modell könnte lernen, solche Abbrüche herbeizuführen oder zu vermeiden. Die Quote gehört bei jeder Messung dazu.

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
