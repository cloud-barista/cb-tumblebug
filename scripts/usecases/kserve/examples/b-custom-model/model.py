import kserve
from typing import Dict
import numpy as np
from sklearn.datasets import load_digits
from sklearn.neural_network import MLPClassifier


class DigitModel(kserve.Model):
    """Custom pre/post-processing around the classifier — the reason to build a custom image."""

    def __init__(self, name: str):
        super().__init__(name)
        self.load()

    def load(self):
        # Load your own artifact here; the demo trains a small MLP at startup
        X, y = load_digits(return_X_y=True)
        self.model = MLPClassifier(hidden_layer_sizes=(64,), max_iter=300, random_state=42).fit(X, y)
        self.ready = True

    def predict(self, payload: Dict, headers=None) -> Dict:
        # Preprocessing: accept raw 8x8 images or flat vectors
        imgs = np.array(payload["instances"], dtype=float).reshape(len(payload["instances"]), -1)
        probs = self.model.predict_proba(imgs)
        # Postprocessing: return digit AND confidence (standard runtimes return the class only)
        return {"predictions": [
            {"digit": int(p.argmax()), "confidence": round(float(p.max()), 3)} for p in probs
        ]}


if __name__ == "__main__":
    kserve.ModelServer().start([DigitModel("custom-model")])
