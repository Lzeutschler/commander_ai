# Wie das Netz spielt

Stand 25.11.2026, Netz v5 nach zwei Selbstspiel-Runden. Der PoC ist erreicht: Die Baumsuche mit gelerntem Netz schlägt die Baumsuche mit der handgebauten Bewertung der Engine (siehe [README](../README.md)). Dieses Dokument untersucht, *wie* das Netz spielt, was es bewertet und was in den Runden passiert ist. Daraus leitet es die nächsten Schritte ab.

## Kurzfassung

- **Der Vorsprung zeigt sich als seltenes Ausscheiden.** v5 scheidet in 51 % der Partien aus, die Suche mit Handbewertung (`hsearch`) in 59 %. Bei 25 Simulationen fallen drei Viertel der zusätzlichen Siege in lange Partien (mehr als 12 eigene Züge).
- **Sein Stil:**
  - Es lässt Angriffe öfter ganz aus, greift dann aber mit mehr Kreaturen an.
  - Es meidet den Gegner mit dem größten Brett.
  - Es behält etwas mehr Karten und Leben und beendet lange Partien öfter mit Kommandeurschaden.
  - Es spielt nicht weniger Karten, nur in anderer Mischung: weniger Fähigkeiten, die Spielsteine erzeugen, mehr Ausrüsten und Kartenfluss.
- **Der Stil allein erklärt den Vorsprung nicht.** v3 hatte die vorsichtige Spielweise schon und war nur gleich stark wie `hsearch`. Mit dem Selbstspiel kam das Tempo dazurück, und damit kamen die Siege. An den 5 Prüfdecks spielt v5 im selben Stil, überlebt dort bei 25 Simulationen aber nicht länger.
- **An unbekannten Decks fehlt der Vorsprung bei 25 Simulationen** (29,3 % gegenüber 29,1 %). Erst bei 100 Simulationen ist er da (39,1 % gegenüber 33,3 %). Die Übertragung auf neue Decks ist die gemessene Schwachstelle.
- **Als Bewertung ist das Netz der Handbewertung weit überlegen.** Es sagt Sieger viel besser voraus (AUC 0,83 gegenüber 0,74) und erkennt sie früher. Die Handbewertung zählt jedes Objekt auf dem Brett hoch, auch Schatz- und Hinweis-Spielsteine, und gewichtet ausscheidende Gegner zu schwach. Bessere Vorhersage heißt aber nicht automatisch stärkeres Spiel: v3 und v5 sagen fast gleich gut voraus und spielen sehr verschieden stark.
- **Seine Einschätzungen sind verschoben:** Sie passen zu einem Piloten mit 30 bis 35 % Siegquote. In den eigenen Partien unterschätzt sich das Netz deshalb.
- **Auffällige Fehler:**
  - Mit einigen wiederholbaren Schadenseffekten (Aether Grid, Zacama, Ion Storm) zielt es auf eigene bleibende Karten.
  - Die Mulligan- und die Landwahl liegen bei allen Piloten beim Heuristik-Bot, das Netz entscheidet sie nie.
- **Gefunden und behoben:** ein Absturz in der Merkmalscodierung der Engine. Er verursachte 14 der 15 „failed"-Partien der Netz-Piloten. Pull-Request bei Upstream: [adams-shaun/gorge#4855](https://github.com/adams-shaun/gorge/pull/4855).
- **Die wichtigsten offenen Fragen:**
  - Hält der Vorsprung gegen stärkere Gegner als drei Heuristik-Bots?
  - Was genau trägt ihn? Das lässt sich nur mit gezielten Gegenproben klären.

## Wie gemessen wurde

**Partien beobachten.** `podbench study` spielt dieselben 1.600 gezogenen Tische wie die Arena, nacheinander mit fünf Piloten auf dem Kandidatenplatz: Heuristik-Bot, `hsearch` und die Suche mit den Netzen v3, v4 und v5, je 25 Simulationen. Ein zweiter Lauf vergleicht `hsearch` und v5 mit 100 Simulationen an 800 Tischen.

- **Übereinstimmung mit der Arena:** Für die vier Such-Piloten stimmen alle 1.600 Ausgänge Partie für Partie mit der Arena überein. Für den Heuristik-Bot fehlt ein Arena-Bericht je Partie, die Gesamtzahl der Siege (396) stimmt.
- **Was aufgezeichnet wird**, für den Kandidatenplatz:
  - was er spielt (Länder, Zauber nach Art, Fähigkeiten, Angriffe und auf wen, Blocks);
  - zu Beginn jedes Zuges der Tisch (Leben, Länder, Hand, Kreaturen, Stärke, Kommandeurschaden);
  - wann und woran jeder Sitz ausscheidet;
  - die Einschätzung von Netz v5 und Handbewertung für den Kandidaten.
- **Deckzusammensetzung:** Etwa 91 % der Tische haben ein Trainingsdeck auf dem Kandidatenplatz. Wo es darauf ankommt, sind die Prüfdecks getrennt ausgewiesen.

**Entscheidungen vergleichen.** An jeder dritten Entscheidung von v5 mit mehr als einer Option fragt das Werkzeug auf einer Kopie der Partie, was die anderen Piloten spielen würden. Das sind 55.706 Entscheidungen. Diese Proben haben Grenzen:

