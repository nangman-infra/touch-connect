#!/usr/bin/env python3

from __future__ import annotations

from datetime import datetime
import re
import sys
from collections import Counter
from dataclasses import dataclass
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DOCS_ROOT = ROOT / "docs"
ACTIVE_ROOT = DOCS_ROOT / "active"
PLANNED_ROOT = DOCS_ROOT / "planned"
ARCHIVE_ROOT = DOCS_ROOT / "archive"
SUPPORT_ROOT = DOCS_ROOT / "templates"
ACTIVE_INDEX = ACTIVE_ROOT / "README.md"

STRUCTURAL_ACTIVE_EXCEPTIONS = {
    DOCS_ROOT / "README.md",
    PLANNED_ROOT / "README.md",
    ARCHIVE_ROOT / "README.md",
}

VALID_STATUSES = {"active", "planned", "archived", "support"}
VALID_TYPES = {
    "root-index",
    "active-index",
    "governance-policy",
    "engineering-baseline",
    "contract-model",
    "product-scenario",
    "foundation",
    "research",
    "working-draft",
    "archive-note",
    "state-directory-index",
    "template-asset",
}
REQUIRED_FIELDS = (
    "Document Status",
    "Document Type",
    "Scope",
    "Canonical Path",
    "Source Of Truth",
    "Last Reviewed",
)
LINK_PATTERN = re.compile(r"\[[^\]]+\]\(([^)]+)\)")
DATE_PATTERN = re.compile(r"^\d{4}-\d{2}-\d{2}$")


@dataclass(frozen=True)
class Document:
    path: Path
    metadata: dict[str, str]
    body: str


def strip_wrapping(value: str) -> str:
    stripped = value.strip()
    if stripped.startswith("`") and stripped.endswith("`"):
        return stripped[1:-1]
    if stripped.startswith("<") and stripped.endswith(">"):
        return stripped[1:-1]
    return stripped


def parse_metadata(text: str) -> dict[str, str]:
    metadata: dict[str, str] = {}
    for line in text.splitlines():
        if not line.startswith("> "):
            break
        key, separator, raw_value = line[2:].partition(":")
        if not separator:
            continue
        metadata[key.strip()] = strip_wrapping(raw_value)
    return metadata


def load_document(path: Path) -> Document:
    text = path.read_text(encoding="utf-8")
    return Document(path=path, metadata=parse_metadata(text), body=text)


def is_under(path: Path, parent: Path) -> bool:
    try:
        path.relative_to(parent)
        return True
    except ValueError:
        return False


def resolve_repo_path(value: str, base_dir: Path | None = None) -> Path:
    path = Path(strip_wrapping(value))
    if path.is_absolute():
        return path
    if path.parts and path.parts[0] == "docs":
        return ROOT / path
    if base_dir is not None:
        return base_dir / path
    return ROOT / path


def path_kind(path: Path) -> str:
    if path in STRUCTURAL_ACTIVE_EXCEPTIONS:
        return "structural-active"
    if path == ACTIVE_INDEX:
        return "active-self-index"
    if is_under(path, ACTIVE_ROOT):
        return "active"
    if is_under(path, PLANNED_ROOT):
        return "planned"
    if is_under(path, ARCHIVE_ROOT):
        return "archived"
    if is_under(path, SUPPORT_ROOT):
        return "support"
    return "unknown"


def expected_status(kind: str) -> str | None:
    if kind in {"structural-active", "active-self-index", "active"}:
        return "active"
    if kind == "planned":
        return "planned"
    if kind == "archived":
        return "archived"
    if kind == "support":
        return "support"
    return None


def load_active_registry(index_path: Path) -> list[Path]:
    text = index_path.read_text(encoding="utf-8")
    registered = [ACTIVE_INDEX]
    for raw_target in LINK_PATTERN.findall(text):
        target = strip_wrapping(raw_target)
        if "://" in target or target.startswith("#"):
            continue
        path = resolve_repo_path(target, index_path.parent)
        if is_under(path, DOCS_ROOT):
            registered.append(path)
    return registered


def validate_archive_note(document: Document) -> list[str]:
    errors: list[str] = []
    required_note_fields = ("Archived", "Reason", "Superseded by")
    for field in required_note_fields:
        pattern = rf"^> {re.escape(field)}:\s+.+$"
        if not re.search(pattern, document.body, flags=re.MULTILINE):
            errors.append(f"archive note 필드가 빠져 있습니다: {field}")
    return errors


