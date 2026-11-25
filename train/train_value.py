"""Train a value network on positions recorded by `podbench record`.

    .venv/bin/python train/train_value.py data/v0 models/v0.caiv
    .venv/bin/python train/train_value.py data/v3 data/v4 models/v4.caiv --mark data/v4

A data directory holds shards of feature vectors with the outcome of the game
for the seat that saw the position, and meta.json. The network is a small fully
connected one from the features to the probability that this seat wins. Games,
not positions, are split into a training and a validation part, so no game
feeds both.

Several directories are trained on together. The seats of one may have been
played by a stronger pilot than those of another, and the same position is
then worth more to them. --mark names the directories whose records are the
ones the network is for: the network gets one more input that is 1 on their
records and 0 on the rest, validation is reported on them alone, and the
exported network has that input fixed at 1.

The output is the CAIV0001 file internal/learn/valnet.go reads: the input
standardisation folded into the first layer, weights as little-endian float32.
"""

import argparse
import json
import struct
import time
from pathlib import Path

import numpy as np
import torch
from torch import nn


def load_all(dirs, marked):
    """All directories as one array: features (plus the mark, if any directory
    is marked), outcomes, the game of each record numbered apart across
    directories, and per record whether its directory is marked."""
    metas = [json.loads((d / "meta.json").read_text()) for d in dirs]
    width = metas[0]["width"]
    for d, meta in zip(dirs, metas):
        if meta["width"] != width:
            raise SystemExit(f"{d}: {meta['width']} features, {dirs[0]} has {width}")
    # Records are counted from the files: a run the engine brought down has
    # no counts in its meta.json, and its last record may be torn.
    counts = []
    for d, meta in zip(dirs, metas):
        cols = meta.get("meta_cols", 4)
        counts.append([min((d / f"{s}.y.u8").stat().st_size, (d / f"{s}.x.f32").stat().st_size // (4 * width),
                           (d / f"{s}.m.i32").stat().st_size // (4 * cols)) for s in meta["shards"]])
    total = sum(map(sum, counts))
    x = np.zeros((total, width + bool(marked)), dtype=np.float32)
    y = np.zeros(total, dtype=np.float32)
    game = np.zeros(total, dtype=np.int64)
    mark = np.zeros(total, dtype=bool)
    at, base = 0, 0
    for d, meta, ns in zip(dirs, metas, counts):
        cols, first = meta.get("meta_cols", 4), at
        for shard, n in zip(meta["shards"], ns):
            if n == 0:
                continue
            x[at : at + n, :width] = np.fromfile(d / f"{shard}.x.f32", dtype="<f4", count=n * width).reshape(n, width)
            y[at : at + n] = np.fromfile(d / f"{shard}.y.u8", dtype=np.uint8, count=n)
            game[at : at + n] = np.fromfile(d / f"{shard}.m.i32", dtype="<i4", count=n * cols).reshape(n, cols)[:, 0] + base
            at += n
        if d in marked:
            x[first:at, width] = 1
            mark[first:at] = True
        if not meta.get("complete", True):
            meta["records"], meta["finished"] = at - first, int(len(np.unique(game[first:at])))
            print(f"{d}: an unfinished run, {at - first} records of {meta['finished']} games")
        base = int(game[first:at].max()) + 1 if at > first else base
    return metas, x, y, game, mark


def auc(y: np.ndarray, p: np.ndarray) -> float:
    """Probability that a won position is scored above a lost one."""
    order = np.argsort(p, kind="stable")
    ranks = np.empty(len(p))
    ranks[order] = np.arange(1, len(p) + 1)
    pos = y > 0.5
    n_pos, n_neg = pos.sum(), (~pos).sum()
    if n_pos == 0 or n_neg == 0:
        return float("nan")
    return float((ranks[pos].sum() - n_pos * (n_pos + 1) / 2) / (n_pos * n_neg))


def logloss(y: np.ndarray, p: np.ndarray) -> float:
    p = np.clip(p, 1e-6, 1 - 1e-6)
    return float(-(y * np.log(p) + (1 - y) * np.log(1 - p)).mean())


class Net(nn.Module):
    """A fully connected network plus a linear skip from the inputs to the output.

    What is linear in the inputs (the engine's own leaf logit above all) goes
    through the skip; the layers learn what it misses.
    """

    def __init__(self, width, hidden, dropout, skip):
        super().__init__()
        sizes = [width, *hidden]
        layers = []
        for a, b in zip(sizes, sizes[1:]):
            layers += [nn.Linear(a, b), nn.ReLU(), nn.Dropout(dropout)]
        self.mlp = nn.Sequential(*layers, nn.Linear(sizes[-1], 1))
        self.skip = nn.Linear(width, 1, bias=False) if skip else None
        if skip:
            nn.init.zeros_(self.skip.weight)

    def forward(self, x):
        out = self.mlp(x)
        return out + self.skip(x) if self.skip is not None else out


def export(path: Path, model: Net, mean: np.ndarray, scale: np.ndarray, hidden, meta: dict, fixed: bool):
    """Write the network with the standardisation folded in.

    With fixed, the model's last input is the mark, which the reader does not
    have: it is 1 for every position the network is asked about, so what it
    adds goes into the biases as well.

    The model reads xn = (x - mean) * scale. The reader computes on x * scale,
    so the constant share of the mean moves into biases: the first layer's
    b0' = b0 - W0 @ (mean * scale), and with a skip the last layer's
    b' = b - skip @ (mean * scale). W0 is stored input-major so a zero input
    costs nothing.
    """
    linears = [m for m in model.mlp if isinstance(m, nn.Linear)]
    shift = mean.astype(np.float64) * scale
    if fixed:
        shift[-1] -= scale[-1]  # the last input stands at 1: (1 - mean) * scale
    w0 = linears[0].weight.detach().cpu().numpy().astype(np.float64)
    b0 = linears[0].bias.detach().cpu().numpy().astype(np.float64) - w0 @ shift
    skip = model.skip.weight.detach().cpu().numpy().astype(np.float64)[0] if model.skip is not None else None
    last_shift = skip @ shift if skip is not None else 0.0
    if fixed:
        mean, scale, w0 = mean[:-1], scale[:-1], w0[:, :-1]
        skip = skip[:-1] if skip is not None else None
    header = json.dumps({"width": int(len(mean)), "hidden": list(hidden), "skip": skip is not None, "meta": meta}).encode()
    with open(path, "wb") as f:
        f.write(b"CAIV0001")
        f.write(struct.pack("<Q", len(header)))
        f.write(header)
        f.write(scale.astype("<f4").tobytes())
        if skip is not None:
            f.write(skip.astype("<f4").tobytes())
        f.write(np.ascontiguousarray(w0.T).astype("<f4").tobytes())
        f.write(b0.astype("<f4").tobytes())
        for i, lin in enumerate(linears[1:], start=1):
            bias = lin.bias.detach().cpu().numpy().astype(np.float64)
            if skip is not None and i == len(linears) - 1:
                bias = bias - last_shift
            f.write(lin.weight.detach().cpu().numpy().astype("<f4").tobytes())
            f.write(bias.astype("<f4").tobytes())


def read_back(path: Path, x: np.ndarray) -> np.ndarray:
    """What the file at path answers for the recorded inputs x, computed the way the Go reader does."""
    raw = path.read_bytes()
    n = struct.unpack("<Q", raw[8:16])[0]
    head, at = json.loads(raw[16 : 16 + n]), 16 + n

    def take(*shape):
        nonlocal at
        a = np.frombuffer(raw, dtype="<f4", count=int(np.prod(shape)), offset=at).astype(np.float64).reshape(shape)
        at += 4 * a.size
        return a

    width = head["width"]
    z = x[:, :width].astype(np.float64) * take(width)
    out = z @ take(width) if head["skip"] else 0.0
    sizes = [*head["hidden"], 1]
    a = z @ take(width, sizes[0]) + take(sizes[0])
    for m, k in zip(sizes, sizes[1:]):
        a = np.maximum(a, 0) @ take(k, m).T + take(k)
    return 1 / (1 + np.exp(-(a[:, 0] + out)))


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("data", type=Path, nargs="+")
    ap.add_argument("out", type=Path)
    ap.add_argument("--mark", type=Path, nargs="+", default=[], help="the directories among data whose records the network is for")
    ap.add_argument("--hidden", type=int, nargs="+", default=[256, 128])
    ap.add_argument("--epochs", type=int, default=12)
    ap.add_argument("--batch", type=int, default=2048)
    ap.add_argument("--lr", type=float, default=1e-3)
    ap.add_argument("--weight-decay", type=float, default=1e-4)
    ap.add_argument("--dropout", type=float, default=0.1)
    ap.add_argument("--no-skip", action="store_true", help="leave out the linear skip from the inputs to the output")
    ap.add_argument("--val", type=float, default=0.1, help="share of games held out for validation")
    ap.add_argument("--seed", type=int, default=1)
    ap.add_argument("--probe", type=Path, help="also write a few validation inputs and outputs here, for the Go reader's test")
    ap.add_argument("--probe-count", type=int, default=64)
    args = ap.parse_args()

    torch.manual_seed(args.seed)
    rng = np.random.default_rng(args.seed)
    if not set(args.mark) <= set(args.data):
        raise SystemExit("--mark names a directory that is not among the data")
    if args.mark and args.probe:
        raise SystemExit("--probe writes the inputs as recorded; it does not go with --mark")
    metas, x, y, game, marked = load_all(args.data, set(args.mark))
    games = np.unique(game)
    rng.shuffle(games)
    is_val = np.isin(game, games[: max(1, int(len(games) * args.val))])
    if args.mark:
        is_val &= marked  # the unmarked records all train
    train, val = np.flatnonzero(~is_val), np.flatnonzero(is_val)
    yt, yv = y[train], y[val]
    print(f"{len(x)} positions from {len(games)} games, {x.shape[1]} inputs; "
          f"train {len(train)}, validation {len(val)}; wins {y.mean():.3f}"
          + (f", {y[marked].mean():.3f} on the {int(marked.sum())} marked" if args.mark else ""))
    check_x = x[val[: max(args.probe_count, 256)]].astype(np.float64)  # as recorded, to check the export against

    # Standardise in place, a slice at a time: the array is most of the memory.
    step = 1 << 16
    tot, sq = np.zeros(x.shape[1]), np.zeros(x.shape[1])
    for i in range(0, len(x), step):
        part = x[i : i + step][~is_val[i : i + step]].astype(np.float64)
        tot += part.sum(axis=0)
        sq += (part * part).sum(axis=0)
    mean = tot / len(train)
    std = np.sqrt(np.maximum(sq / len(train) - mean * mean, 0))
    scale = np.where(std > 1e-6, 1.0 / np.maximum(std, 1e-6), 0.0)  # a constant input carries nothing
    mean, scale = mean.astype(np.float32), scale.astype(np.float32)
    for i in range(0, len(x), step):
        x[i : i + step] -= mean
        x[i : i + step] *= scale
    device = "mps" if torch.backends.mps.is_available() else "cuda" if torch.cuda.is_available() else "cpu"
    x_t, yt_t = torch.from_numpy(x), torch.from_numpy(yt)
    train_t = torch.from_numpy(train)
    xv_t = x_t[torch.from_numpy(val)].to(device)
    model = Net(x.shape[1], args.hidden, args.dropout, not args.no_skip).to(device)
    opt = torch.optim.AdamW(model.parameters(), lr=args.lr, weight_decay=args.weight_decay)
    steps = args.epochs * ((len(train) + args.batch - 1) // args.batch)
    sched = torch.optim.lr_scheduler.OneCycleLR(opt, max_lr=args.lr, total_steps=steps)
    loss_fn = nn.BCEWithLogitsLoss()

    def predict(t):
        model.eval()
        with torch.no_grad():
            return torch.sigmoid(model(t).squeeze(1)).cpu().numpy()

    base = float(yt.mean())
    print(f"device {device}; a constant guess of {base:.3f} scores log loss {logloss(yv, np.full(len(yv), base)):.4f} on validation")
    best, best_state, t0 = float("inf"), None, time.time()
    for epoch in range(1, args.epochs + 1):
        model.train()
        perm = torch.randperm(len(train))
        total = 0.0
        for i in range(0, len(perm), args.batch):
            idx = perm[i : i + args.batch]
            xb, yb = x_t[train_t[idx]].to(device), yt_t[idx].to(device)
            opt.zero_grad()
            loss = loss_fn(model(xb).squeeze(1), yb)
            loss.backward()
            opt.step()
            sched.step()
            total += loss.item() * len(idx)
        pv = predict(xv_t)
        ll = logloss(yv, pv)
        print(f"epoch {epoch:2d}  train loss {total / len(perm):.4f}  validation log loss {ll:.4f}  AUC {auc(yv, pv):.4f}  ({time.time() - t0:.0f}s)")
        if ll < best:
            best, best_state = ll, {k: v.detach().clone() for k, v in model.state_dict().items()}
    model.load_state_dict(best_state)
    pv = predict(xv_t)
    result = {"validation_log_loss": round(logloss(yv, pv), 4), "validation_auc": round(auc(yv, pv), 4),
              "constant_log_loss": round(logloss(yv, np.full(len(yv), base)), 4),
              "positions": int(len(x)), "games": int(len(games))}
    print("best:", result)

    args.out.parent.mkdir(parents=True, exist_ok=True)
    keys = ("split", "pilot", "games", "finished", "every", "records")
    export(args.out, model, mean, scale, args.hidden,
           {"data": [{"dir": str(d), "marked": d in args.mark, **{k: m[k] for k in keys}} for d, m in zip(args.data, metas)],
            "engine_rev": metas[-1]["provenance"]["engine_rev"], "pool_digest": metas[-1]["pool"]["digest"],
            "hidden": args.hidden, "skip": not args.no_skip, "epochs": args.epochs, **result},
           fixed=bool(args.mark))
    off = float(np.abs(read_back(args.out, check_x) - pv[: len(check_x)]).max())
    if off > 1e-4:
        raise SystemExit(f"{args.out}: the file answers up to {off:.2g} away from the trained network")
    print(f"wrote {args.out} (read back: within {off:.1g} of the trained network)")
    if args.probe:
        k = args.probe_count
        probe = {"x": check_x[:k].tolist(), "p": pv[:k].astype(np.float64).tolist()}
        args.probe.write_text(json.dumps(probe))
        print("wrote", args.probe)


if __name__ == "__main__":
    main()
