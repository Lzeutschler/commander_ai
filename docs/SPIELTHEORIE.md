# Was ein Netz über Commander zu viert verrät

Stand 25.11.2026. Dieses Dokument stellt die auffälligsten Spielmuster von Netz v5 heraus, deutet sie spieltheoretisch und vergleicht sie mit dem, was die Commander-Community für richtig hält. Die Messungen dahinter stehen in [ANALYSE.md](ANALYSE.md), die Zusatzauswertung für dieses Dokument in [results/analysis/table.json](../results/analysis/table.json).

Der Reiz daran: Commander zu viert ist ein Spiel, für das es keine fertige Theorie gibt. Vier Spieler, verdeckte Hände, wechselnde Interessen. Die Community hat dafür über zwanzig Jahre Faustregeln gesammelt, aber fast nie Daten. Wir haben jetzt ein Labor, in dem dieselbe Partie mit denselben Karten fünfmal gespielt wird und sich nur der Pilot auf einem Platz ändert. Niemand hat dem Netz eine dieser Faustregeln gesagt. Trotzdem hat es mehrere davon selbst gefunden, und an zwei Stellen widerspricht es ihnen.

## Kurzfassung

- **Das Netz hat „Second Best" von allein entdeckt.** Es hat seltener das größte Brett als die Suche mit Handbewertung, scheidet aber viel seltener als Erster aus (16 % gegenüber 22 %) und gewinnt die langen Partien. Das ist ziemlich genau, was Sheldon Menery 2013 als Philosophie beschrieben hat.
- **Angreifen kostet in der Folgerunde.** Nach einem Zug ohne Schaden verliert ein Spieler bis zu seinem nächsten Zug etwa 1,2 Leben, nach einem Angriff auf das größte Brett 2,2 bis 3,0. Das stützt die Seite der Community, die Angreifen im Mehrspielerspiel für teuer hält.
- **Den Stärksten anzugreifen ist ein Dienst am Tisch, den der Angreifer allein bezahlt.** Das Netz meidet das größte Brett. Die Community lehrt das Gegenteil. Beide haben recht, es kommt darauf an, ob der Tisch mitzieht.
- **Wer einen Spieler ausschaltet, hat davon fast nichts.** Er gewinnt danach 35 % der Partien, die beiden Unbeteiligten je 32 %.
- **Unter Bots fliegt der Schwächste zuerst, nicht der Stärkste.** Den „Archenemy"-Effekt gibt es nur an Tischen, die Bedrohungen gemeinsam einschätzen.
- **Zwei Volksweisheiten halten in unserem Labor nicht:** „Leben ist kein Punktestand" und „Wer anfängt, gewinnt öfter". Beim zweiten liegt es an einem Detail des Bots, und gerade das ist lehrreich.
- **Es gibt kein bestes Spiel, nur ein bestes Spiel gegen diesen Tisch.** Gegen drei Kopien von sich selbst ist der Vorsprung des Netzes weg.

## Das Labor und was es nicht kann

Damit die Schlüsse richtig eingeordnet werden, vorab die Grenzen:

- **Die Gegner sind Bots ohne Gedächtnis.** Der Heuristik-Bot kennt keine Rache, keine Absprachen und keine Einschätzung, wer gerade gewinnt.
- **Seine Angriffsregel ist einfach:** Er greift an, wo es tödlich ist, sonst wo niemand blocken kann, sonst wo der Tausch gut ist. Bei Gleichstand trifft es den Spieler mit dem wenigsten Leben, danach den, der in der Sitzreihenfolge zuerst kommt.
- **Die Decks sind Precons, ausgedünnt um rund 15 Karten.** Es gibt keine Combos und wenig Spiel in gegnerischen Zügen. Das Labor bildet also kampflastiges Commander auf Precon-Niveau ab, kein cEDH.
- **Das Netz spricht nicht.** Es kann nicht verhandeln, drohen oder bluffen.

Alles Folgende sind deshalb Hypothesen für echte Tische, keine Beweise. Wo ein Befund an einer Eigenheit des Bots hängt, steht es dabei.

## Die Befunde im Vergleich mit der Community

### 1. Angreifen hat einen Preis, und er wird in der Folgerunde bezahlt

**Was wir messen.** Lebensverlust bis zum eigenen nächsten Zug, solange drei Gegner leben:

| im eigenen Zug | Suche mit Handbewertung | Netz v5 |
|---|---:|---:|
| kein Schaden an Gegnern | 1,20 | 1,26 |
| Schaden vor allem am kleinsten Brett | 2,00 | 1,64 |
| Schaden vor allem am größten Brett | 2,96 | 2,24 |