- **Die Suche ist deterministisch.** Eine frische Kopie von v5 antwortet an Prioritätsentscheidungen immer gleich. Sie zeigt also, dass eine Wahl feststeht, nicht dass sie robust ist. Fast gleichwertige Optionen, zwischen denen die Reihenfolge entscheidet, sehen genauso fest aus.
- **Nur Stellungen, die v5 erreicht hat**, und jeweils nur ein Schritt: Was die anderen danach getan hätten, bleibt offen.
- **Pläne über mehrere Schritte:** Die Suche plant z. B. alle Angreifer auf einmal und spielt den Plan in den folgenden Entscheidungen ab, eine frische Kopie fragt dagegen jedes Mal neu. Bei Angriff und Block liegt die Selbstübereinstimmung deshalb nur bei 85 %.

**Netze prüfen.** Für die Netzanalyse wurden drei frische Datensätze mit neuen Seeds aufgezeichnet, auf denen kein Netz trainiert hat:

- 4.000 Partien des Heuristik-Bots an den Trainingsdecks;
- 3.000 Partien mit v5 auf einem Sitz an den Trainingsdecks;
- 3.000 Partien mit v5 auf einem Sitz an den Prüfdecks. Anders als in der Arena stammen dabei alle vier Decks aus den 5 Prüfdecks, auch die der Gegner. Es gibt also nur 5 Deckkombinationen, und v5 gewinnt dort 54 %.

**Befunde prüfen.** Fünf Analyse-Agenten haben die Daten aus je einem Blickwinkel untersucht: Prioritätsentscheidungen, Kampf, Zielwahl, Spielverlauf und Bewertung. Jeden ihrer 35 Befunde hat ein zweiter Agent mit eigenem Code nachgerechnet und zu widerlegen versucht. 6 Befunde hielten unverändert, 29 mit Korrekturen. Ein dritter Durchgang hat dieses Dokument Zahl für Zahl gegen die Daten geprüft. Hier steht nur, was beide Prüfungen bestanden hat, mit den korrigierten Zahlen. Die Befunde mit Methode und Gegenprüfung liegen in [results/analysis/findings.json](../results/analysis/findings.json).

Unterschiede sind gepaart gemessen: gleicher Tisch, gleiche Decks, gleiche Startkarten. „±" ist ein Standardfehler, Intervalle in eckigen Klammern sind 95 %.

## Wie das Netz spielt

### Woran sich der Vorsprung zeigt

| je Partie (1.600 Tische, 25 Simulationen) | hsearch | v5 |
|---|---:|---:|
| Siegquote | 38,8 % | 45,3 % |
| ausgeschieden | 58,6 % | 51,3 % |
| Sieg innerhalb von 12 eigenen Zügen | 24,5 % | 25,9 % |
| Sieg nach mehr als 12 eigenen Zügen | 14,3 % | 19,4 % |
| Gegner, die auf dem eigenen Zug ausscheiden | 1,04 | 1,14 |
| Siege, deren letzter Schlag Kommandeurschaden ist | 5,8 % | 10,1 % |

- **Mehr Siege heißt hier fast zwangsläufig weniger Ausscheiden**, denn die Abbruchquote ist bei beiden gleich (+6,5 = 7,3 − 0,8 Punkte). Aussagekräftig ist, *wann* die zusätzlichen Siege fallen.
- **Bei 25 Simulationen:** Schnelle Siege sind kaum häufiger (+1,4 ± 1,3 Punkte), lange deutlich (+5,1 ± 1,2). Nach dem 12. eigenen Zug lebt v5 noch in 32,7 % der Partien, `hsearch` in 26,0 %. Von dort gewinnen beide ähnlich oft (59 % gegenüber 55 %).
- **Bei 100 Simulationen** (800 Tische):
  - Schnelle Siege sind häufiger (35,5 % gegenüber 31,4 %, +4,1 ± 1,9, knapp gesichert).
  - Bei langen Siegen ist der Unterschied unsicher (16,1 % gegenüber 13,5 %, +2,6 ± 1,5).
  - Ausgeschieden ist v5 in 45,5 %, `hsearch` in 52,7 %.
- **Kommandeurschaden:** Der zusätzliche Anteil kommt aus Siegen ab dem 10. eigenen Zug (`hsearch` 3,9 bis 6,6 %, v5 9,4 bis 12,3 %). Nur in sehr schnellen Siegen (höchstens 9 eigene Züge) nutzt `hsearch` ihn etwas öfter (17,3 % gegenüber 13,4 %, kleine Fallzahl).
- **Unverändert:**
  - wie Partien abgeschlossen werden (97 bis 98 % der Siegschläge fallen auf dem eigenen Zug);
  - wer den Kandidaten ausschaltet;
  - die Abbruchquote;
  - Länder und Handgröße der Gegner.

### Kampf: seltener, breiter, weg vom größten Brett

- **Ganze Angriffe auslassen.** Das ist das deutlichste Merkmal.
  - Bei der ersten Angriffsentscheidung eines Zuges greift v5 etwa 1,8-mal so oft gar nicht an wie `hsearch`. Solange noch drei Gegner leben, ist es 2,3-mal so oft.
  - Betroffen sind vor allem frühe Züge mit kleinem oder unterlegenem Brett. Im Zweikampf am Ende verschwindet der Unterschied.
  - Insgesamt greift v5 in 42 % seiner Züge an, `hsearch` in 52 %.
