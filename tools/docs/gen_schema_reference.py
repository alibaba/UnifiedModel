#!/usr/bin/env python3
"""Generate the UModel **Schema reference** (bilingual) from the source schemas.

Source of truth: ``schemas/core/**/*.schema.yaml`` and ``schemas/includes/*.schema.yaml``.
Output: ``docs/{en,zh}/reference/schema/**`` Markdown (consumed by the VitePress site)
plus a sidebar fragment imported by the VitePress config.

Organized per the UModel standard layering (see the contribution spec): the L0 Core
abstractions — **EntitySet / DataSet / Link / Storage** — with shared building blocks
(``metadata``, ``schema``, ``telemetry_data``, ``link``, ``field_spec``, ``metric`` …)
documented once on a shared-types page and referenced from each schema.

These Markdown files are a GENERATED ARTIFACT. Do not hand-edit them; change the
schemas or this generator and re-run.

Usage::

    python3 tools/docs/gen_schema_reference.py            # (re)generate
    python3 tools/docs/gen_schema_reference.py --check     # CI drift gate: non-zero exit if output would change
"""
from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[2]
CORE = ROOT / "schemas" / "core"
INCLUDES = ROOT / "schemas" / "includes"
OUT = {lang: ROOT / "docs" / lang / "reference" / "schema" for lang in ("en", "zh")}
SIDEBAR = ROOT / "docs" / ".vitepress" / "config" / "schema-sidebar.json"

LANGS = ("en", "zh")
LK = {"en": "en_us", "zh": "zh_cn"}

# L0 Core abstraction categories. dir + the entity_set special-case.
CATEGORIES = [
    ("entity-set", {"en": "EntitySet", "zh": "EntitySet（实体集）"}),
    ("dataset", {"en": "DataSet", "zh": "DataSet（数据集）"}),
    ("link", {"en": "Link", "zh": "Link（链接）"}),
    ("storage", {"en": "Storage", "zh": "Storage（存储）"}),
]

# Shared includes documented once on the shared-types page (name -> heading).
SHARED_TYPES = [
    "metadata",
    "schema",
    "telemetry_data",
    "link",
    "field_spec",
    "metric",
    "observation",
    "value_mapping",
]

# Property names whose array items are a well-known shared type -> link instead of inline.
ARRAY_SHARED = {"fields": "field_spec", "labels": "field_spec", "metrics": "metric"}

# UI strings.
T = {
    "en": {
        "title": "Schema Reference",
        "kind": "Kind",
        "field": "Field",
        "type": "Type",
        "required": "Required",
        "default": "Default",
        "description": "Description",
        "envelope": "Every element shares the standard envelope",
        "inherits": "Inherits",
        "spec_fields": "`spec` fields",
        "no_spec": "This element has no additional `spec` fields beyond the inherited base.",
        "see_source": "see schema source",
        "shared_intro": "Reusable building blocks referenced by the schemas above. Documented once here.",
        "yes": "yes",
        "values": "values",
    },
    "zh": {
        "title": "Schema 参考",
        "kind": "Kind",
        "field": "字段",
        "type": "类型",
        "required": "必填",
        "default": "默认值",
        "description": "说明",
        "envelope": "每个元素共享标准信封",
        "inherits": "继承",
        "spec_fields": "`spec` 字段",
        "no_spec": "除继承的基类外，该元素没有额外的 `spec` 字段。",
        "see_source": "见 schema 源文件",
        "shared_intro": "上述 schema 引用的可复用构建块，在此统一记录一次。",
        "yes": "是",
        "values": "个取值",
    },
}


def load(path: Path) -> dict:
    with path.open(encoding="utf-8") as fh:
        return yaml.safe_load(fh) or {}


def lang_text(desc, lang: str) -> str:
    """Pull a bilingual description, sanitize internal links, collapse + truncate."""
    if not isinstance(desc, dict):
        return ""
    raw = desc.get(LK[lang]) or desc.get(LK["en" if lang == "zh" else "zh"]) or ""
    if not isinstance(raw, str):
        return ""
    # Strip internal references that must not leak into public docs.
    raw = re.sub(r"\s*(参考|See|see)\s*[:：]\s*https?://alidocs\.dingtalk\.com/\S*", "", raw)
    raw = re.sub(r"https?://alidocs\.dingtalk\.com/\S*", "", raw)
    raw = re.sub(r"\s+", " ", raw).strip()
    if len(raw) > 220:
        raw = raw[:217].rstrip() + "…"
    return raw.replace("|", r"\|")


