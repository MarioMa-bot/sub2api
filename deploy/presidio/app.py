"""
Sub2API 智能脱敏旁路服务（Presidio Analyzer）
Smart-detection sidecar for Sub2API privacy masking, backed by Microsoft Presidio Analyzer.

API:
  GET  /health   -> {"status": "ok", "languages": [...]}
  POST /analyze  {"text": "...", "entities": ["PERSON", ...]?, "min_score": 0.6?, "language": "zh"?}
              -> [{"start": 0, "end": 2, "kind": "PERSON", "score": 0.85}, ...]

start/end 是 Unicode 码点偏移（Python str 下标），左闭右开。
Offsets are Unicode code-point indices (Python str indices), end exclusive.
服务只返回位置与类型，不记录、不保存任何文本。
Only spans and kinds are returned; request text is never logged or stored.
"""

import json
import logging
import os
import re
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from presidio_analyzer import AnalyzerEngine, RecognizerRegistry
from presidio_analyzer.nlp_engine import NlpEngineProvider

LOG = logging.getLogger("privacy-ner")

HOST = os.getenv("PRIVACY_NER_HOST", "0.0.0.0")
PORT = int(os.getenv("PRIVACY_NER_PORT", "5002"))
LANGUAGES = [x.strip() for x in os.getenv("PRIVACY_NER_LANGUAGES", "en,zh").split(",") if x.strip()]
MIN_SCORE = float(os.getenv("PRIVACY_NER_MIN_SCORE", "0.5"))
MAX_TEXT_CHARS = int(os.getenv("PRIVACY_NER_MAX_TEXT_CHARS", "65536"))
MAX_BODY_BYTES = MAX_TEXT_CHARS * 4 + 4096

SPACY_MODELS = {
    "en": os.getenv("PRIVACY_NER_EN_MODEL", "en_core_web_sm"),
    "zh": os.getenv("PRIVACY_NER_ZH_MODEL", "zh_core_web_sm"),
}

# Presidio 实体名 -> Sub2API 占位符类型（KIND）
PRESIDIO_TO_KIND = {
    "PERSON": "PERSON",
    "LOCATION": "LOCATION",
    "ORGANIZATION": "ORG",
    "NRP": "NRP",
    "DATE_TIME": "DATE_TIME",
    "EMAIL_ADDRESS": "EMAIL",
    "PHONE_NUMBER": "PHONE",
    "CREDIT_CARD": "CREDIT_CARD",
    "IP_ADDRESS": "IP_ADDRESS",
    "IBAN_CODE": "IBAN",
    "US_SSN": "ID_NUMBER",
    "URL": "URL",
}
KIND_TO_PRESIDIO = {v: k for k, v in PRESIDIO_TO_KIND.items()}
KIND_TO_PRESIDIO["ID_NUMBER"] = "US_SSN"

DEFAULT_KINDS = ["PERSON", "LOCATION", "ORG"]

CJK_RE = re.compile(r"[\u3400-\u9fff\uf900-\ufaff]")
LATIN_WORD_RE = re.compile(r"[A-Za-z]{2,}")


def build_analyzer() -> AnalyzerEngine:
    models = [{"lang_code": lang, "model_name": SPACY_MODELS[lang]} for lang in LANGUAGES if lang in SPACY_MODELS]
    if not models:
        raise SystemExit(f"no supported language in PRIVACY_NER_LANGUAGES={LANGUAGES}")
    nlp_config = {
        "nlp_engine_name": "spacy",
        "models": models,
        "ner_model_configuration": {
            "model_to_presidio_entity_mapping": {
                "PERSON": "PERSON",
                "PER": "PERSON",
                "NORP": "NRP",
                "GPE": "LOCATION",
                "LOC": "LOCATION",
                "FAC": "LOCATION",
                "ORG": "ORGANIZATION",
                "DATE": "DATE_TIME",
                "TIME": "DATE_TIME",
            },
            "low_confidence_score_multiplier": 0.4,
            "low_score_entity_names": [],
            "labels_to_ignore": ["CARDINAL", "ORDINAL", "QUANTITY", "MONEY", "PERCENT", "EVENT",
                                 "LANGUAGE", "LAW", "PRODUCT", "WORK_OF_ART"],
        },
    }
    nlp_engine = NlpEngineProvider(nlp_configuration=nlp_config).create_engine()
    langs = [m["lang_code"] for m in models]
    registry = RecognizerRegistry(supported_languages=langs)
    registry.load_predefined_recognizers(languages=langs, nlp_engine=nlp_engine)
    return AnalyzerEngine(nlp_engine=nlp_engine, registry=registry, supported_languages=langs)