- **Wenn es angreift, dann breiter.** v5 greift mit 86,8 % der möglichen Angreifer an, `hsearch` mit 83,0 %. v3 und v4 tun das genauso, es hat sich in den Runden nicht verändert. Pro Entscheidung hört eine frische v5-Kopie sogar öfter mit dem Angreifen auf als `hsearch` (20,5 % gegenüber 17,5 %). Der breitere Angriff entsteht also vor allem dadurch, dass v5 einen einmal gefassten Plan zu Ende spielt.
- **Weg vom größten Brett.**
  - Pro eigenem Zug fügt v5 dem Gegner mit dem größten Brett in 15,3 % Schaden zu, `hsearch` in 28,3 %. Den Gegner mit dem kleinsten Brett trifft v5 in 27,9 % der Züge, `hsearch` in 32,9 %.
  - Betrachtet man nur Züge mit drei lebenden Gegnern und Schaden, ist das Hauptziel von v5 in 23,6 % das größte Brett (`hsearch` 36,4 %) und in 50,7 % das kleinste (43,0 %).
  - Auffällig ist vor allem `hsearch`, das das größte Brett viel öfter angreift als der Heuristik-Bot. Gut zwei Drittel der Verschiebung hatte schon v3, v4 und v5 setzen sie leicht fort.
- **Nach Lebenspunkten wählt es nicht anders.** Der scheinbare Unterschied in den Rohzahlen entsteht durch Gleichstände und durch weniger lebende Gegner.
- **Weniger Lebensverlust.**
  - v5 verliert pro gegnerischem Zug 0,96 Leben, `hsearch` 1,02, obwohl die Gegner mehr Stärke auf dem Brett haben.
  - Etwa drei Viertel davon erklärt, dass v5 öfter Züge ohne Schaden an Gegnern hat (66 % gegenüber 57 %). Nach solchen Zügen verliert jeder Pilot etwa halb so viel Leben.
  - Über ganze Partien blockt v5 je Blockschritt öfter (0,73 gegenüber 0,67). An denselben Stellungen blockt eine frische Kopie aber etwa so oft wie `hsearch`.
  - Dass ungetappte Kreaturen den Heuristik-Bot abschrecken, ist eine Vermutung, geprüft ist sie nicht.
- **Kosten:**
  - Solange vier Spieler leben, schaltet v5 seltener einen Gegner aus, selbst wenn es genug Stärke dafür hätte. Steht ein Gegner in Reichweite, fügt v5 ihm in 44,5 % der Züge Schaden zu, `hsearch` in 59,0 %.
  - Die Gegner bauen gegen v5 größere Bretter auf: im 8. und 9. eigenen Zug 0,9 bis 1,7 mehr Stärke beim stärksten Gegner.
  - Gegen den Heuristik-Bot zahlt sich die Zurückhaltung aus. Gegen Gegner, die selbst suchen, könnte sie sich rächen.

### Karten und Fähigkeiten

- **Es spielt nicht weniger Karten.** Pro eigenem Zug wirkt v5 1,023 Zauber, `hsearch` 1,018 (+0,006 ± 0,007).
  - Der Eindruck, v5 halte Karten zurück, entsteht aus einer Kennzahl: v5 passt insgesamt seltener, deshalb ist der Anteil der Pässe, bei denen ein Zauber möglich war, höher.
  - Passiv war v3: 0,83 Pässe trotz möglichem Zauber pro Zug gegenüber 0,57 bei `hsearch`, also knapp die Hälfte mehr.
- **Eine andere Mischung.**
  - Etwas weniger nicht-kreaturhafte bleibende Karten (−0,016 ± 0,003 pro Zug), etwas mehr Spontanzauber und Hexereien (+0,012 ± 0,003).
  - Wählen beide Suchen verschiedene Zauber, nimmt v5 öfter den billigeren, Nicht-Kreatur- oder Rampenzauber. Der Kreaturenanteil liegt dann bei 54 % gegenüber 77 %. Teilweise liegt das an `hsearch`, das in diesen Fällen zu 55 % dieselbe Karte wählt wie der Heuristik-Bot (v5 zu 25 %), also seinem Vorschlag folgt.
- **Mehr Kommandeur.** v5 wirkt seinen Kommandeur öfter aus der Kommandozone (+0,018 ± 0,002 pro Zug), meist nachdem er dorthin zurückgekehrt ist. Das kam mit dem Selbstspiel (v4 und v5).
- **Fähigkeiten: weniger Spielsteine, mehr Ausrüsten und Kartenfluss.** Etwa die Hälfte der Abweichungen bei Fähigkeiten ist symmetrisch: dieselben Karten auf beiden Seiten, nahezu gleichwertige Optionen. Der Rest zeigt ein Muster:
  - v5 erzeugt selten Spielsteine, wo `hsearch` es tut (nur v5: 49, nur `hsearch`: 387). Ebenso Populate, Inkubatoren, Hellsicht und Zählmarken für alle.
  - Dafür rüstet v5 öfter aus, opfert öfter bleibende Karten für Karten und nutzt öfter Ziehfähigkeiten von Planeswalkern.
  - Das passt zur Bewertungsanalyse unten: Die Handbewertung belohnt jedes zusätzliche Objekt auf dem Brett, das Netz nicht.
