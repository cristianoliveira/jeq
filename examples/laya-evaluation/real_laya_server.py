"""Start the real pinned Laya Agent on loopback for the opt-in acceptance test."""

import os
import time

import torch
import uvicorn
from laya import Agent, Router
from laya.serve import create_app

model_path = os.environ["LAYA_MODEL_PATH"]
port = int(os.environ["LAYA_PORT"])
threads = int(os.environ.get("LAYA_THREADS", "4"))

torch.set_num_threads(threads)
started = time.perf_counter()
agent = Agent(model_path, device="cpu")
router = Router(device="cpu", max_loaded=1)
router.attach("english", agent)
print(f"real Laya model loaded in {time.perf_counter() - started:.3f}s", flush=True)
uvicorn.run(create_app(router=router), host="127.0.0.1", port=port, log_level="warning")
