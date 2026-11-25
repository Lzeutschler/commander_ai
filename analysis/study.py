"""Summarise a `podbench study` run: how each pilot in the candidate's seat played.

    .venv/bin/python analysis/study.py data/study/s25.jsonl --arena results/arena --out results/analysis/s25.json

Every table was played once per pilot on the same seed, so the pilots' numbers
are paired: the same decks, seats and opening draws.
"""

import argparse
import collections
import json
from pathlib import Path

import numpy as np


def own_turns(g):
    return [s for s in g["snaps"] if s["active"] == g["seat"] and not s["lost"][g["seat"]]]


def per_pilot(games):
    by = collections.defaultdict(list)
    for g in games:
        by[g["pilot"]].append(g)
    return by


def mean(xs):
    xs = list(xs)
    return round(float(np.mean(xs)), 3) if xs else None


def summarise(gs):
    n = len(gs)
    seat = lambda g: g["seat"]  # noqa: E731
    won = [g for g in gs if g["outcome"] == "won" and g["winner"] == seat(g)]
    out = {"games": n, "win_rate": round(len(won) / n, 4)}
    oc = collections.Counter(g["outcome"] for g in gs)
    out["outcomes"] = dict(oc)
    out["stalls"] = dict(collections.Counter(g["stall"] for g in gs if g["outcome"] == "stalled").most_common(8))
    out["turns_all"] = mean(g["turns"] for g in gs if g["outcome"] == "won")
    out["turns_when_won"] = mean(g["turns"] for g in won)
    # how the candidate left, and how its opponents left
    ex = [g["exits"][seat(g)] for g in gs if g["exits"][seat(g)]["turn"]]
    out["own_exit_cause"] = dict(collections.Counter(e["cause"] for e in ex))
    out["own_exit_turn"] = mean(e["turn"] for e in ex)
    opp = [(g, e) for g in gs for i, e in enumerate(g["exits"]) if i != seat(g) and e["turn"]]
    out["opp_exits_per_game"] = round(len(opp) / n, 3)
    out["opp_exit_cause"] = dict(collections.Counter(e["cause"] for _, e in opp))
    out["opp_exits_on_own_turn_per_game"] = round(sum(e.get("active", 0) == seat(g) for g, e in opp) / n, 3)
    out["win_cause"] = dict(collections.Counter(e["cause"] for g, e in opp if g in won and e.get("active", 0) == seat(g)))

    # play per own turn
    t = sum(len(own_turns(g)) for g in gs)
    P = lambda k: sum(g["play"][k] for g in gs)  # noqa: E731
    out["own_turns_per_game"] = round(t / n, 2)
    per = {k: round(P(k) / t, 3) for k in ("lands", "casts", "creatures", "permanents", "spells", "commander", "activations", "attackers")}
    out["per_own_turn"] = per
    out["mulligans_per_game"] = round(P("mulligans") / n, 3)
    out["pass_with_cast_share"] = round(P("pass_cast") / max(1, P("passes")), 3)
    out["attack_turn_share"] = round(P("attack_turns") / t, 3)
    out["attackers_of_offered"] = round(P("attackers") / max(1, P("attack_offered")), 3)
    bl = np.sum([g["play"]["by_life"] for g in gs], axis=0)
    bp = np.sum([g["play"]["by_power"] for g in gs], axis=0)
    out["attack_target_by_life"] = [round(float(v), 3) for v in bl / max(1, bl.sum())]  # highest, middle, lowest life
    out["attack_target_by_power"] = [round(float(v), 3) for v in bp / max(1, bp.sum())]
    out["attacks_at_permanents"] = round(P("at_permanent") / max(1, P("attackers")), 3)
    out["blocks_per_block_step"] = round(P("blocks") / max(1, P("block_steps")), 3)

    # damage: opponents' life lost from the start of each own turn to the next snapshot
    dealt, taken, nt = 0, 0, 0
    for g in gs:
        sn = g["snaps"]
        s0 = seat(g)
        for a, b in zip(sn, sn[1:]):
            if a["lost"][s0]:
                break
            opp_drop = sum(max(0, a["life"][q] - b["life"][q]) for q in range(4) if q != s0 and not a["lost"][q])
            if a["active"] == s0:
                dealt += opp_drop
                nt += 1
            else:
                taken += max(0, a["life"][s0] - b["life"][s0])
    out["opp_life_lost_per_own_turn"] = round(dealt / max(1, nt), 2)
    out["life_lost_per_opp_turn"] = round(taken / max(1, sum(len(g["snaps"]) for g in gs) - nt), 2)

    # the table at the candidate's k-th own turn
    curves = {}
    for k in (1, 3, 5, 7, 9, 12):
        rows = [own_turns(g)[k - 1] for g in gs if len(own_turns(g)) >= k]
        if len(rows) < 50:
            continue
        s0 = lambda r: r  # noqa: E731
        c = {"n": len(rows)}
        for f in ("life", "lands", "hand", "creatures", "power"):
            mine = [r[f][gs_seat] for r, gs_seat in ((r, r["_seat"]) for r in rows)]
            opp = [np.mean([r[f][q] for q in range(4) if q != r["_seat"] and not r["lost"][q]] or [0]) for r in rows]
            c[f] = [round(float(np.mean(mine)), 2), round(float(np.mean(opp)), 2)]
        curves[k] = c
    out["at_own_turn"] = curves
    return out