- **Spät und im Überfluss lässt es Kleinigkeiten liegen.**
  - Je später die Partie, je mehr Mana übrig, je weniger Optionen und je höher die eigene Einschätzung, desto öfter lehnt v5 einen Zauber ab, den `hsearch` wirkt: vor Zug 20 in 13,7 %, ab Zug 36 in 30,5 %, bei nur 2 Optionen in 41 %.
  - Häufigstes Beispiel ist Sol Ring mit 5 oder mehr Ländern. Das wirkt wie Gleichgültigkeit, nicht wie ein Plan.
  - Einen großen Preis hat es nicht: Ab einer Einschätzung von 0,9 gewinnen beide 96 % dieser Partien.

### Zielwahl

- **Schaden auf eigene Karten.**
  - In einer Stichprobe von 300 Partien, ausgewählt nach vielen Zielentscheidungen, wählt v5 bei eindeutig schädlichen Zielen in 20,7 % eigene bleibende Karten oder sich selbst. `hsearch` tut das in 7,7 %, der Heuristik-Bot in 2,8 %.
  - Das ballt sich: Aether Grid, Zacama und Ion Storm liefern fast die Hälfte dieser Fälle, aus etwa 8 Partien. Ohne diese drei Quellen sind es 13,9 % gegenüber 9,2 %.
  - Bei echten Entfernungszaubern („zerstöre", „schicke ins Exil") gibt es kaum einen Unterschied.
  - Wahrscheinliche Ursache: Bei Effekten, die das Netz kaum unterscheidet, entscheidet die Reihenfolge der Optionen.
  - Was es an Siegen kostet, ist nicht gemessen.
- **Weniger auf den Anführer.** Ein Gegner als Ziel ist bei v5 seltener der mit der meisten Stärke (51,6 % gegenüber 57,2 %) und seltener der mit dem meisten Material (48,3 % gegenüber 53,9 %). Nach Lebenspunkten gibt es keinen Unterschied.
- **Ausrüsten und Verstärken:**
  - Beides gibt v5 eher kleineren eigenen Kreaturen.
  - v5 rüstet oft dieselbe Kreatur neu aus. Diese Schleife stammt aus der Standardstrategie des Heuristik-Bots, der sie in 52 % dieser Entscheidungen wiederholt. v5 behält mehr davon als `hsearch` (34 %).
  - Ob das Tempo kostet, ist offen.
- **Kein Artefakt entschiedener Stellungen:** Die geringe Übereinstimmung bei Zielen (47 %) gilt über den ganzen Wertebereich und auf beiden Zügen. Mit mehr Optionen sinkt sie, von 61 % bei 2 Optionen auf 39 % ab 9 Optionen.
- **„Wähle"-Entscheidungen** unterscheiden sich kaum, meist sind es Kostenentscheidungen: was getappt, geopfert oder gesucht wird.

### Was gleich bleibt, und warum

- **Länder:** Jedes geprüfte Land war bei allen Piloten dasselbe. Das ist bauartbedingt: Länder spielt in jeder Suche der Heuristik-Bot, die Landsuche der Engine ist abgeschaltet.
- **Mulligan:** In 8.000 Kandidatenpartien hat kein Pilot einen Mulligan genommen. Die Entscheidung beantwortet bei allen Piloten der Heuristik-Bot, und der behält immer. Keine Suche bewertet sie.
- **Spontanzauber:** v5 hebt sie nicht für gegnerische Züge auf. Auf gegnerischen Zügen ist es sogar etwas passiver als `hsearch`.

### An den Prüfdecks

| Kandidatendeck aus den Prüfdecks (148 Tische, 25 Simulationen) | hsearch | v3 | v4 | v5 |
|---|---:|---:|---:|---:|
| Siegquote | 26,4 % | 25,7 % | 23,6 % | 25,0 % |
| ausgeschieden | 71,6 % | 71,6 % | 75,0 % | 71,6 % |
| Anteil eigener Züge mit Angriff | 52,5 % | 39,4 % | 38,6 % | 38,0 % |
| angreifende von möglichen Angreifern | 85,2 % | 85,7 % | 86,9 % | 87,1 % |

- **Stil gleich, Vorteil weg.** An den Prüfdecks spielt v5 im selben Stil wie an den Trainingsdecks: Es greift seltener und breiter an. Den Überlebensvorteil hat es dort bei 25 Simulationen aber nicht.
- **Die Arena bestätigt das** mit 1.600 Partien: 29,3 % gegenüber 29,1 % bei 25 Simulationen, 39,1 % gegenüber 33,3 % bei 100 Simulationen.
- **Was folgt:** Der vorsichtige Stil ist nicht das, was den Vorsprung trägt. Er ist an den Prüfdecks da und hilft dort nicht. Fehlen wird dort vermutlich, wie gut das Netz die einzelnen Karten einschätzt (siehe den nächsten Abschnitt).
- **Abhängigkeit vom Deck auch unter den Trainingsdecks:** Der Vorsprung von v5 schwankt mehr als zufällig (Permutationstest p = 0,003) und wiederholt sich teilweise über die Netzversionen. Mit der Deckstärke hängt er nicht zusammen. Kreaturenlastige Decks gewinnen am wenigsten, Blood Rites verliert unter jedem Netz.

