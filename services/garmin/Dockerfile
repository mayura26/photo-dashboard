FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY poller.py .
ENV GARMIN_TOKEN_DIR=/data/tokens
VOLUME ["/data/tokens"]
CMD ["python", "-u", "poller.py"]
