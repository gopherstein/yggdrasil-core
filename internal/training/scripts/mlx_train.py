"""Yggdrasil MLX LoRA trainer.

The daemon runs this script inside its managed Python environment with a
JSON config path as the only argument. Progress lines start with "@@ygg " and
carry one JSON object. Everything else is log output.

Stages: loading_model (download and load weights), training, exporting (write
the adapter as a GGUF LoRA that llama-server loads next to the base GGUF).
"""

import json
import os
import sys
import threading
import time
import types


def emit(**kw):
    print("@@ygg " + json.dumps(kw), flush=True)


def dir_bytes(path):
    total = 0
    for root, _, files in os.walk(path):
        for f in files:
            path = os.path.join(root, f)
            # Snapshot entries are symlinks to blobs; count each file once.
            if os.path.islink(path):
                continue
            try:
                total += os.path.getsize(path)
            except OSError:
                pass
    return total


def download(repo):
    from huggingface_hub import HfApi, snapshot_download

    patterns = ["*.json", "*.safetensors", "*.model", "*.txt", "*.tiktoken", "*.jinja"]
    total = 0
    try:
        info = HfApi().model_info(repo, files_metadata=True)
        total = sum((s.size or 0) for s in info.siblings if s.rfilename.endswith(".safetensors"))
    except Exception as exc:  # offline with a warm cache still works
        print(f"could not read repository size: {exc}", flush=True)

    hub = os.path.join(os.environ.get("HF_HOME", os.path.expanduser("~/.cache/huggingface")), "hub")
    folder = os.path.join(hub, "models--" + repo.replace("/", "--"))
    done = threading.Event()

    def watch():
        while not done.wait(1.0):
            emit(event="download", bytes=dir_bytes(folder), total=total)

    t = threading.Thread(target=watch, daemon=True)
    t.start()
    try:
        path = snapshot_download(repo, allow_patterns=patterns)
    finally:
        done.set()
        t.join()
    emit(event="download", bytes=dir_bytes(folder), total=total)
    return path