## Was das Netz bewertet

### Es sagt viel besser voraus als die Handbewertung

Auf frischen Daten, Fläche unter der ROC-Kurve (AUC: Wie oft wird eine gewonnene Stellung höher bewertet als eine verlorene?):

| Datensatz | Handbewertung | v2 | v3 | v4 | v5 |
|---|---:|---:|---:|---:|---:|
| Heuristik-Bot, Trainingsdecks | 0,732 | 0,829 | 0,828 | 0,823 | 0,825 |
| v5 sucht, Trainingsdecks | 0,742 | 0,823 | 0,826 | 0,830 | 0,834 |
| v5 sucht, nur Prüfdecks am Tisch | 0,715 | 0,749 | 0,752 | 0,760 | 0,759 |

- **Die Netze unterscheiden sich als Vorhersager kaum**, obwohl v5 viel stärker spielt als v3. An den Prüfdecks sagt das Netz besser voraus als die Handbewertung und spielt bei 25 Simulationen trotzdem nicht stärker. Vorhersagegüte taugt also nicht, um Netze auszuwählen, das entscheidet nur die Arena. Vermutlich liegt es daran, wie die Suche die Folgestellungen der einzelnen Optionen gegeneinander abwägt. Prüfen lässt sich das erst mit Werten je Option an der Wurzel der Suche.
- **In jeder Spielphase vorn:** Bei Zug 17 bis 28 erreicht das Netz an den Trainingsdecks 0,79 bis 0,81 (Handbewertung 0,70), an den Prüfdecks 0,69 (0,63).
- **Die Handbewertung fügt kaum etwas hinzu.** Wer die Einschätzung des Netzes kennt, gewinnt aus der Handbewertung kaum Information. Das ist teils bauartbedingt, weil das Netz sie als Eingabe sieht. Liegen beide weit auseinander, hat meist das Netz recht.
- **Es erkennt den Sieger früher.**
  - So gut wie die Handbewertung 8 bis 11 Züge vor Schluss ist das Netz schon 16 bis 23 Züge vor Schluss, also etwa zwei Runden früher.
  - Schon in Zug 1 kennt es die Deckpaarungen etwa so gut wie eine Tabelle der Deck-Siegquoten.

### Worauf es sich stützt

Gemessen, indem die Spalten einer Merkmalsgruppe zwischen Stellungen vertauscht werden: Um so viel steigt der Vorhersagefehler (Log-Loss). Netz v5, frische Daten:

| Merkmalsgruppe | Trainingsdecks | nur Prüfdecks am Tisch |
|---|---:|---:|
| eigene Karten auf dem Brett (zusammengefasste Eigenschaften) | +0,129 | +0,046 |
| eigene Zählwerte (Leben, Hand, Bibliothek, Brett, Stärke) | +0,095 | +0,093 |
| eigene Handkarten | +0,041 | +0,026 |
| Karten der Gegner auf dem Brett, je Gegner | +0,025 bis +0,031 | +0,016 bis +0,022 |
| Zählwerte der Gegner, je Gegner | +0,012 bis +0,017 | +0,009 bis +0,011 |
| Kommandeur-Blöcke (Zauber aus der Zone, Kommandeurschaden) | +0,009 | +0,017 |
| Handbewertung für sich selbst | +0,022 | +0,020 |
| Handbewertung der Gegner | +0,002 | −0,003 |
| Gift, Monarch, Lebensgewinn im Zug (je Sitz) | ≤ +0,001 | ≤ +0,002 |

- **Es stützt sich vor allem auf sich selbst.** Das eigene Brett und die eigenen Zählwerte tragen mehr als alle drei Gegner zusammen.
- **Mit nur Prüfdecks am Tisch fällt das eigene Brett von +0,129 auf +0,046**, die Zählwerte bleiben gleich. Zwei Lesarten passen dazu:
  - Was das Netz aus den Karteneigenschaften gelernt hat, gilt vor allem für die Karten der 40 Trainingsdecks.
  - Mit nur 5 Decks unterscheiden sich die Bretter weniger, und Vertauschen kostet deshalb weniger.

  Zusammen mit dem fehlenden Vorsprung an den Prüfdecks spricht mehr für die erste Lesart, bewiesen ist sie nicht.
- **Gegenüber v3** stützt sich v5 weniger auf die Handbewertung (von +0,032 auf +0,022) und mehr auf die Bretter der Gegner (von +0,024 auf +0,029 je Gegner).
- **Kommandeurschaden** steckt in zwei Gruppen, das Vertauschen einer Gruppe misst ihn daher nur zum Teil. Laut Regression bestraft das Netz erlittenen Kommandeurschaden.
- **Vorbehalt:** Die Zahlen messen, wie sehr sich das Netz bei gegebenen anderen Gruppen auf eine Gruppe stützt. Weil vertauschte Eingaben auch unrealistische Stellungen erzeugen, sind sie ungefähre Einzelbeiträge, weder Ober- noch Untergrenze.