Das Netz zieht daraus die Konsequenz: Zu Beginn eines Zuges verzichtet es etwa 1,8-mal so oft ganz auf den Angriff. Wenn es angreift, dann mit mehr Kreaturen. Der Grund für den Preis steckt in der Angriffsregel der Bots: Wer angreift, steht getappt da, und getappte Bretter sind das bevorzugte Ziel.

**Was die Community sagt.** Hier gibt es zwei Lager:

- **Angreifen ist teuer.** Cas Hinds nennt es auf EDHREC „möglicherweise die schlechteste Spielaktion": Der Angreifer legt sich fest, der Verteidiger wählt, und wer einen Spieler unter Druck setzt, steht drei anderen offen gegenüber. Kristen Gregory rät bei Card Kingdom schlicht, Blocker zu spielen, weil Spieler dorthin angreifen, wo es nichts kostet. Anthony Alongi hat dafür schon um 2000 den Begriff „Rattlesnake" geprägt: Karten, die abschrecken, statt zu handeln.
- **Greift mehr an.** Travis Norman hält in „An Ode to Chip Damage" dagegen: Ohne frühe kleine Angriffe bauen alle ungestört ihre Festung, und die Angst vor Feinden sei übertrieben.

**Deckt sich das?** Ja, und zwar mit beiden. Die Messung stützt Hinds und Gregory: Der Preis des Angreifens ist real und messbar. Unsere Rundengeschichte stützt aber auch Norman. Netz v3 hatte die Zurückhaltung schon und war trotzdem nur so stark wie die Handbewertung, weil es insgesamt zu passiv war. Erst als das Selbstspiel das Tempo zurückbrachte, kamen die Siege. Die Synthese des Netzes lautet: selten angreifen, aber dann mit allem, und in der Zwischenzeit das Mana trotzdem ausgeben.

### 2. Wen angreifen? Nicht den Stärksten

**Was wir messen.** Das Netz fügt dem Gegner mit dem größten Brett in 15 % seiner Züge Schaden zu, die Suche mit Handbewertung in 28 %. Bei gezielten Effekten wählt es seltener den Spieler mit der meisten Stärke (52 % gegenüber 57 %).

**Was die Community sagt.** Fast einhellig das Gegenteil. „Threat Assessment" heißt die Kernkompetenz: herausfinden, wer am ehesten gewinnt, und dort ansetzen. Draftsim, EDHREC und Nerd Leagues lehren es so, und im Forum heißt es: zuerst das stärkste Brett.

**Deckt sich das?** Nein, und der Widerspruch ist der spannendste Punkt der ganzen Analyse. Spieltheoretisch ist das Eindämmen des Führenden ein öffentliches Gut: Alle drei anderen profitieren davon, aber bezahlt wird es von dem, der angreift. Er verliert danach am meisten Leben (siehe Tabelle oben). Das ist das klassische Trittbrettfahrer-Problem. An einem Tisch, an dem niemand sonst den Führenden angreift, ist es für den Einzelnen besser, es ebenfalls zu lassen. Genau so ein Tisch sind drei Bots.

An einem menschlichen Tisch löst die Community das Problem mit Sprache und Normen: „Er ist die Bedrohung, hilfst du mir?" Threat Assessment ist also weniger eine Rechenregel als eine Absprache, wer die Kosten trägt. Wo die Absprache fehlt, hat das Netz recht. Wo sie funktioniert, hat die Community recht. Unsere Gegenprobe passt dazu: Gegen drei Kopien von sich selbst, die alle dem größten Brett ausweichen, ist der Vorsprung des Netzes verschwunden.

### 3. „Second Best" entsteht von selbst

**Was wir messen.** Je 1.600 Partien auf denselben Tischen. Das größte Brett ist zu Beginn von Runde 7 gemessen, wenn noch alle vier leben:

| Kandidat | hat das größte Brett | scheidet als Erster aus | Siegquote |
|---|---:|---:|---:|
| Heuristik-Bot | 22 % | 23 % | 25 % |
| Suche mit Handbewertung | 33 % | 22 % | 39 % |
| Netz v5 | 28 % | 16 % | 45 % |

Die Suche mit Handbewertung baut am schnellsten das größte Brett. Das Netz ist seltener vorn, aber viel seltener als Erster draußen, und es gewinnt die Partien, die länger als 12 eigene Züge dauern.

