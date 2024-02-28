from flask import Flask, request, jsonify
import threading
import argparse
from langchain_community.llms import VLLM

app = Flask(__name__)

model="tiiuae/falcon-7b-instruct"

parser = argparse.ArgumentParser(description='Start a Flask app with a specified model.')
parser.add_argument('--port', type=int, default=5000, help='Port number to run the Flask app on.')
parser.add_argument('--model', type=str, default=model, help='Model name to load.')
args = parser.parse_args()

port=args.port
model=args.model

# Global variable to indicate model loading status
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
    return jsonify({"input": input, "output": output, "model": model})

if __name__ == "__main__":
    start_model_loading()
    app.run(host="0.0.0.0", port=port, debug=False, threaded=True)
    
