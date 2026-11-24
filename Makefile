# commander_ai: four-seat Commander simulation on the dirac kernel of gorge.
#
#   make setup      engine checkout at the pinned revision, card corpus, tables, podbench
#   make test       frozen deck lists and the committed baseline must reproduce
#   make baseline   rewrite results/baseline (after a deliberate engine or deck change)
#   make reference  the four original decks on gorge's broad engine (slow, plays everything)

ENGINE  := third_party/gorge
GAMES   ?= 1000
ORIGINALS := foundations-calling-all-angels,foundations-keen-engineering,foundations-reign-of-dragons,foundations-wretched-ranks

.PHONY: setup engine build test baseline reference clean

setup: engine build

# The Forge card scripts are GPL-3.0. They are fetched into the engine checkout's
# gitignored .cards/ as a build input and are never committed here.
engine:
	git submodule update --init $(ENGINE)
	$(MAKE) -C $(ENGINE) fetch-cards compile-cards bin/mtgsim dirac-tables

build:
	go build -o bin/podbench ./cmd/podbench

test:
	go vet ./...
	go test ./...

# `run -table original` exits 3 while the kernel still refuses the original pod.
baseline: build
	bin/podbench cards -table original -out results/baseline
	bin/podbench run -table original -n $(GAMES) -out results/baseline; rc=$$?; [ $$rc -eq 0 ] || [ $$rc -eq 3 ]
	bin/podbench run -table adjusted -n $(GAMES) -out results/baseline
	bin/podbench run -table mirror -n $(GAMES) -out results/baseline

reference:
	cd $(ENGINE) && GOMAXPROCS=1 /usr/bin/time -p bin/mtgsim -format commander -seats 4 -mulligans 1 \
		-deckdir $(CURDIR)/decks/original -decks $(ORIGINALS) -games 100 -seed 1

clean:
	rm -rf bin