def probes(gs):
    rows = [p for g in gs for p in g.get("probes", [])]
    if not rows:
        return None
    alts = None
    out = {"probed": len(rows)}
    k = len(rows[0]["same"])
    out["agree"] = [round(np.mean([r["same"][i] for r in rows]), 3) for i in range(k)]
    by = collections.defaultdict(list)
    for r in rows:
        by[r["kind"]].append(r)
    out["agree_by_kind"] = {kind: {"n": len(rs), "agree": [round(np.mean([r["same"][i] for r in rs]), 3) for i in range(k)]}
                            for kind, rs in sorted(by.items(), key=lambda kv: -len(kv[1]))}
    # where the candidate and each alternative differ: which kinds of action
    diffs = []
    for i in range(k):
        c = collections.Counter((r["picks"][0], r["picks"][i + 1]) for r in rows if not r["same"][i])
        diffs.append({f"{a} vs {b}": n for (a, b), n in c.most_common(12)})
    out["differ_as"] = diffs
    return out


def values(gs):
    """The candidate's own-turn snapshots: the network's and the leaf's value against the outcome."""
    xs = []
    for g in gs:
        win = g["outcome"] == "won" and g["winner"] == g["seat"]
        for s in own_turns(g):
            if "net_v" in s or "leaf_v" in s:
                xs.append((s.get("net_v", 0.0), s.get("leaf_v", 0.0), win, s["turn"]))
    if not xs:
        return None
    a = np.array(xs, dtype=float)
    out = {"n": len(a), "corr_net_leaf": round(float(np.corrcoef(a[:, 0], a[:, 1])[0, 1]), 3)}
    for name, col in (("net", 0), ("leaf", 1)):
        order = np.argsort(a[:, col])
        out[f"calibration_{name}"] = [[round(float(a[p, col].mean()), 3), round(float(a[p, 2].mean()), 3)] for p in np.array_split(order, 5)]
    return out


def arena_check(gs, arena_dir):
    """The study's candidate games are the arena's: the same outcomes, game by game."""
    if arena_dir is None:
        return {}
    res = {}
    for pilot, games in per_pilot(gs).items():
        label = pilot.replace(":", "-").replace("/", "_").replace(".", "_")
        f = arena_dir / f"arena-pool-all-{label}.json"
        if not f.exists():
            continue
        oc = json.loads(f.read_text()).get("outcomes", "")
        games = sorted(games, key=lambda g: g["game"])
        mine = "".join("W" if g["outcome"] == "won" and g["winner"] == g["seat"] else ("L" if g["outcome"] in ("won", "drawn") else "U") for g in games)
        res[pilot] = {"games": len(mine), "same": sum(a == b for a, b in zip(mine, oc[: len(mine)]))}
    return res


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("study", type=Path)
    ap.add_argument("--arena", type=Path)
    ap.add_argument("--out", type=Path, required=True)
    args = ap.parse_args()
    games = [json.loads(line) for line in args.study.open()]
    for g in games:
        for s in g["snaps"]:
            s["_seat"] = g["seat"]
    report = {"arena_check": arena_check(games, args.arena), "pilots": {}}
    for pilot, gs in per_pilot(games).items():
        r = summarise(gs)
        if (p := probes(gs)) is not None:
            r["probes"] = p
        if (v := values(gs)) is not None:
            r["values"] = v
        report["pilots"][pilot] = r
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(report, indent=1) + "\n")
    print(json.dumps(report["arena_check"]))
    keys = ["win_rate", "outcomes", "turns_all", "turns_when_won", "own_turns_per_game", "per_own_turn", "pass_with_cast_share",
            "attack_turn_share", "attackers_of_offered", "attack_target_by_life", "attack_target_by_power", "blocks_per_block_step",
            "opp_life_lost_per_own_turn", "life_lost_per_opp_turn", "own_exit_cause", "own_exit_turn", "opp_exits_on_own_turn_per_game",
            "win_cause", "mulligans_per_game", "stalls"]
    for k in keys:
        print(f"\n{k}")
        for pilot, r in report["pilots"].items():
            print(f"  {pilot:28s} {r[k]}")
    print("\nat own turn k: [mine, opponents' mean]")
    for pilot, r in report["pilots"].items():
        print(f"  {pilot}")
        for k, c in r["at_own_turn"].items():
            print(f"    {k:>2}: " + "  ".join(f"{f} {c[f]}" for f in ("life", "lands", "hand", "creatures", "power")))
    for pilot, r in report["pilots"].items():
        if "probes" in r:
            print(f"\nprobes of {pilot}:", json.dumps(r["probes"], indent=1))
        if "values" in r:
            print(f"\nvalues in {pilot}'s games:", json.dumps(r["values"]))
    print("\nwrote", args.out)


if __name__ == "__main__":
    main()
