from typing import Union
from fastapi import BackgroundTasks, FastAPI, Request
from fastapi.responses import JSONResponse
import uvicorn
# Correcting the import based on your initial code snippet
from langchain_community.llms import VLLM

app = FastAPI()
port = 5001

# Global variable to indicate model loading status
model="tiiuae/falcon-7b-instruct"

model_loaded = False
llm = None

async def load_model():
    global llm, model_loaded
    # Create and initialize the model instance
    llm = VLLM(model=model,
               trust_remote_code=True,  # Required for loading HF models
               max_new_tokens=50,
               temperature=0.6
              )
    model_loaded = True  # Update model loading status to True

@app.on_event("startup")
async def startup_event():
    await load_model()

@app.get("/status")
def get_status():
    if not model_loaded:
        return {"model": model, "loaded": model_loaded, "message": "Model is not loaded yet."}
    return {"model": model, "loaded": model_loaded}

# Common function to generate text based on the prompt
def generate_text_from_prompt(prompt: str) -> str:
    if not model_loaded:
        return "Model is not loaded yet."
    output = llm(prompt) # Generate text based on the prompt
    return output.replace("\n", "")

@app.get("/query")
def query_get(prompt: str) -> JSONResponse:
    if not model_loaded:
        return JSONResponse(content={"error": output}, status_code=503)

    output = generate_text_from_prompt(prompt)
    return JSONResponse(content={"text": output})

@app.post("/query")
async def query_post(request: Request) -> JSONResponse:
    if not model_loaded:
        return JSONResponse(content={"error": output}, status_code=503)

    request_dict = await request.json()
    prompt = request_dict.get("prompt", "")
    
    output = generate_text_from_prompt(prompt)
    return JSONResponse(content={"text": output})

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=port)
    
