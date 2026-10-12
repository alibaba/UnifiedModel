"""Regression coverage for private links leaking through generated assets."""

import contextlib
import io
import pathlib
import tempfile
import unittest

from public_link_guard import main


class PublicLinkGuardTest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = pathlib.Path(self.directory.name)

    def write(self, path, text):
        file = self.root / path
        file.parent.mkdir(parents=True, exist_ok=True)
        file.write_text(text, encoding="utf-8")

    def check(self):
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            result = main(self.root)
        return result, output.getvalue()

    def test_rejects_links_in_sources_and_all_generated_copies(self):
        paths = (
            "schemas/core/dataset/entity_set.schema.yaml",
            "expanded_schemas/entity_set.expanded.yaml",
            "internal/umodel/schemaspec/data/entity_set.expanded.yaml",
            "sdk/go/umodel/entity_set.go",
            "sdk/python/umodel/entity_set.py",
            "generated/java/src/main/java/com/umodel/schema/EntitySetV100.java",
            "docs/html/entity_set.html",
            "docs/html_cn/entity_set.html",
            "docs/html_en/entity_set.html",
            "docs/en/reference/schema/entity_set.md",
            "docs/zh/reference/schema/entity_set.md",
            "tools/generators/README.md",
            "README.md",
        )
        for path in paths:
            self.write(path, "Description\nSee: https://alidocs.dingtalk.com/i/nodes/test\n")
        result, output = self.check()
        self.assertEqual(result, 1)
        for path in paths:
            self.assertIn(f"{path}:2", output)
        self.assertNotIn("/i/nodes/test", output)

    def test_rejects_case_and_scheme_variants(self):
        links = (
            "https://alidocs.dingtalk.com",
            "http://alidocs.dingtalk.com/i/nodes/test",
            "HTTPS://ALIDOCS.DINGTALK.COM/i/nodes/test",
            "//alidocs.dingtalk.com/i/nodes/test",
            "https://sub.alidocs.dingtalk.com/i/nodes/test",
        )
        for link in links:
            with self.subTest(link=link):
                self.write("docs/page.html", f'<a href="{link}">Reference</a>')
                self.assertEqual(self.check()[0], 1)

    def test_allows_public_links_and_plain_domain_mentions(self):
        self.write("docs/page.md", "\n".join((
            "[Reference](https://github.com/alibaba/UnifiedModel)",
            "[DingTalk API](https://open.dingtalk.com/document/)",
            "[Different host](https://alidocs.dingtalk.com.example.org/page)",
            "Do not publish links to alidocs.dingtalk.com.",
        )))
        self.assertEqual(self.check()[0], 0)

    def test_ignores_local_artifacts_and_guard_fixtures(self):
        for path in (
            "benchmarks/README.md",
            "docs/node_modules/package/README.md",
            "docs/.vitepress/dist/page.html",
            "sdk/python/umodel/__pycache__/fixture.py",
            "generated/java/target/fixture.java",
            "tools/guards/test_fixture.py",
        ):
            self.write(path, "https://alidocs.dingtalk.com/i/nodes/test")
        self.assertEqual(self.check()[0], 0)


if __name__ == "__main__":
    unittest.main()