def first_version(schema: dict) -> dict:
    versions = schema.get("versions") or []
    return versions[0] if versions else {}


def constraint_of(prop: dict) -> dict:
    c = prop.get("constraint")
    return c if isinstance(c, dict) else {}


def is_required(prop: dict) -> bool:
    return bool(constraint_of(prop).get("required"))


def default_of(prop: dict) -> str:
    c = constraint_of(prop)
    if "default_value" in c and c["default_value"] is not None:
        v = c["default_value"]
        if isinstance(v, bool):
            v = "true" if v else "false"
        return f"`{v}`"
    return ""


def type_str(prop: dict, link_prefix: str) -> str:
    """Human-readable type, linking shared types where possible."""
    c = constraint_of(prop)
    t = prop.get("type")
    ref = prop.get("type_ref")
    if ref:
        name = ref.split(":")[0]
        if name in SHARED_TYPES:
            return f"[{name}]({link_prefix}shared-types#{name})"
        return f"`{name}`"
    # enum may be expressed as type: enum, or via constraint.enum on any type
    enum = c.get("enum") if isinstance(c.get("enum"), dict) else None
    if t == "enum" or enum:
        vals = (enum or {}).get("values") or []
        shown = ", ".join(f"`{v}`" for v in vals[:8])
        if len(vals) > 8:
            shown += f" … ({len(vals)} {T['en']['values']})"
        return f"enum: {shown}" if shown else "enum"
    if t == "array":
        item = (c.get("array") or {}).get("item") or {}
        iref = item.get("type_ref")
        if iref:
            name = iref.split(":")[0]
            if name in SHARED_TYPES:
                return f"array&lt;[{name}]({link_prefix}shared-types#{name})&gt;"
        return f"array&lt;{item.get('type', 'object')}&gt;"
    if t == "map":
        m = c.get("map") or {}
        kt = (m.get("key") or {}).get("type", "string")
        vt = (m.get("value") or {}).get("type", "string")
        return f"map&lt;{kt}, {vt}&gt;"
    if t == "semantic_string":
        return "semantic_string (i18n)"
    if t == "object":
        return "object"
    return f"`{t}`" if t else "object"


def collect_rows(props: dict, lang: str, link_prefix: str, prefix: str = "", depth: int = 0):
    """Flatten properties into table rows (depth-limited, shared structures linked)."""
    rows = []
    if not isinstance(props, dict):
        return rows
    for name, prop in props.items():
        if not isinstance(prop, dict):
            continue
        path = f"{prefix}{name}"
        # Collapse well-known arrays of shared types to a single linked row.
        shared = ARRAY_SHARED.get(name)
        if shared and prop.get("type") == "array":
            tstr = f"array&lt;[{shared}]({link_prefix}shared-types#{shared})&gt;"
        else:
            tstr = type_str(prop, link_prefix)
        req = T[lang]["yes"] if is_required(prop) else ""
        rows.append((f"`{path}`", tstr, req, default_of(prop), lang_text(prop.get("description"), lang)))
        # Recurse into nested objects (cap depth) unless collapsed to a shared link.
        if shared and prop.get("type") == "array":
            continue
        if depth < 1:
            sub = None
            if prop.get("type") == "object" and isinstance(prop.get("properties"), dict):
                sub = prop["properties"]
            elif prop.get("type") == "array":
                item = (constraint_of(prop).get("array") or {}).get("item") or {}
                if isinstance(item.get("properties"), dict) and not item.get("type_ref"):
                    sub = item["properties"]
            if sub:
                rows.extend(collect_rows(sub, lang, link_prefix, prefix=f"{path}.", depth=depth + 1))
    return rows


def render_table(props: dict, lang: str, link_prefix: str) -> str:
    rows = collect_rows(props, lang, link_prefix)
    if not rows:
        return ""
    h = T[lang]
    out = [f"| {h['field']} | {h['type']} | {h['required']} | {h['default']} | {h['description']} |",
           "|---|---|---|---|---|"]
    for f, t, r, d, desc in rows:
        out.append(f"| {f} | {t} | {r} | {d} | {desc} |")
    return "\n".join(out)


