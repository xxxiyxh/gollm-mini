# server.py 关键改动
from functools import lru_cache
from fastapi import FastAPI
from pydantic import BaseModel
from transformers import AutoTokenizer, AutoModelForCausalLM
import torch

app = FastAPI()

class ChatReq(BaseModel):
    input: str
    model: str | None = None   # 允许前端指定模型；留空则用默认

@lru_cache                         # 多次请求同一个模型时复用
def load(model_id: str):
    tok = AutoTokenizer.from_pretrained(model_id)
    mod = AutoModelForCausalLM.from_pretrained(model_id, torch_dtype="auto")
    return tok, mod

@app.post("/generate")
def generate(req: ChatReq):
    model_id = req.model or "TinyLlama/TinyLlama-1.1B-Chat-v1.0"
    tokenizer, model = load(model_id)

    inputs = tokenizer(req.input, return_tensors="pt")
    eos = tokenizer.convert_tokens_to_ids("</s>")
    outputs = model.generate(
        **inputs,
        max_new_tokens=1024,
        eos_token_id=eos, 
        do_sample=False
    )
    text = tokenizer.decode(outputs[0], skip_special_tokens=False)
    return {"text": text}
