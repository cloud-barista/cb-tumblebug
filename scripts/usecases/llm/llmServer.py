from flask import Flask, request, jsonify
import threading
from langchain_community.llms import VLLM


app = Flask(__name__)
port = 5001

# Global variable to indicate model loading status
model="tiiuae/falcon-7b-instruct"

model_loaded = False
llm = None

def start_model_loading():
    thread = threading.Thread(target=load_model)
    thread.start()

def load_model():
    global llm, model_loaded
    llm = VLLM(model=model,
               trust_remote_code=True,
               max_new_tokens=50,
               temperature=0.6)
    model_loaded = True

@app.route("/status", methods=["GET"])
def get_status():
    if not model_loaded:
        return jsonify({"model": model, "loaded": model_loaded, "message": "Model is not loaded yet."})
    return jsonify({"model": model, "loaded": model_loaded})


@app.route("/prompt", methods=["GET", "POST"])
def prompt():
    if not model_loaded:
        return jsonify({"error": "Model is not loaded yet."}), 503

    input = ""
    if request.method == "POST":
        data = request.json
        input = data.get("input", "")
    elif request.method == "GET":
        input = request.args.get("input", "")

    output = llm(input)
    return jsonify({"output": output, "model": model})

if __name__ == "__main__":
    start_model_loading()
    app.run(host="0.0.0.0", port=port, debug=False, threaded=True)
    