def export_gguf(adapter_dir, model_dir, architecture, out_path):
    """Write an MLX LoRA adapter as a GGUF LoRA for llama.cpp.

    MLX stores lora_a as (in, r) and lora_b as (r, out) and applies
    scale * (x @ a) @ b. llama.cpp expects PEFT layout, lora_a (r, in) and
    lora_b (out, r), scaled by alpha / r, so alpha = scale * r. Llama-family
    GGUFs permute q and k rows for rotary embeddings; the rows of lora_b get
    the same permutation.
    """
    import gguf
    import mlx.core as mx
    import numpy as np

    with open(os.path.join(model_dir, "config.json")) as f:
        hf = json.load(f)
    with open(os.path.join(adapter_dir, "adapter_config.json")) as f:
        acfg = json.load(f)
    rank = acfg["lora_parameters"]["rank"]
    scale = acfg["lora_parameters"]["scale"]
    arch = {"qwen2": gguf.MODEL_ARCH.QWEN2, "llama": gguf.MODEL_ARCH.LLAMA}[architecture]
    tmap = gguf.TensorNameMap(arch, hf["num_hidden_layers"])
    weights = {
        k: np.array(v.astype(mx.float32))
        for k, v in mx.load(os.path.join(adapter_dir, "adapters.safetensors")).items()
    }

    def permute(t, n_head):
        return t.reshape(n_head, 2, t.shape[0] // n_head // 2, *t.shape[1:]).swapaxes(1, 2).reshape(t.shape)

    tmp = out_path + ".tmp"
    writer = gguf.GGUFWriter(tmp, gguf.MODEL_ARCH_NAMES[arch])
    writer.add_type(gguf.GGUFType.ADAPTER)
    writer.add_string(gguf.Keys.Adapter.TYPE, "lora")
    writer.add_float32(gguf.Keys.Adapter.LORA_ALPHA, float(scale * rank))
    count = 0
    for key in sorted(weights):
        if not key.endswith(".lora_a"):
            continue
        stem = key[: -len(".lora_a")]
        name = tmap.get_name(stem + ".weight", try_suffixes=(".weight",))
        if name is None:
            raise ValueError(f"no GGUF tensor name for {stem}")
        a = weights[stem + ".lora_a"].T.copy()
        b = weights[stem + ".lora_b"].T.copy()
        if architecture == "llama":
            if stem.endswith("q_proj"):
                b = permute(b, hf["num_attention_heads"])
            elif stem.endswith("k_proj"):
                b = permute(b, hf.get("num_key_value_heads", hf["num_attention_heads"]))
        writer.add_tensor(name + ".lora_a", a)
        writer.add_tensor(name + ".lora_b", b)
        count += 1
    writer.write_header_to_file()
    writer.write_kv_data_to_file()
    writer.write_tensors_to_file()
    writer.close()
    os.replace(tmp, out_path)
    return count


def main():
    with open(sys.argv[1]) as f:
        cfg = json.load(f)

    emit(event="stage", stage="loading_model", detail="Downloading base weights")
    model_dir = download(cfg["repo"])

    emit(event="stage", stage="loading_model", detail="Loading the model")
    import mlx.core as mx
    from mlx_lm import load
    from mlx_lm.lora import CONFIG_DEFAULTS, train_model
    from mlx_lm.tuner.callbacks import TrainingCallback
    from mlx_lm.tuner.datasets import load_dataset

    h = cfg["hyper"]
    args = dict(CONFIG_DEFAULTS)
    args.update(
        model=model_dir,
        train=True,
        data=cfg["data_dir"],
        fine_tune_type="lora",
        num_layers=h["layers"],
        batch_size=h["batch_size"],
        iters=h["iters"],
        val_batches=25,
        learning_rate=h["learning_rate"],
        steps_per_report=max(1, h["iters"] // 50),
        steps_per_eval=max(5, h["iters"] // 5),
        adapter_path=cfg["adapter_dir"],
        save_every=max(h["iters"], 1),
        max_seq_length=h["max_seq_length"],
        grad_checkpoint=bool(h.get("grad_checkpoint")),
        lora_parameters={"rank": h["rank"], "dropout": 0.0, "scale": h["scale"]},
        mask_prompt=True,
        seed=h.get("seed", 0),
    )
    args = types.SimpleNamespace(**args)

    model, tokenizer = load(model_dir)
    train_set, valid_set, _ = load_dataset(args, tokenizer)

    class Reporter(TrainingCallback):
        def __init__(self):
            self.train_loss = None
            self.val_loss = None

        def on_train_loss_report(self, info):
            self.train_loss = info.get("train_loss")
            emit(
                event="progress",
                iter=info.get("iteration"),
                iters=h["iters"],
                train_loss=info.get("train_loss"),
                tokens_per_sec=info.get("tokens_per_second"),
                it_per_sec=info.get("iterations_per_second"),
                peak_memory_gb=info.get("peak_memory"),
            )

        def on_val_loss_report(self, info):
            self.val_loss = info.get("val_loss")
            emit(event="val", iter=info.get("iteration"), val_loss=info.get("val_loss"))

    reporter = Reporter()
    emit(event="stage", stage="training", detail="Training")
    started = time.time()
    train_model(args, model, train_set, valid_set, reporter)
    mx.clear_cache()

    emit(event="stage", stage="exporting", detail="Writing the adapter")
    tensors = export_gguf(cfg["adapter_dir"], model_dir, cfg["architecture"], cfg["adapter_out"])
    emit(
        event="done",
        adapter=cfg["adapter_out"],
        tensors=tensors,
        train_loss=reporter.train_loss,
        val_loss=reporter.val_loss,
        seconds=round(time.time() - started, 1),
    )


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:  # the daemon shows this message to the user
        emit(event="error", message=f"{type(exc).__name__}: {exc}")
        raise
