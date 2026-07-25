FROM python:3.11-slim
RUN pip install --no-cache-dir kserve scikit-learn
COPY model.py .
CMD ["python", "model.py"]
