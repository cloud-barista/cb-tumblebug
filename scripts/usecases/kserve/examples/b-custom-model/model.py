import kserve
from typing import Dict


class CustomModel(kserve.Model):
    """Wrap arbitrary inference logic; KServe provides the API server, health checks, and protocol."""

    def __init__(self, name: str):
        super().__init__(name)
        self.load()

    def load(self):
        # Load your model artifact here (e.g., joblib.load("/mnt/models/model.joblib"))
        self.model = lambda x: sum(x)
        self.ready = True

    def predict(self, payload: Dict, headers=None) -> Dict:
        # Custom pre/post-processing goes here
        instances = payload["instances"]
        return {"predictions": [self.model(x) for x in instances]}


if __name__ == "__main__":
    kserve.ModelServer().start([CustomModel("custom-model")])
