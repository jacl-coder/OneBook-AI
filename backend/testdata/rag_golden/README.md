# RAG Golden Test Data

This directory stores product-level regression cases for the document expert RAG flow.

`internship_certificate.jsonl` targets a single-page internship certificate and covers:

- document overview
- single facts
- follow-up rewriting
- summary
- evidence-bound refusal

Each JSONL row is intentionally model-agnostic. `expectedAnswerContains` and
`expectedCitationCountMin` are the stable checks; exact wording is not fixed.
