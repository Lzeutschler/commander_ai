"""Train a value network on positions recorded by `podbench record`.

    .venv/bin/python train/train_value.py data/v0 models/v0.caiv

The data directory holds shards of feature vectors with the outcome of the game
for the seat that saw the position, and meta.json. The network is a small fully
connected one from the features to the probability that this seat wins. Games,
not positions, are split into a training and a validation part, so no game
feeds both.

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


def load(data_dir: Path):
    meta = json.loads((data_dir / "meta.json").read_text())
    width = meta["width"]
    xs, ys, ms = [], [], []
    for shard in meta["shards"]:
        y = np.fromfile(data_dir / f"{shard}.y.u8", dtype=np.uint8)
        if len(y) == 0:
            continue
        x = np.fromfile(data_dir / f"{shard}.x.f32", dtype="<f4").reshape(len(y), width)
        m = np.fromfile(data_dir / f"{shard}.m.i32", dtype="<i4").reshape(len(y), 4)
        xs.append(x), ys.append(y), ms.append(m)
    return meta, np.concatenate(xs), np.concatenate(ys).astype(np.float32), np.concatenate(ms)


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


def export(path: Path, model: Net, mean: np.ndarray, scale: np.ndarray, hidden, meta: dict):
    """Write the network with the standardisation folded in.

    The model reads xn = (x - mean) * scale. The reader computes on x * scale,
    so the constant share of the mean moves into biases: the first layer's
    b0' = b0 - W0 @ (mean * scale), and with a skip the last layer's
    b' = b - skip @ (mean * scale). W0 is stored input-major so a zero input
    costs nothing.
    """
    linears = [m for m in model.mlp if isinstance(m, nn.Linear)]
    shift = mean.astype(np.float64) * scale
    w0 = linears[0].weight.detach().cpu().numpy().astype(np.float64)
    b0 = linears[0].bias.detach().cpu().numpy().astype(np.float64) - w0 @ shift
    skip = model.skip.weight.detach().cpu().numpy().astype(np.float64)[0] if model.skip is not None else None
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
                bias = bias - skip @ shift
            f.write(lin.weight.detach().cpu().numpy().astype("<f4").tobytes())
            f.write(bias.astype("<f4").tobytes())


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("data", type=Path)
    ap.add_argument("out", type=Path)
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
    meta, x, y, m = load(args.data)
    games = np.unique(m[:, 0])
    rng.shuffle(games)
    val_games = set(games[: max(1, int(len(games) * args.val))].tolist())
    is_val = np.fromiter((g in val_games for g in m[:, 0]), dtype=bool, count=len(m))
    xt, yt, xv, yv = x[~is_val], y[~is_val], x[is_val], y[is_val]
    print(f"{len(x)} positions from {len(games)} games, {x.shape[1]} features; "
          f"train {len(xt)}, validation {len(xv)} ({len(val_games)} games); wins {y.mean():.3f}")

    mean = xt.mean(axis=0)
    std = xt.std(axis=0)
    scale = np.where(std > 1e-6, 1.0 / np.maximum(std, 1e-6), 0.0)  # a constant input carries nothing
    device = "mps" if torch.backends.mps.is_available() else "cuda" if torch.cuda.is_available() else "cpu"

    def norm(a):
        return torch.from_numpy(((a - mean) * scale).astype(np.float32))

    xt_t, yt_t, xv_t = norm(xt), torch.from_numpy(yt), norm(xv).to(device)
    model = Net(x.shape[1], args.hidden, args.dropout, not args.no_skip).to(device)
    opt = torch.optim.AdamW(model.parameters(), lr=args.lr, weight_decay=args.weight_decay)
    steps = args.epochs * ((len(xt_t) + args.batch - 1) // args.batch)
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
        perm = torch.randperm(len(xt_t))
        total = 0.0
        for i in range(0, len(perm), args.batch):
            idx = perm[i : i + args.batch]
            xb, yb = xt_t[idx].to(device), yt_t[idx].to(device)
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
    export(args.out, model, mean, scale, args.hidden,
           {"data": {k: meta[k] for k in ("split", "pilot", "games", "finished", "every", "records")},
            "engine_rev": meta["provenance"]["engine_rev"], "pool_digest": meta["pool"]["digest"],
            "hidden": args.hidden, "skip": not args.no_skip, "epochs": args.epochs, **result})
    print("wrote", args.out)
    if args.probe:
        k = min(args.probe_count, len(xv))
        probe = {"x": xv[:k].astype(np.float64).tolist(), "p": pv[:k].astype(np.float64).tolist()}
        args.probe.write_text(json.dumps(probe))
        print("wrote", args.probe)


if __name__ == "__main__":
    main()