### Was die Handbewertung falsch macht

- **Material zählt zu viel.**
  - Die Handbewertung gibt 3 Punkte je Land, 10 je sonstiger bleibender Karte und zusätzlich zweimal Stärke plus Widerstand je Kreatur. Ein Schatz- oder Hinweis-Spielstein zählt so viel wie eine echte Karte.
  - In Partien des Heuristik-Bots und von `hsearch` sind die größten Abweichungen meist Bretter, die der Handbewertung stark erscheinen und dem Netz nicht: breite Spielstein- oder Artefaktbretter bei niedrigem Leben und laufendem Kommandeurschaden. Das Netz hatte dort meist recht.
  - In Partien der Netz-Piloten überwiegt der umgekehrte Fall.
  - Ob die Stellvertreter-Länder im Deckpool gerade Go-wide-Decks schwächen und die Handbewertung deshalb hier besonders danebenliegt, ist ungeprüft.
- **Ausscheidende Gegner zählen zu wenig.** Die Handbewertung vergleicht nur mit dem stärksten Gegner und setzt Gleichstand auf 0,5, auch wenn noch drei Gegner leben. Fällt die Zahl der Gegner von 3 auf 1:
  - steigt die Handbewertung um 0,11 bis 0,15,
  - das Netz um 0,17 (Partien des Heuristik-Bots) bis 0,27 (v5-Partien),
  - die tatsächliche Siegquote um 0,23 bis 0,27.
- **Eine Suche findet die Fehler ihrer Bewertung.** In `hsearch`-Partien überschätzt die Handbewertung den Kandidaten stärker als in den Partien des gleich starken v3. Im Mittel liegt sie +0,037 über dem Ausgang, bei v3 −0,028 darunter (Abstand 0,068 [0,040; 0,094]). Beim Netz zeigt sich der Effekt schwächer.

### Die Einschätzungen sind verschoben

| Datensatz | mittlere Einschätzung v3 | mittlere Einschätzung v5 | tatsächlich gewonnen |
|---|---:|---:|---:|
| Heuristik-Bot | 0,306 | 0,348 | 0,310 |
| v5 sucht, Trainingsdecks | 0,384 | 0,452 | 0,497 |
| v5 sucht, nur Prüfdecks am Tisch | 0,418 | 0,504 | 0,543 |

- **Ursache:** Das Netz bewertet eine Stellung, nicht den Spieler. Gelernt hat es aus Partien, in denen auf dem gemessenen Platz ein Pilot mit 15 % zufälligen Zügen und einem älteren Netz saß, gemischt mit Partien des Heuristik-Bots. Seine Zahlen passen deshalb zu einem Piloten mit 30 bis 35 % Siegquote. Am stärksten ist die Verschiebung früh in der Partie, bevor das Brett zeigt, wie gut jemand spielt.
- **Jede Runde hat sie verkleinert:** in v5-Partien von −0,11 bei v3 auf −0,045 bei v5.
- **Die Markierung beim Training** (`--mark`, eine zusätzliche Eingabe für die Partien der Suche) sollte genau das ausgleichen. Ob sie wirkt, ist offen: Früh in der Partie gibt v5 etwa die mittlere Siegquote aller Trainingsdaten aus.
- **Für die Zugwahl ist die Verschiebung vermutlich zweitrangig**, weil die Suche Optionen untereinander vergleicht. Sie stört aber, wo Einschätzungen auf feste Werte treffen (Sieg = 1, Niederlage = 0). Mit der Erkundungskonstante der Suche könnte sie wechselwirken, geprüft ist das nicht.

## Was in den Runden passiert ist

| | v3 | v4 | v5 | hsearch |
|---|---:|---:|---:|---:|
| Trainingsdaten | 100.000 Partien, 15 % Zufallszüge | + 60.000 mit v3-Suche | + 60.000 mit v4-Suche | – |
| Siegquote, alle Decks, 25 Simulationen | 38,7 % | 42,2 % | 45,3 % | 38,8 % |
| Siegquote, Prüfdecks (Arena), 25 / 100 Simulationen | 25,8 / 35,4 % | 26,4 / 35,8 % | 29,3 / 39,1 % | 29,1 / 33,3 % |
| Zauber pro eigenem Zug | 0,97 | 1,00 | 1,02 | 1,02 |
| Pässe trotz möglichem Zauber, pro eigenem Zug | 0,83 | 0,69 | 0,56 | 0,57 |
| Fähigkeiten pro eigenem Zug | 0,75 | 0,79 | 0,84 | 0,79 |
| Angreifer pro eigenem Zug | 1,24 | 1,26 | 1,33 | 1,50 |
| Schaden an Gegnern pro eigenem Zug | 3,50 | 3,74 | 3,96 | 4,49 |
| Lebensverlust pro gegnerischem Zug | 0,98 | 0,99 | 0,96 | 1,02 |
| Gegner auf dem eigenen Zug ausgeschaltet, pro Partie | 1,05 | 1,07 | 1,14 | 1,04 |