**Was die Community sagt.** Sheldon Menery hat das 2013 als „The Philosophy of Being Second Best" beschrieben, mit fünf Elementen: ordentlicher Start, leiser Aufbau, keinen Zorn auf sich ziehen, die anderen sich gegenseitig abnutzen lassen, verhandeln.

**Deckt sich das?** Verblüffend gut. Vier der fünf Elemente zeigt das Netz, nur verhandeln kann es nicht. Der Grund ist aber ein anderer als bei Menery. Bei ihm geht es um Ruf: Wer bedrohlich aussieht, wird von allen angegriffen. Bots haben kein Gedächtnis und keinen Blick für Bedrohung. Das Netz spielt „Second Best", weil ein offenes, getapptes Brett bestraft wird, nicht weil ein großes Brett Feinde macht. Dass dieselbe Strategie aus zwei verschiedenen Gründen herauskommt, spricht dafür, dass sie robust ist.

### 4. Unter Bots fliegt der Schwächste zuerst

**Was wir messen.** Vier Heuristik-Bots, Stand zu Beginn von Runde 5:

| Rang am Tisch | gewinnt | scheidet als Erster aus |
|---|---:|---:|
| größte Stärke auf dem Brett | 38 % | 10 % |
| kleinste Stärke auf dem Brett | 14 % | 48 % |
| meistes Leben | 31 % | 13 % |
| wenigstes Leben | 13 % | 54 % |

**Was die Community sagt.** Wer vorn liegt, wird zum „Archenemy" und von allen gejagt. Daraus folgt der Rat, nicht zu früh zu stark auszusehen.

**Deckt sich das?** Nein, an unserem Tisch gewinnt der Führende am häufigsten und der Schwächste stirbt zuerst. Der Archenemy-Effekt ist demnach keine Eigenschaft des Spiels, sondern eine der Spieler: Er entsteht erst, wenn ein Tisch Bedrohungen gemeinsam einschätzt und sich gegen den Führenden zusammentut. Ohne diese Koordination ist Commander ein Spiel, in dem der Stärkste davonzieht und alle auf den Schwächsten einschlagen. Für echte Runden heißt das: Threat Assessment ist das, was das Format im Gleichgewicht hält. Ein Tisch, der es nicht betreibt, spielt ein anderes und einseitigeres Spiel.

### 5. Wer einen Spieler ausschaltet, beschenkt den Tisch

**Was wir messen.** Nach der ersten Ausschaltung an einem Tisch aus vier Bots gewinnt der, der sie herbeigeführt hat, 35,1 % der Partien. Die beiden Unbeteiligten gewinnen je 32,5 %. Der Unterschied ist statistisch nicht gesichert, obwohl der Ausschaltende im Schnitt der Stärkere sein dürfte. Das Netz verhält sich passend: Solange vier Spieler leben, schaltet es seltener jemanden aus als die Suche mit Handbewertung, selbst wenn es könnte. Dabei bewertet es ein Ausscheiden durchaus hoch, es wartet nur lieber, bis ein anderer die Arbeit macht.

**Was die Community sagt.** Sie ist gespalten. Die einen wollen einen Gegner schnell herausnehmen, weil dann weniger Karten und weniger Antworten am Tisch sind. Die anderen verteilen den Schaden, damit alle lange mitspielen. Die zweite Gruppe argumentiert fast immer sozial, nicht strategisch.

**Deckt sich das?** Das Ergebnis gibt den Verteilern recht, aus einem strategischen Grund, den sie selbst selten nennen: Eine Ausschaltung nützt allen Überlebenden gleich, die Kosten trägt einer. Es ist dasselbe Muster wie beim Angriff auf den Führenden. Die Ausnahme ist die Ausschaltung, die das Spiel bald beendet. Im Zweikampf am Ende spielt das Netz so entschlossen wie die Handbewertung.

### 6. Leben ist doch ein Punktestand, zumindest auf diesem Niveau

**Was wir messen.** Unter vier Bots sagt der Lebensrang ab Runde 7 den Sieger so gut voraus wie die Stärke auf dem Brett (AUC je 0,70). Wer dann das wenigste Leben hat, gewinnt 7 % der Partien und scheidet in 71 % als Erster aus.

**Was die Community sagt.** „Life total is not how you keep score in Commander", schreibt Roman Milan bei EDHREC. Was zähle, seien Karten, Mana und Brett.

