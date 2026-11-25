"""What decides a four-player game: seat order, resources, eliminations, retaliation.

    .venv/bin/python analysis/table.py data/study/s25.jsonl --out results/analysis/table.json

Reads a `podbench study` run. The games with the heuristic bot in every seat
(pilot hbot) are four equal players, so every seat is a sample: who wins by
seat order, by rank in a resource at the start of a round, and after the first
elimination. The search pilots' games add what follows a candidate's own turn.
"""

import argparse
import collections
import json
from pathlib import Path

import numpy as np


def auc(y, p):
    order = np.argsort(p, kind="stable")
    ranks = np.empty(len(p))
    # average ranks for ties
    sp = np.asarray(p)[order]
    i = 0
    while i < len(sp):
        j = i
        while j + 1 < len(sp) and sp[j + 1] == sp[i]:
            j += 1
        ranks[order[i : j + 1]] = (i + j) / 2 + 1
        i = j + 1
    y = np.asarray(y) > 0.5
    n1, n0 = y.sum(), (~y).sum()
    return float((ranks[y].sum() - n1 * (n1 + 1) / 2) / (n1 * n0))


def rate(k, n):
    return [round(k / n, 4), int(n)] if n else [None, 0]


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("study", type=Path)
    ap.add_argument("--out", type=Path, required=True)
    args = ap.parse_args()
    games = [json.loads(line) for line in args.study.open()]
    hb = [g for g in games if g["pilot"] == "hbot" and g["outcome"] == "won"]
    rep = {"hbot_games": len(hb)}

    # A. seat order
    wins = collections.Counter(g["winner"] for g in hb)
    rep["win_by_seat"] = [rate(wins[s], len(hb)) for s in range(4)]
    first_out = collections.Counter()
    for g in hb:
        ex = [(e["turn"], s) for s, e in enumerate(g["exits"]) if e["turn"]]
        if ex:
            first_out[min(ex)[1]] += 1
    rep["first_out_by_seat"] = [rate(first_out[s], len(hb)) for s in range(4)]

    # B. rank in a resource at the start of a round, all four alive
    res = ("life", "hand", "lands", "creatures", "power")
    rep["by_rank"], rep["auc"], rep["first_out_by_rank"] = {}, {}, {}
    for rnd in (3, 5, 7, 9):
        turn = 4 * (rnd - 1) + 1
        rows = []
        for g in hb:
            sn = next((s for s in g["snaps"] if s["turn"] == turn), None)
            if sn is None or any(sn["lost"]):
                continue
            ex = [(e["turn"], s) for s, e in enumerate(g["exits"]) if e["turn"]]
            rows.append((sn, g["winner"], min(ex)[1] if ex else -1))
        if len(rows) < 200:
            continue
        key = f"round {rnd}"
        rep["by_rank"][key], rep["auc"][key], rep["first_out_by_rank"][key] = {"tables": len(rows)}, {}, {}
        for f in res:
            w, fo, n = np.zeros(4), np.zeros(4), np.zeros(4)
            ys, xs = [], []
            for sn, winner, out in rows:
                v = sn[f]
                for s in range(4):
                    # rank 0: the most; ties share the better ranks at random by seat order, so count fractional
                    better = sum(v[q] > v[s] for q in range(4))
                    tied = sum(v[q] == v[s] for q in range(4))
                    for r in range(better, better + tied):
                        n[r] += 1 / tied
                        w[r] += (winner == s) / tied
                        fo[r] += (out == s) / tied
                    ys.append(winner == s)
                    xs.append(v[s] - np.mean([v[q] for q in range(4) if q != s]))
            rep["by_rank"][key][f] = [round(float(a / b), 3) for a, b in zip(w, n)]
            rep["first_out_by_rank"][key][f] = [round(float(a / b), 3) for a, b in zip(fo, n)]
            rep["auc"][key][f] = round(auc(ys, xs), 3)

    # C. after the first elimination: the seat whose turn it was against the other survivors
    kill = {"killer": [0, 0], "bystander": [0, 0]}
    for g in hb:
        ex = sorted((e["turn"], s, e.get("active", 0)) for s, e in enumerate(g["exits"]) if e["turn"])
        if not ex or (len(ex) > 1 and ex[1][0] == ex[0][0]):
            continue
        _, victim, active = ex[0]
        if active == victim:
            continue
        for s in range(4):
            if s == victim:
                continue
            k = kill["killer" if s == active else "bystander"]
            k[0] += g["winner"] == s
            k[1] += 1
    rep["after_first_elimination"] = {k: rate(*v) for k, v in kill.items()}

    # D. what follows a candidate's own turn (search pilots, three opponents alive):
    # life it loses until its next turn, by whether and whom it damaged
    rep["after_own_turn"] = {}
    for pilot in sorted({g["pilot"] for g in games}):
        cells = collections.defaultdict(list)
        for g in games:
            if g["pilot"] != pilot:
                continue
            me, sn = g["seat"], g["snaps"]
            by_turn = {s["turn"]: s for s in sn}
            for a in sn:
                if a["active"] != me or any(a["lost"]):
                    continue
                b, nxt = by_turn.get(a["turn"] + 1), by_turn.get(a["turn"] + 4)
                if b is None or nxt is None or nxt["active"] != me or nxt["lost"][me]:
                    continue
                opp = [q for q in range(4) if q != me]
                drop = {q: a["life"][q] - b["life"][q] for q in opp}
                lost = max(0, b["life"][me] - nxt["life"][me])
                hit = max(drop, key=drop.get)
                if drop[hit] <= 0:
                    cells["no damage"].append(lost)
                    continue
                pw = sorted(opp, key=lambda q: -a["power"][q])
                if a["power"][pw[0]] == a["power"][pw[-1]]:
                    cells["damage, boards equal"].append(lost)
                elif hit == pw[0] and a["power"][pw[0]] > a["power"][pw[1]]:
                    cells["damage to the biggest board"].append(lost)
                elif hit == pw[-1] and a["power"][pw[-1]] < a["power"][pw[1]]:
                    cells["damage to the smallest board"].append(lost)
                else:
                    cells["damage, other"].append(lost)
        rep["after_own_turn"][pilot] = {k: [round(float(np.mean(v)), 2), len(v)] for k, v in sorted(cells.items())}

    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(rep, indent=1) + "\n")
    print(json.dumps(rep, indent=1))


if __name__ == "__main__":
    main()