- **v3 hat schon fast alles Defensive:** seltener angreifen, breiter angreifen, das größte Brett meiden, weniger Leben verlieren. Trotzdem war es nur gleich stark wie `hsearch`, weil es zu passiv war: Es ließ knapp die Hälfte mehr Zauber liegen und baute langsamer auf.
- **Die Runden haben das Tempo zurückgebracht.** Von v3 zu v5 stiegen Zauber, Fähigkeiten, Kommandeurzauber und Angreifer pro Zug deutlich. Die Pässe trotz möglichem Zauber sanken um ein Drittel, die Stärke auf dem Brett im 6. eigenen Zug stieg um 0,8. Handkarten und Leben gingen dabei nicht verloren.
- **Auch an den Prüfdecks hat jede Runde geholfen**, bei 25 Simulationen von 25,8 % auf 29,3 %. Abgeflacht ist der Zuwachs bisher nicht.
- **Die Vorhersage wurde dabei kaum besser** (AUC 0,826 auf 0,834 in v5-Partien), die Verschiebung der Einschätzungen aber kleiner.
- **Kosten je Runde:** etwa 50 Minuten Aufzeichnung (60.000 Partien auf 14 Kernen), unter einer Minute Training, 15 Minuten Messung.

## Gefundene Fehler und Grenzen

