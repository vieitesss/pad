from __future__ import annotations

import os
import re
import sys

from daily_update_workflow import resolve_template_date


def main() -> int:
    template_path = os.getenv(
        "ISSUE_TEMPLATE_PATH", ".github/ISSUE_TEMPLATE/daily-update.yml"
    ).strip()
    if not template_path:
        raise RuntimeError("ISSUE_TEMPLATE_PATH cannot be empty")

    if not os.path.exists(template_path):
        print(f"Skipping template refresh; {template_path} does not exist")
        return 0

    target_date = resolve_template_date(os.getenv("TEMPLATE_DATE", ""))
    with open(template_path, "r", encoding="utf-8") as handle:
        content = handle.read()

    updated, replacements = re.subn(
        r"^title:\s*.*$",
        f'title: "[Daily Update] [{target_date}]"',
        content,
        count=1,
        flags=re.MULTILINE,
    )
    if replacements == 0:
        raise RuntimeError(f"could not find title line in {template_path}")

    with open(template_path, "w", encoding="utf-8") as handle:
        handle.write(updated)

    print(f"Updated {template_path} to {target_date}")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:  # pragma: no cover
        print(f"Error: {exc}", file=sys.stderr)
        raise SystemExit(1) from exc