def extends_links(node: dict, lang: str, link_prefix: str) -> str:
    refs = node.get("extends") or []
    parts = []
    for ref in refs:
        name = ref.split(":")[0]
        if name in SHARED_TYPES:
            parts.append(f"[{name}]({link_prefix}shared-types#{name})")
        else:
            parts.append(f"`{name}`")
    return ", ".join(parts)


def link_prefix_for(rel_path: str) -> str:
    """Relative prefix from a page (path relative to schema root) back to schema root."""
    return "../" * rel_path.count("/")


def kind_value(spec_props: dict) -> str:
    kind = spec_props.get("kind") or {}
    vals = ((constraint_of(kind).get("enum")) or {}).get("values") or []
    return vals[0] if vals else ""


def render_schema_page(name: str, schema: dict, lang: str, rel_path: str) -> str:
    h = T[lang]
    lp = link_prefix_for(rel_path)
    ver = first_version(schema)
    spec_props = (ver.get("spec") or {}).get("properties") or {}
    inner_spec = spec_props.get("spec") or {}
    lines = [f"# {name}", ""]
    desc = lang_text(schema.get("description"), lang)
    if desc:
        lines += [desc, ""]
    kv = kind_value(spec_props)
    if kv:
        lines += [f"**{h['kind']}**: `{kv}`", ""]
    lines += [f"> {h['envelope']} `kind` · [metadata]({lp}shared-types#metadata) · [schema]({lp}shared-types#schema).", ""]
    inh = extends_links(inner_spec, lang, lp)
    if inh:
        lines += [f"**{h['inherits']}**: {inh}", ""]
    lines += [f"## {h['spec_fields']}", ""]
    table = render_table(inner_spec.get("properties") or {}, lang, lp)
    lines += [table if table else f"_{h['no_spec']}_", ""]
    return "\n".join(lines)


def render_shared_page(includes: dict, lang: str) -> str:
    h = T[lang]
    lines = [f"# {'Shared types' if lang == 'en' else '共享类型'}", "", h["shared_intro"], ""]
    for name in SHARED_TYPES:
        schema = includes.get(name)
        if not schema:
            continue
        ver = first_version(schema)
        props = (ver.get("spec") or {}).get("properties") or {}
        lines += [f"## {name}", ""]
        desc = lang_text(schema.get("description"), lang)
        if desc:
            lines += [desc, ""]
        table = render_table(props, lang, "")
        if table:
            lines += [table, ""]
    return "\n".join(lines)


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--check", action="store_true", help="fail if generated output would change")
    args = ap.parse_args()

    includes = {p.stem.replace(".schema", ""): load(p) for p in INCLUDES.glob("*.schema.yaml")}
    core = {}
    for p in CORE.rglob("*.schema.yaml"):
        core[p.stem.replace(".schema", "")] = (p.relative_to(CORE).parts[0], load(p))

    # category -> list of (name, schema)
    cats = {key: [] for key, _ in CATEGORIES}
    for name, (subdir, schema) in sorted(core.items()):
        if name == "entity_set":
            cats["entity-set"].append((name, schema))
        elif subdir == "dataset":
            cats["dataset"].append((name, schema))
        elif subdir == "link":
            cats["link"].append((name, schema))
        elif subdir == "storage":
            cats["storage"].append((name, schema))

    files: dict[str, str] = {}  # rel path (no lang) -> content is per-lang; store per lang below
    per_lang_files: dict[str, dict[str, str]] = {lang: {} for lang in LANGS}
    sidebar = {lang: [] for lang in LANGS}

    for lang in LANGS:
        # per-schema pages
        for key, label in CATEGORIES:
            items = cats[key]
            if not items:
                continue
            sb_items = []
            for name, schema in items:
                if key == "entity-set":
                    rel = f"core/{name.replace('_', '-')}.md"
                else:
                    rel = f"core/{key}/{name.replace('_', '-')}.md"
                per_lang_files[lang][rel] = render_schema_page(name, schema, lang, rel)
                sb_items.append({"text": name, "link": f"/{lang}/reference/schema/{rel[:-3]}"})
            sidebar[lang].append({"text": label[lang], "collapsed": False, "items": sb_items})
        # shared types
        per_lang_files[lang]["shared-types.md"] = render_shared_page(includes, lang)
        sidebar[lang].append({
            "text": "Shared types" if lang == "en" else "共享类型",
            "link": f"/{lang}/reference/schema/shared-types",
        })
        # index
        per_lang_files[lang]["index.md"] = render_index(cats, lang)

    import json
    sidebar_json = json.dumps(sidebar, ensure_ascii=False, indent=2) + "\n"

    if args.check:
        changed = []
        for lang in LANGS:
            for rel, content in per_lang_files[lang].items():
                path = OUT[lang] / rel
                if not path.exists() or path.read_text(encoding="utf-8") != content:
                    changed.append(str(path))
        if not SIDEBAR.exists() or SIDEBAR.read_text(encoding="utf-8") != sidebar_json:
            changed.append(str(SIDEBAR))
        if changed:
            print("Schema docs are stale; run `make docs-schema`. Changed:", file=sys.stderr)
            for c in changed:
                print(f"  {c}", file=sys.stderr)
            return 1
        print("Schema docs up to date.")
        return 0

    written = 0
    for lang in LANGS:
        for rel, content in per_lang_files[lang].items():
            path = OUT[lang] / rel
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(content, encoding="utf-8")
            written += 1
    SIDEBAR.write_text(sidebar_json, encoding="utf-8")
    print(f"Generated {written} schema pages + sidebar for {len(LANGS)} locales.")
    return 0


