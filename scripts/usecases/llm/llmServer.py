from flask import Flask, request, jsonify
from flasgger import Swagger
import threading
import argparse
from datetime import datetime, timedelta
from langchain_community.llms import VLLM

app = Flask(__name__)
template = {
  "swagger": "2.0",
  "info": {
    "title": "Cloud-Barista Language Model API",
    "description": "API for generating text using a pre-trained language model.",
    "version": "0.1.0"
  },
  "basePath": "/",  # base bash for blueprint registration
  "schemes": [
    "http"
  ],
  "tags": [
    {
      "name": "System",
      "description": "Endpoints related to model information"
    },
    {
      "name": "Generation",
      "description": "Endpoints for generating text"
    }
  ],
}
swagger = Swagger(app, template=template)

model="tiiuae/falcon-7b-instruct"

parser = argparse.ArgumentParser(description='Start a Flask app with a specified model.')
parser.add_argument('--port', type=int, default=5000, help='Port number to run the Flask app on.')
parser.add_argument('--model', type=str, default=model, help='Model name to load.')
parser.add_argument('--token', type=int, default=1024, help='Set max_new_tokens.')
args = parser.parse_args()

port=args.port
model=args.model
token=args.token

# Global variable to indicate model loading status
model_loaded = False
llm = None
loading_start_time = None
loading_end_time = None
loading_total_time = None
loading_expected_time = timedelta(seconds=600)

def start_model_loading():
    thread = threading.Thread(target=load_model)
    loading_start_time = datetime.now()
    thread.start()

def load_model():
    global llm, model_loaded
    llm = VLLM(model=model,
               trust_remote_code=True,
               max_new_tokens=token,
               temperature=0.6)
    model_loaded = True
    loading_end_time = datetime.now()
    loading_total_time = loading_end_time - loading_start_time

@app.route("/status", methods=["GET"])
def get_status():
    """
    This endpoint returns the model loading status.
    ---
    tags:
      - System    
    responses:
      200:
        description: Model loading status
        schema:
          id: status_response
          properties:
            model:
              type: string
              description: The model identifier
            loaded:
              type: boolean
              description: Whether the model has been loaded
    """    
    if not model_loaded:
        elapsed_time = datetime.now() - loading_start_time
        remaining_time = max(loading_expected_time - elapsed_time, timedelta(seconds=0))
        return jsonify({
            "model": model, 
            "loaded": model_loaded, 
            "message": "Model is not loaded yet.",
            "elapsed_time": str(elapsed_time),
            "remaining_time": str(remaining_time)
        })
    return jsonify({
        "model": model, 
        "loaded": model_loaded,
        "loading_time": str(loading_total_time)
    })


@app.route("/prompt", methods=["POST"])
def prompt_post():
    """
    This is the language model prompt API.
    ---
    tags:
      - Generation
    parameters:
      - name: input
        in: body
        type: string
        required: true
        example: {"input": "What is the Multi-Cloud?"}
    responses:
      200:
        description: A successful response
        schema:
          id: output_response
          properties:
            input:
              type: string
              description: The input prompt
            output:
              type: string
              description: The generated text
            model:
              type: string
              description: The model used for generation
    """
    if not model_loaded:
        return jsonify({"error": "Model is not loaded yet."}), 503

    data = request.json
    input = data.get("input", "")
    if not input:
        return jsonify({"error": "Input text cannot be empty."}), 400

    output = llm(input)
    return jsonify({"input": input, "output": output, "model": model})

@app.route("/prompt", methods=["GET"])
def prompt_get():
    """
    This is the language model prompt API for GET requests.
    ---
    tags:
      - Generation
    parameters:
      - name: input
        in: query
        type: string
        required: true
        example: "What is the Multi-Cloud?"
    responses:
      200:
        description: A successful response
        schema:
          id: output_response
          properties:
            input:
              type: string
              description: The input prompt
            output:
              type: string
              description: The generated text
            model:
              type: string
              description: The model used for generation
    """
    if not model_loaded:
        return jsonify({"error": "Model is not loaded yet."}), 503

    input = request.args.get("input", "")
    if not input:
        return jsonify({"error": "Input text cannot be empty."}), 400

    output = llm(input)
    return jsonify({"input": input, "output": output, "model": model})

if __name__ == "__main__":
    start_model_loading()
    app.run(host="0.0.0.0", port=port, debug=False, threaded=True)
    