**Deckt sich das?** Nur teilweise. Milans Satz stimmt für Runden, in denen Combos und Kartenmaschinen entscheiden. In kampflastigen Runden auf Precon-Niveau ist Leben ein sehr guter Zähler. Ein Teil davon liegt am Bot, der bei Gleichstand den Spieler mit dem wenigsten Leben angreift. Menschen tun das allerdings auch gern.

### 7. Kartenvorteil ja, Horten nein

**Was wir messen.**

- **Volle Hand heißt bei Bots: festgefahren.** Wer zu Beginn von Runde 5 die meisten Karten auf der Hand hat, gewinnt nur 21 %, wer die wenigsten hat, 27,5 %.
- **Das Netz spielt gleich viele Karten wie die Handbewertung, aber andere.** Es erzeugt viel seltener Spielsteine (49 gegenüber 387 solcher Fähigkeiten) und nutzt öfter Ausrüstung und Kartenziehen. Auf der Hand hält es nur 0,1 bis 0,35 Karten mehr.
- **Die Handbewertung liegt bei breiten Brettern daneben.** Sie zählt jeden Spielstein wie eine echte Karte, und genau diese Stellungen hält das Netz für schwächer, meist zu Recht.

**Was die Community sagt.** Kartenvorteil gilt als die wichtigste Ressource. Dazu kommt die Warnung, sich nicht zu überdehnen: Wer alles aufs Brett legt, verliert alles an einen Massenvernichter.

**Deckt sich das?** Im Kern ja, mit einer Präzisierung. Karten auf der Hand sind für sich kein Vorteil. Zählbar wird Kartenvorteil erst als Kartenfluss: Karten ziehen und ausspielen. Die Abneigung des Netzes gegen breite Spielsteinbretter passt zur Warnung vor dem Überdehnen. Ob wirklich Massenvernichter der Grund sind, haben wir nicht gemessen.

### 8. Kommandeurschaden beendet die langen Partien

**Was wir messen.** 10,1 % der Siege des Netzes enden mit Kommandeurschaden, bei der Handbewertung 5,8 %. Der Unterschied entsteht in Partien ab dem 10. eigenen Zug. Das Netz wirkt seinen Kommandeur auch öfter erneut.

**Was die Community sagt.** Wer über Kommandeurschaden gewinnen will, soll laut Wargamer möglichst früh einen Spieler herausnehmen, weil dann weniger Gegner Antworten haben.

**Deckt sich das?** Nur halb. Das Netz nutzt Kommandeurschaden nicht als frühen Plan, sondern als Abschluss, wenn die Partie lang wird und nur noch wenige Gegner leben. In sehr schnellen Siegen nutzt ihn die Handbewertung sogar öfter.

### 9. Wer anfängt, verliert? Ein Lehrstück über Konventionen

**Was wir messen.** An Tischen aus vier Bots gewinnt der erste Sitz 22,7 %, der letzte 26,6 %. Der erste Sitz scheidet in 35 % der Partien als Erster aus, der letzte in 16 %.

**Was die Community misst.** Das Gegenteil. Playgroup wertet 304.458 Viererpartien aus seinem Lebenspunktezähler aus: Sitz 1 gewinnt 29,1 %, Sitz 4 nur 21,5 %. Turnierdaten aus dem cEDH zeigen dasselbe Gefälle.

**Deckt sich das?** Nein, und der Grund ist ein Detail: Der Bot greift bei völligem Gleichstand den Spieler an, der in der Sitzreihenfolge zuerst kommt. Der erste Sitz ist damit das Standardziel von drei Gegnern. Das ist eine Eigenheit unseres Labors und kein Befund über Magic. Lehrreich ist es trotzdem: Eine winzige Konvention, wen man im Zweifel angreift, dreht einen Sitzvorteil von sieben Punkten um. An echten Tischen gibt es solche Konventionen auch, etwa „wer Sol Ring legt, bekommt den ersten Angriff". Für uns ist es ein Auftrag, diese Verzerrung aus dem Bot zu nehmen. Die Arena-Vergleiche betrifft sie nicht, weil der Kandidat zufällig sitzt.

### 10. Was das Netz nicht tut

