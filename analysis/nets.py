"""Look inside the value networks: how well each predicts, on which positions,
and what it looks at.

    .venv/bin/python analysis/nets.py --nets models/v3.caiv models/v4b.caiv models/v5.caiv \
        --data data/test/hbot-train data/test/v5-train data/test/v5-eval --out results/analysis/nets.json

Every data directory is one recorded set (podbench record). The sets should be
recorded fresh for this, with seeds no network trained on. For each network and
set it reports log loss, AUC and calibration, overall and by game phase, next to
the engine's hand-made leaf (feature column leafOff, the probability the leaf
gives the seat). It then measures what each network leans on: the columns of a
feature group are shuffled across positions, and the rise in log loss is that
group's share.
"""

import argparse
import json
import sys
from pathlib import Path

import numpy as np

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "train"))
import train_value as tv  # noqa: E402

# The feature layout of internal/learn/features.go.
DENSE_BASE = 92  # feat.DenseWidth: see groups()
SEAT_W = 16  # one pod block per seat in turn order from the viewer
GLOBAL_W = 3
DENSE_W = DENSE_BASE + 4 * SEAT_W + GLOBAL_W
RAW_W = 120
GROUP_W = 1 + 2 * RAW_W
GROUP_NAMES = ["battlefield (self)", "battlefield (next seat)", "battlefield (seat after)", "battlefield (seat before)",
               "hand (self)", "stack", "command zone"]
LEAF_OFF = DENSE_W + len(GROUP_NAMES) * GROUP_W
WIDTH = LEAF_OFF + 8


def groups():
    """Name -> column indices of each feature group.

    The base dense block (feat/state.go): turn, step, phase, active and
    priority flags, stack size and mana pool in columns 0-27; then one block of
    ten per seat in turn order from the viewer at 28 + 10*k: life, hand,
    library, graveyard, permanents, creatures, power, toughness, untapped
    permanents, untapped lands; then the commander blocks up to column 91.
    """
    rel = ["self", "next seat", "seat after", "seat before"]
    g = {"turn, step, priority, stack, mana pool": range(0, 28)}
    for i in range(4):
        g[f"seat counts ({rel[i]}): life, hand, library, board, power"] = range(28 + 10 * i, 38 + 10 * i)
    g["commander blocks"] = range(68, DENSE_BASE)
    for i in range(4):
        g[f"pod block ({rel[i]}): poison, commander damage, life gained/lost, monarch"] = range(DENSE_BASE + i * SEAT_W, DENSE_BASE + (i + 1) * SEAT_W)
    g["globals (round, day/night)"] = range(DENSE_BASE + 4 * SEAT_W, DENSE_W)
    for i, name in enumerate(GROUP_NAMES):
        g[f"cards: {name}"] = range(DENSE_W + i * GROUP_W, DENSE_W + (i + 1) * GROUP_W)
    g["hand-made leaf (self)"] = range(LEAF_OFF, LEAF_OFF + 2)
    g["hand-made leaf (opponents)"] = range(LEAF_OFF + 2, WIDTH)
    return {k: np.array(list(v)) for k, v in g.items()}


def load(d: Path, limit: int, rng):
    meta = json.loads((d / "meta.json").read_text())
    _, x, y, game, _ = tv.load_all([d], set())
    cols = meta.get("meta_cols", 4)
    turn = np.concatenate([np.fromfile(d / f"{s}.m.i32", dtype="<i4").reshape(-1, cols)[:, 2]
                           for s in meta["shards"] if (d / f"{s}.y.u8").stat().st_size])
    if len(y) > limit:
        keep = np.sort(rng.choice(len(y), limit, replace=False))
        x, y, game, turn = x[keep], y[keep], game[keep], turn[keep]
    return meta, x.astype(np.float64), y, game, turn


def calibration(y, p, bins=10):
    """Per decile of the prediction: mean prediction and observed win rate."""
    order = np.argsort(p)
    out = []
    for part in np.array_split(order, bins):
        out.append([round(float(p[part].mean()), 3), round(float(y[part].mean()), 3), int(len(part))])
    return out


def scores(y, p):
    return {"log_loss": round(tv.logloss(y, p), 4), "auc": round(tv.auc(y, p), 4), "mean_p": round(float(p.mean()), 3),
            "mean_y": round(float(y.mean()), 3)}


PHASES = [(1, 8), (9, 16), (17, 28), (29, 10**6)]  # turns counted for all seats: 4 to a round


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--nets", type=Path, nargs="+", required=True)
    ap.add_argument("--data", type=Path, nargs="+", required=True)
    ap.add_argument("--limit", type=int, default=60000, help="positions per set at most")
    ap.add_argument("--importance", type=Path, nargs="*", help="networks to measure feature groups for (default: all)")
    ap.add_argument("--out", type=Path, required=True)
    args = ap.parse_args()
    rng = np.random.default_rng(7)
    report = {"sets": {}, "importance": {}, "agreement": {}}
    imp_nets = set(map(str, args.importance if args.importance is not None else args.nets))
    for d in args.data:
        meta, x, y, game, turn = load(d, args.limit, rng)
        leaf = np.clip(x[:, LEAF_OFF], 1e-6, 1 - 1e-6)
        preds = {"hand-made leaf": leaf}
        for n in args.nets:
            preds[n.stem] = tv.read_back(n, x)
        entry = {"pilot": meta["pilot"], "split": meta["split"], "positions": int(len(y)), "games": int(len(np.unique(game))),
                 "by": {}, "phases": {}, "calibration": {}}
        for name, p in preds.items():
            entry["by"][name] = scores(y, p)
            entry["calibration"][name] = calibration(y, p)
            ph = {}
            for lo, hi in PHASES:
                m = (turn >= lo) & (turn <= hi)
                if m.sum() > 200:
                    ph[f"{lo}-{hi if hi < 10**6 else ''}"] = {**scores(y[m], p[m]), "n": int(m.sum())}
            entry["phases"][name] = ph
        names = list(preds)
        entry["correlation"] = {a: {b: round(float(np.corrcoef(preds[a], preds[b])[0, 1]), 3) for b in names} for a in names}
        entry["spread"] = {name: round(float(np.std(p)), 3) for name, p in preds.items()}
        report["sets"][d.name] = entry
        print(f"\n{d} ({meta['pilot']}, {meta['split']}): {len(y)} positions")
        for name in names:
            s = entry["by"][name]
            print(f"  {name:16s} log loss {s['log_loss']:.4f}  AUC {s['auc']:.4f}  mean {s['mean_p']:.3f} (wins {s['mean_y']:.3f})  spread {entry['spread'][name]}")

        # Feature groups: shuffle a group's columns together across positions.
        for n in args.nets:
            if str(n) not in imp_nets:
                continue
            base = tv.logloss(y, preds[n.stem])
            imp = {}
            for gname, cols in groups().items():
                xs = x.copy()
                xs[:, cols] = xs[rng.permutation(len(xs))][:, cols]
                imp[gname] = round(tv.logloss(y, tv.read_back(n, xs)) - base, 4)
            report["importance"].setdefault(n.stem, {})[d.name] = imp
            top = sorted(imp.items(), key=lambda kv: -kv[1])[:6]
            print(f"  {n.stem} leans on: " + "; ".join(f"{k} +{v:.4f}" for k, v in top))
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(report, indent=1) + "\n")
    print("\nwrote", args.out)


if __name__ == "__main__":
    main()