ANALYZER = None


def pick_languages(text: str, requested: str | None) -> list:
    """中英混排时两种模型都跑一遍再合并；纯中文只跑 zh，其余只跑 en"""
    supported = ANALYZER.supported_languages
    if requested and requested in supported:
        return [requested]
    langs = []
    if "zh" in supported and CJK_RE.search(text):
        langs.append("zh")
    if "en" in supported and (not langs or LATIN_WORD_RE.search(text)):
        langs.append("en")
    return langs or [supported[0]]


def merge_overlaps(results: list) -> list:
    """多个模型结果重叠时保留得分高、跨度长的那个"""
    ordered = sorted(results, key=lambda r: (-r.score, -(r.end - r.start), r.start))
    kept = []
    for r in ordered:
        if all(r.end <= k.start or r.start >= k.end for k in kept):
            kept.append(r)
    return kept


def analyze(payload: dict) -> list:
    text = payload.get("text")
    if not isinstance(text, str):
        raise ValueError("text must be a string")
    if not text.strip():
        return []
    text = text[:MAX_TEXT_CHARS]

    kinds = payload.get("entities") or DEFAULT_KINDS
    entities = sorted({KIND_TO_PRESIDIO.get(str(k).upper(), str(k).upper()) for k in kinds})
    min_score = payload.get("min_score")
    try:
        min_score = max(MIN_SCORE, float(min_score)) if min_score is not None else MIN_SCORE
    except (TypeError, ValueError):
        min_score = MIN_SCORE

    results = []
    for language in pick_languages(text, payload.get("language")):
        results.extend(ANALYZER.analyze(text=text, language=language, entities=entities, score_threshold=min_score))
    out = []
    for r in merge_overlaps(results):
        kind = PRESIDIO_TO_KIND.get(r.entity_type, r.entity_type)
        out.append({"start": r.start, "end": r.end, "kind": kind, "score": round(float(r.score), 4)})
    out.sort(key=lambda x: (x["start"], -x["end"]))
    return out


class Handler(BaseHTTPRequestHandler):
    server_version = "privacy-ner/1"

    def log_message(self, fmt, *args):  # 不打印请求行以外的任何内容
        LOG.debug("%s - %s", self.address_string(), fmt % args)

    def _send(self, status: int, body) -> None:
        data = json.dumps(body, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        if self.path.rstrip("/") in ("/health", ""):
            self._send(200, {"status": "ok", "languages": ANALYZER.supported_languages})
            return
        self._send(404, {"error": "not found"})

    def do_POST(self):
        if self.path.rstrip("/") != "/analyze":
            self._send(404, {"error": "not found"})
            return
        try:
            length = int(self.headers.get("Content-Length") or 0)
        except ValueError:
            length = -1
        if length <= 0 or length > MAX_BODY_BYTES:
            self._send(413 if length > MAX_BODY_BYTES else 400, {"error": "invalid content length"})
            return
        try:
            payload = json.loads(self.rfile.read(length).decode("utf-8"))
            if not isinstance(payload, dict):
                raise ValueError("body must be a JSON object")
            self._send(200, analyze(payload))
        except ValueError as exc:
            self._send(400, {"error": str(exc)})
        except Exception:  # noqa: BLE001
            LOG.exception("analyze failed")
            self._send(500, {"error": "analyze failed"})


def main() -> None:
    global ANALYZER
    logging.basicConfig(level=os.getenv("PRIVACY_NER_LOG_LEVEL", "INFO"))
    logging.getLogger("presidio-analyzer").setLevel(logging.WARNING)
    ANALYZER = build_analyzer()
    # 预热，避免第一次请求超时
    for lang in ANALYZER.supported_languages:
        ANALYZER.analyze(text="warm up 预热", language=lang)
    LOG.info("privacy-ner listening on %s:%d, languages=%s", HOST, PORT, ANALYZER.supported_languages)
    ThreadingHTTPServer((HOST, PORT), Handler).serve_forever()


if __name__ == "__main__":
    main()