- **Absturz in der Merkmalscodierung, behoben.**
  - Ursache: Die Merkmalscodierung der Engine (`dirac/feat`) sammelte die Ziele eines Stapeleintrags in einem festen Feld für 16 Objekte. Ein Eintrag kann mehr nennen, etwa „beliebig viele Kreaturen" auf vollem Brett.
  - Betroffen war jede Suche mit Netz in solchen Stellungen: 14 der 15 „failed"-Partien im 25er-Lauf.
  - Behoben im Fork (Commit `cd5605f381`, mit Test), Pull-Request bei Upstream: [#4855](https://github.com/adams-shaun/gorge/pull/4855).
  - Die bisherigen Arena-Zahlen sind davon um höchstens 7 von 1.600 Partien je Netz betroffen.
- **Lücken im Kern:** 3 bis 4 % der Partien brechen ab, gleich oft bei allen Piloten. Dazu kommen seltene Abstürze im Kern („autopay found no plan", „a decision with no options").
- **Messlücken im Werkzeug:**
  - Die Beschreibung eines Angriffs nennt den angegriffenen Gegner nicht.
  - Bei Zielentscheidungen fehlen Quelle und Optionsliste.
  - Werte je Option an der Wurzel der Suche fehlen.
  - Die Proben nutzen immer denselben Seed der Suche.
  - Mit diesen Angaben ließen sich die Befunde zu Kampf und Zielwahl direkt statt über Umwege belegen.
- **Gegner:** Alle Befunde gelten gegen drei Heuristik-Bots.

## Nachtrag: gegen stärkere Gegner

Schritt 1 der folgenden Liste ist erledigt (Zahlen und Einordnung in der [README](../README.md#gegen-stärkere-gegner)):

- **Gegen drei Suchen mit Handbewertung** gewinnt v5 30,6 %, die Handbewertung auf demselben Platz 23,7 % (+6,9 [+4,2; +9,6]). Der Vorsprung hält dort voll.
- **Gegen drei Kopien von v5** sind es 23,7 % gegenüber 22,5 % (+1,2 [−1,4; +3,8]). Dort ist kein Vorsprung messbar.

Damit ist die Frage aus der Kurzfassung beantwortet: Ein Teil des Gelernten ist gut gegen Gegner der Heuristik-Bot-Familie, nicht allgemein. Schritt 2, Selbstspiel mit Netzen auch auf den Gegnerplätzen, ist damit der nächste.

## Nachtrag: Selbstspiel gegen gemischte Gegner

Auch Schritt 2 ist erledigt, mit negativem Ergebnis (Zahlen in der [README](../README.md#dritte-runde-gemischte-gegner-kein-fortschritt)): Netz v6 ist gegen Netz-Gegner nicht besser als v5 und gegen Heuristik-Bots schwächer. Der wahrscheinliche Grund: Die Bewertung ist eine Zahl ohne Wissen über die Gegner, und im Suchbaum spielt die Gegner immer der Heuristik-Bot. Die Schritte 5, 7 und 8 der folgenden Liste rücken damit nach vorn, ergänzt um ein Gegnermodell in der Suche.

## Nächste Schritte

Die Reihenfolge folgt aus den Befunden:

- **Gemessene Schwachstelle:** die Übertragung auf neue Decks.
- **Wichtigste offene Frage:** Hält der Vorsprung gegen andere Gegner?
- **Einziger Hebel mit nachgewiesener Wirkung:** das Selbstspiel.
- **Größere Umbauten** folgen erst, wenn eine Messung ihren Nutzen zeigt.

1. **Gegen stärkere Gegner messen.** Die Arena bekommt Tische, an denen die Gegner selbst suchen (`hsearch:25`), und Tische mit v5 auf mehreren Plätzen. Ein konkretes Risiko: Gegen v5 bauen die Gegner größere Bretter auf, und Gegner, die suchen, nutzen das womöglich. Der Aufwand ist klein, die Aussagekraft groß.
2. **Die nächste Selbstspiel-Runde gegen gemischte Gegner.** In den Trainingspartien sitzen dann Heuristik-Bots, `hsearch` und frühere Netze, und nach jeder Runde werden die Prüfdecks gemessen. Das verbindet den bisher einzigen nachgewiesenen Hebel (rund 3 Punkte je Runde, auch an den Prüfdecks) mit der Gegnerfrage.
3. **Übertragung auf neue Decks.**
   - Trainingsdecks abwandeln (Karten zwischen Decks tauschen), damit das Netz keine Listen lernen kann;
   - mehr Decks;
   - die bisher ungenutzten Merkmale der Engine prüfen (Kartennamen, taktische Merkmale wie „tödlich ungeblockt");
   - Prüfdatensätze aufzeichnen, deren Tische wie in der Arena aufgebaut sind: nur der Kandidat mit Prüfdeck.
4. **Gegenproben, was den Vorsprung trägt.** Billige Versuche in der Arena:
   - v5, aber die erste Angriffsentscheidung trifft `hsearch`;
   - v5, aber Fähigkeiten, die Spielsteine erzeugen, wählt `hsearch`;
   - v5 mit einer Regel, die Schadens- und Zerstörungseffekte nicht auf eigene Karten lenkt, solange ein gegnerisches Ziel möglich ist.

   Was davon Siege kostet oder bringt, zeigt, wo das Netz wirklich besser ist.
5. **Werkzeug erweitern:** Angriffsziel in der Aktionsbeschreibung, Quelle und Optionen bei Zielentscheidungen, Werte je Option an der Wurzel, Proben mit anderem Seed. Die Werte je Option sind die Grundlage, um Netze ohne Arena vergleichen zu können, und für jede Arbeit an der Zugauswahl.
6. **Einschätzungen geraderücken.** Prüfen, ob die Markierung beim Training wirkt. Als Trainingsziel nur Plätze ohne Zufallszüge verwenden, und die Wirkung in der Arena messen.
7. **Zugauswahl lernen.** Ein Netz schlägt der Suche Optionen vor. Das soll fast gleichwertige Optionen entscheiden, an denen heute die Reihenfolge entscheidet, und die Simulationen auf wichtige Züge lenken. Upstream hat dieses Verfahren zu zweit erfolgreich gezeigt.
8. **Bewertung pro Sitz statt als eine Zahl** (mit der max^n-Suche der Engine). Das skalare Netz bildet Ausscheiden bereits ab. Ob eine Bewertung je Sitz Spielstärke bringt, ist offen. Sie lohnt sich erst, wenn die Werte je Option (Schritt 5) einen Mangel des skalaren Netzes zeigen.
9. **Kleinere Punkte:** Mulligan und Landwahl zu Entscheidungen der Suche machen (heute beantwortet sie immer der Heuristik-Bot).

## Werkzeuge und Daten

- `podbench study`: dieselben Tische wie die Arena, mehrere Piloten nacheinander, Aufzeichnung je Partie. Mit `-probe` fragt es andere Piloten an den Entscheidungen eines Piloten, mit `-value` bewertet es jeden Zug mit einem Netz und der Handbewertung.
- [analysis/study.py](../analysis/study.py): fasst einen `study`-Lauf zusammen.
- [analysis/nets.py](../analysis/nets.py): Vorhersagegüte, Kalibrierung und Merkmalsgruppen der Netze.
- Ergebnisse: [results/analysis/](../results/analysis/). Die Rohdaten (`data/study`, `data/test`, zusammen etwa 1 GB) sind nicht eingecheckt und lassen sich mit diesen Befehlen neu erzeugen:

```sh
bin/podbench study -pilots hbot,hsearch:25,net:models/v3.caiv:25,net:models/v4b.caiv:25,net:models/v5.caiv:25 \
    -probe net:models/v5.caiv:25 -alts self,hbot,hsearch:25 -value models/v5.caiv -n 1600 -out data/study/s25.jsonl
bin/podbench study -pilots hsearch:100,net:models/v5.caiv:100 -probe net:models/v5.caiv:100 -alts self,hsearch:100 \
    -every 6 -value models/v5.caiv -n 800 -out data/study/s100.jsonl
bin/podbench record -split train -pilot hbot -n 4000 -every 20 -seed 1100001 -out data/test/hbot-train
bin/podbench record -split train -pilot net:models/v5.caiv:25,hbot,hbot,hbot -keep 0 -n 3000 -every 20 -seed 1200001 -out data/test/v5-train
bin/podbench record -split eval -pilot net:models/v5.caiv:25,hbot,hbot,hbot -keep 0 -n 3000 -every 20 -seed 1300001 -out data/test/v5-eval
.venv/bin/python analysis/study.py data/study/s25.jsonl --arena results/arena --out results/analysis/s25.json
.venv/bin/python analysis/study.py data/study/s100.jsonl --out results/analysis/s100.json
.venv/bin/python analysis/nets.py --nets models/v2.caiv models/v3.caiv models/v4b.caiv models/v5.caiv \
    --importance models/v3.caiv models/v5.caiv --data data/test/hbot-train data/test/v5-train data/test/v5-eval \
    --limit 40000 --out results/analysis/nets.json
```

Der Lauf mit 25 Simulationen wurde vor dem Fix der Merkmalscodierung gespielt, seine 15 abgestürzten Partien zählen als Niederlagen. Der Lauf mit 100 Simulationen und die Prüfdatensätze entstanden danach.