- **Kein Bluff, keine offenen Antworten.** Es hebt Spontanzauber nicht für gegnerische Züge auf. An einem Tisch aus Bots gibt es auch niemanden, den offenes Mana beeindruckt. Die Community diskutiert, wann sich das lohnt. Nerd Leagues weist darauf hin, dass ein Zug mit offenem Mana zu viert gegenüber drei Gegnern zugleich zurückwirft, nicht nur gegenüber einem.
- **Kein Mulligan, keine Landwahl.** Beides entscheidet in unserem Aufbau immer der Bot.
- **Kein Gedächtnis für Verhalten.** Es weiß nicht, wer es letzte Runde angegriffen hat.
- **Eine Ausnahme bei verdeckter Information:** Schon im ersten Zug schätzt das Netz die Paarung der Decks so gut ein wie eine Tabelle der Deck-Siegquoten, allein aus den offen liegenden Kommandeuren. Menschen tun dasselbe, wenn sie einen Kommandeur sehen und wissen, was auf sie zukommt.

## Die spieltheoretische Klammer

**Es gibt keinen sicheren Hafen.** Im Duell garantiert die Spieltheorie eine Strategie, mit der man im Schnitt nicht verliert, egal was der Gegner tut. Ab drei Spielern gibt es diese Garantie nicht mehr. Die Entwickler des Poker-Programms Pluribus haben das 2019 deutlich gesagt: Zu sechst kann man selbst mit einem exakten Gleichgewicht verlieren, deshalb haben sie auf die Garantie verzichtet und einfach gemessen, was gegen echte Gegner gewinnt. Unser Projekt steht an derselben Stelle.

**Unsere Zahlen zeigen das direkt.** Gegen drei Heuristik-Bots liegt das Netz 6,5 Punkte vor der Handbewertung, gegen drei Suchen mit Handbewertung 6,9 Punkte, gegen drei Kopien von sich selbst noch 1,2 Punkte. Was es gelernt hat, ist zum großen Teil eine Antwort auf diesen Tisch. Commander-Spieler kennen das als „Kenne deine Runde".

**Zwei Befunde, ein Muster.** Den Führenden eindämmen und einen Spieler ausschalten sind beides öffentliche Güter mit privaten Kosten. Ein Spieler, der nur seinen eigenen Sieg maximiert und nicht reden kann, liefert davon zu wenig. Das Netz tut genau das. Menschliche Tische haben dafür Sprache, Ruf und Vergeltung erfunden. Was die Community „Politik" nennt, ist aus dieser Sicht kein Beiwerk, sondern die Lösung eines echten strategischen Problems.

**Die Suche denkt paranoid.** Unsere Baumsuche behandelt die drei Gegner als ein Team, das gemeinsam gegen den Kandidaten spielt. Das ist die einfachste Annahme und gegen Bots gut genug. Sie kann aber nicht abbilden, dass zwei Gegner sich gegenseitig schaden. Die Alternative aus der Literatur, max^n, gibt jedem Spieler eigene Ziele. Ob sie hier hilft, ist offen. Eine erste Messung zeigt aber, dass die Koalitionsannahme kostet: Spielt im Suchbaum einfach der Heuristik-Bot die Gegner, gewinnt v5 bei 25 Simulationen 51,4 % statt 45,3 %, allerdings bei knapp vierfacher Rechenzeit. Wer annimmt, dass sich der ganze Tisch gegen ihn verschworen hat, spielt schlechter.

## Faustregeln für den echten Tisch

Als Hypothesen für kampflastige Runden auf Precon-Niveau, abgeleitet aus dem, was im Labor gewinnt:

1. **Greif nicht aus Gewohnheit an.** Ein Angriff kostet in der Folgerunde etwa ein bis zwei Leben zusätzlich. Er lohnt sich, wenn er etwas entscheidet oder der Gegner offen steht.
2. **Wenn du angreifst, dann richtig.** Halbe Angriffe zahlen den vollen Preis.
3. **Gib dein Mana trotzdem aus.** Zurückhaltung im Kampf ist gut, Zurückhaltung beim Ausspielen nicht. Daran ist Netz v3 gescheitert.
4. **Sei nie der mit dem wenigsten Leben und ohne Blocker.** Das ist die gefährlichste Position am Tisch.
5. **Den Führenden anzugreifen ist ein Dienst am Tisch.** Tu es, wenn andere mitziehen, und sag es laut. Allein bezahlst du dafür.
6. **Schalte niemanden als Erster aus, wenn es dich dein Brett kostet.** Der Gewinn verteilt sich auf alle.
7. **Kartenfluss schlägt Masse.** Eine Karte ziehen ist meist mehr wert als ein weiterer Spielstein.
8. **In langen Partien: Kommandeurschaden mitzählen.** Er ist oft der kürzere Weg.

Regel 5 ist die interessanteste. Das Netz befolgt nur ihren zweiten Teil, weil es nicht reden kann. Ein Mensch kann beides.

