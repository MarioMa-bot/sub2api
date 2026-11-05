# Sub2API 智能脱敏旁路（Presidio Analyzer，CPU）
# Smart-detection sidecar for Sub2API privacy masking (Presidio Analyzer, CPU only)
FROM python:3.12-slim

ARG PIP_INDEX_URL=https://pypi.org/simple
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_INDEX_URL=${PIP_INDEX_URL}

WORKDIR /app
COPY requirements.txt .
RUN pip install -r requirements.txt

COPY app.py .
RUN useradd --system --uid 10001 ner
USER ner

EXPOSE 5002
HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
    CMD python -c "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://127.0.0.1:5002/health', timeout=3).status == 200 else 1)"
CMD ["python", "app.py"]