def validate_document(document: Document, active_registry: set[Path]) -> list[str]:
    errors: list[str] = []
    metadata = document.metadata

    for field in REQUIRED_FIELDS:
        if field not in metadata or not metadata[field]:
            errors.append(f"필수 메타데이터가 없습니다: {field}")

    status = metadata.get("Document Status")
    if status and status not in VALID_STATUSES:
        errors.append(f"허용되지 않은 Document Status입니다: {status}")

    doc_type = metadata.get("Document Type")
    if doc_type and doc_type not in VALID_TYPES:
        errors.append(f"허용되지 않은 Document Type입니다: {doc_type}")

    canonical_path = metadata.get("Canonical Path")
    if canonical_path and resolve_repo_path(canonical_path) != document.path:
        errors.append(
            f"Canonical Path가 실제 경로와 다릅니다: {canonical_path} != {document.path}"
        )

    source_of_truth = metadata.get("Source Of Truth")
    if source_of_truth and source_of_truth not in {"yes", "no"}:
        errors.append(f"Source Of Truth 값이 잘못되었습니다: {source_of_truth}")

    last_reviewed = metadata.get("Last Reviewed")
    if last_reviewed:
        if not DATE_PATTERN.match(last_reviewed):
            errors.append(
                "Last Reviewed 형식이 잘못되었습니다. YYYY-MM-DD 형식이어야 합니다"
            )
        else:
            try:
                datetime.strptime(last_reviewed, "%Y-%m-%d")
            except ValueError:
                errors.append(f"Last Reviewed 날짜가 유효하지 않습니다: {last_reviewed}")

    for field in ("Supersedes", "Superseded By"):
        value = metadata.get(field)
        if not value:
            continue
        if value == "none":
            continue
        supersession_path = resolve_repo_path(value)
        if not is_under(supersession_path, DOCS_ROOT):
            errors.append(f"{field}는 docs 경로를 가리켜야 합니다: {value}")
            continue
        if not supersession_path.exists():
            errors.append(f"{field}가 존재하지 않는 문서를 가리킵니다: {value}")

    kind = path_kind(document.path)
    expected = expected_status(kind)
    if expected is None:
        errors.append(f"관리 대상 밖의 경로입니다: {document.path}")
        return errors

    if status and expected != status:
        errors.append(
            "메타데이터 상태와 경로 상태가 충돌합니다: "
            f"metadata={status}, path={expected}"
        )

    is_registered_active = document.path in active_registry
    if expected == "active":
        if not is_registered_active:
            errors.append("active 문서인데 active registry에 등록되어 있지 않습니다")
        if source_of_truth == "no":
            errors.append("active 문서는 Source Of Truth가 yes여야 합니다")
    else:
        if is_registered_active:
            errors.append(
                f"{expected} 문서인데 active registry에 등록되어 있습니다"
            )

    if expected == "support" and source_of_truth == "yes":
        errors.append("support 문서는 Source Of Truth가 no여야 합니다")

    if kind == "structural-active" and doc_type not in {
        "root-index",
        "state-directory-index",
    }:
        errors.append(
            "구조 인덱스 예외 문서는 root-index 또는 state-directory-index 타입이어야 합니다"
        )

    if kind == "active-self-index" and doc_type != "active-index":
        errors.append("docs/active/README.md는 Document Type이 active-index여야 합니다")

    if expected == "archived":
        errors.extend(validate_archive_note(document))

    return errors


def main() -> int:
    documents = sorted(DOCS_ROOT.rglob("*.md"))
    if not documents:
        print(f"검사할 문서가 없습니다: {DOCS_ROOT}")
        return 1

    active_registry_entries = load_active_registry(ACTIVE_INDEX)
    active_registry = set(active_registry_entries)
    loaded_documents = [load_document(path) for path in documents]
    known_document_paths = {document.path for document in loaded_documents}
    all_errors: list[str] = []
    status_counter: Counter[str] = Counter()

    dead_registry_targets = sorted(
        path for path in active_registry if path not in known_document_paths
    )
    for path in dead_registry_targets:
        all_errors.append(f"- {ACTIVE_INDEX}: active registry에 없는 파일을 가리킵니다: {path}")

    for document in loaded_documents:
        status = document.metadata.get("Document Status", "missing")
        status_counter[status] += 1
        errors = validate_document(document, active_registry)
        for error in errors:
            all_errors.append(f"- {document.path}: {error}")

    if all_errors:
        print("문서 벨리데이션 실패")
        print()
        print("\n".join(all_errors))
        return 1

    print("문서 벨리데이션 통과")
    print(f"- 검사한 문서 수: {len(loaded_documents)}")
    for status in sorted(status_counter):
        print(f"- {status}: {status_counter[status]}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