## Was wir im Projekt daraus machen

Jede dieser Deutungen lässt sich prüfen. Das ist der eigentliche Wert des Labors.

- **Bots mit Bedrohungseinschätzung.** Ein Gegner, der immer das größte Brett angreift, und einer, der sich rächt. Entsteht dann der Archenemy-Effekt? Lernt das Netz dann, kleiner auszusehen?
- **Bots mit Charakter.** Alongis Tiere als Gegnertypen: der Abschrecker, der Zerstörer, der Lauerer. Ein gemischter Tisch wäre ein härterer und realistischerer Maßstab als drei gleiche Gegner.
- **Die Sitzverzerrung beseitigen.** Gleichstände im Bot zufällig auflösen und den Sitzvorteil neu messen. Kommt dann das Gefälle der Community heraus?
- **Gegenproben zum Stil.** Dem Netz den Angriff aufzwingen oder verbieten und messen, was es kostet. Das steht schon in der [ROADMAP](ROADMAP.md).
- **Selbstspiel gegen gemischte Gegner.** Der nächste Trainingsschritt. Er sollte zeigen, ob „Second Best" auch gegen Gegner hält, die dasselbe versuchen.

## Quellen

Community:

- Cas Hinds, [The History of Combat and Why Attacking in Commander Sucks](https://edhrec.com/articles/the-history-of-combat-and-why-attacking-in-commander-sucks), EDHREC, 2026
- Travis Norman, [An Ode to Chip Damage](https://www.hipstersofthecoast.com/2019/07/an-ode-to-chip-damage/), Hipsters of the Coast, 2019
- Sheldon Menery, [The Philosophy of Being Second Best](https://articles.starcitygames.com/articles/the-philosophy-of-being-second-best/), Star City Games, 2013
- Kristen Gregory, [Stop Wasting Removal and Play Blockers in Commander](https://blog.cardkingdom.com/stop-wasting-removal-and-play-blockers-in-commander/), Card Kingdom, 2023
- Roman Milan, [How To Be New at Threat Assessment in Commander](https://edhrec.com/articles/how-to-be-new-at-threat-assessment-in-commander), EDHREC, 2026
- A. L. Walser, [The Ultimate Guide to Threat Assessment in Commander](https://draftsim.com/mtg-commander-threat-assessment/), Draftsim, 2025
- Nerd Leagues, [Threat Assessment in Commander: How to Pick Your Targets](https://www.nerdleagues.com/blog/threat-assessment-in-commander-how-to-pick-your-targets) und [When to Hold Up Interaction](https://www.nerdleagues.com/blog/when-to-hold-up-interaction-commander)
- Anthony Alongi, [Casual Fridays: Hall of Fame](https://articles.starcitygames.com/articles/casual-fridays-93-hall-of-fame-version-4-0-preview/), Star City Games (Rattlesnake, Gorilla, Spider, Pigeon, Plankton)
- TappedOut-Forum, [Kill 1 opponent fast vs spreading out the dmg evenly?](https://tappedout.net/mtg-forum/kitchen-table/edh-kill-1-opponent-fast-vs-spreading-out-the-dmg-evenly), 2023
- Card Kingdom, [The Cards You Play in Commander Don't Matter](https://blog.cardkingdom.com/the-cards-you-play-in-commander-dont-matter/) (Überdehnen)
- Wargamer, [MTG Commander damage rules](https://www.wargamer.com/magic-the-gathering/mtg-commander-damage-rules)
- Playgroup, [Does Going First Matter in Commander?](https://playgroup.gg/commander/turn-order) (304.458 Viererpartien, selbst gemeldet, kein cEDH)
- TopDeck, [First Player Advantage: Silicon Dynasty](https://topdeck.gg/articles/first-player-adv-silicon-dynasty) (cEDH-Turnierdaten)

Spieltheorie und KI:

- Noam Brown und Tuomas Sandholm, „Superhuman AI for multiplayer poker", Science 2019. Zusammenfassung: [Pluribus](https://ai.meta.com/blog/pluribus-first-ai-to-beat-pros-in-6-player-poker/)
- Luckhardt und Irani, „An Algorithmic Solution of N-Person Games" (max^n), 1986
- Sturtevant und Korf, „On Pruning Techniques for Multi-Player Games" (paranoide Suche), 2000

Die Artikel sind sinngemäß wiedergegeben. Das Zitat von Hinds ist aus dem Englischen übersetzt, das von Milan steht im Original.
