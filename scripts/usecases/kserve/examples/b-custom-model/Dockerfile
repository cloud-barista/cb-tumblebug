FROM python:3.11-slim
RUN pip install --no-cache-dir kserve
COPY model.py .
CMD ["python", "model.py"]