def render_index(cats: dict, lang: str) -> str:
    if lang == "en":
        lines = [
            "# Schema Reference", "",
            "UModel schemas are organized as a layered standard. This reference documents the "
            "**L0 Core** abstractions — the base vocabulary every model is built from.", "",
            "## Standard layers", "",
            "| Layer | Name | Status |", "|---|---|---|",
            "| **L0** | **UModel Core** — EntitySet, DataSet, Link, Storage | Documented below |",
            "| L1 | Semantic Conventions — shared service/host/pod/database semantics | Roadmap |",
            "| L2 | Domain Profiles — DevOps, APM, Kubernetes, AIOps, … | Roadmap |",
            "| L3 | Conformance — automated standard-compliance checks | Roadmap |", "",
            "See the contribution guide for how to propose L1–L3 content.", "",
            "## L0 Core abstractions", "",
        ]
        cat_label = {"entity-set": "EntitySet", "dataset": "DataSet", "link": "Link", "storage": "Storage"}
    else:
        lines = [
            "# Schema 参考", "",
            "UModel 的 schema 按分层标准组织。本参考记录 **L0 Core** 抽象——构建一切模型的基础词汇。", "",
            "## 标准分层", "",
            "| 层级 | 名称 | 状态 |", "|---|---|---|",
            "| **L0** | **UModel Core** —— EntitySet、DataSet、Link、Storage | 见下文 |",
            "| L1 | Semantic Conventions —— 通用 service/host/pod/database 语义 | 规划中 |",
            "| L2 | Domain Profiles —— DevOps、APM、Kubernetes、AIOps…… | 规划中 |",
            "| L3 | Conformance —— 自动化标准兼容性校验 | 规划中 |", "",
            "L1–L3 的贡献方式见贡献指南。", "",
            "## L0 Core 抽象", "",
        ]
        cat_label = {"entity-set": "EntitySet（实体集）", "dataset": "DataSet（数据集）", "link": "Link（链接）", "storage": "Storage（存储）"}
    for key, _ in CATEGORIES:
        items = cats[key]
        if not items:
            continue
        lines += [f"### {cat_label[key]}", ""]
        for name, _schema in items:
            if key == "entity-set":
                link = f"./core/{name.replace('_', '-')}"
            else:
                link = f"./core/{key}/{name.replace('_', '-')}"
            lines.append(f"- [{name}]({link})")
        lines.append("")
    tail = "[Shared types](./shared-types)" if lang == "en" else "[共享类型](./shared-types)"
    lines += [f"### {'Building blocks' if lang == 'en' else '构建块'}", "", f"- {tail}", ""]
    return "\n".join(lines)


if __name__ == "__main__":
    raise SystemExit(main())
