from __future__ import annotations

import json
import os
import re
from dataclasses import dataclass
from datetime import date, datetime, timedelta, timezone
from html import unescape
from typing import Any
from urllib import error, parse, request


API_BASE = "https://api.github.com"


@dataclass
class GitHubClient:
    token: str
    repository: str

    def __post_init__(self) -> None:
        parts = self.repository.split("/", 1)
        if len(parts) != 2 or not parts[0] or not parts[1]:
            raise ValueError(f"invalid GITHUB_REPOSITORY value: {self.repository!r}")
        self.owner = parts[0]
        self.repo = parts[1]

    def request_json(
        self,
        method: str,
        api_path: str,
        *,
        params: dict[str, Any] | None = None,
        body: dict[str, Any] | None = None,
    ) -> Any:
        url = API_BASE + api_path
        if params:
            query = parse.urlencode(params, doseq=True)
            if query:
                url += f"?{query}"

        headers = {
            "Accept": "application/vnd.github+json",
            "Authorization": f"Bearer {self.token}",
            "User-Agent": "pad-workflows",
            "X-GitHub-Api-Version": "2022-11-28",
        }

        payload = None
        if body is not None:
            payload = json.dumps(body).encode("utf-8")
            headers["Content-Type"] = "application/json"

        req = request.Request(url, data=payload, headers=headers, method=method)
        try:
            with request.urlopen(req) as response:
                content = response.read()
        except error.HTTPError as exc:
            detail = exc.read().decode("utf-8", errors="replace").strip()
            raise RuntimeError(
                f"GitHub API {method} {api_path} failed with HTTP {exc.code}: {detail}"
            ) from exc

        if not content:
            return None

        return json.loads(content)

    def paginate_json(
        self, api_path: str, *, params: dict[str, Any] | None = None
    ) -> list[dict[str, Any]]:
        page = 1
        items: list[dict[str, Any]] = []
        base_params = dict(params or {})

        while True:
            page_params = {**base_params, "per_page": 100, "page": page}
            batch = self.request_json("GET", api_path, params=page_params)
            if not batch:
                break
            if not isinstance(batch, list):
                raise RuntimeError(
                    f"expected list response from {api_path}, got {type(batch).__name__}"
                )

            items.extend(batch)
            if len(batch) < 100:
                break
            page += 1

        return items


def require_env(name: str) -> str:
    value = os.getenv(name, "").strip()
    if not value:
        raise RuntimeError(f"missing required environment variable {name}")
    return value


def resolve_report_date(raw: str) -> tuple[str, str]:
    resolved = parse_date(raw) if raw else datetime.now(timezone.utc).date()
    return resolved.strftime("%Y-%m-%d"), resolved.strftime("%Y/%m/%d")


def resolve_template_date(raw: str) -> str:
    resolved = (
        parse_date(raw)
        if raw
        else datetime.now(timezone.utc).date() + timedelta(days=1)
    )
    return resolved.strftime("%Y/%m/%d")


def parse_date(raw: str) -> date:
    trimmed = raw.strip()
    for layout in ("%Y-%m-%d", "%Y/%m/%d"):
        try:
            return datetime.strptime(trimmed, layout).date()
        except ValueError:
            continue

    raise RuntimeError(f"invalid date {raw!r}; use YYYY-MM-DD or YYYY/MM/DD")


def collect_daily_update_issues(
    client: GitHubClient,
    report_date: str,
    label: str,
) -> tuple[list[dict[str, Any]], str, str]:
    date_str, title_date_str = resolve_report_date(report_date)
    print(
        f"Collecting member issues for {date_str} ({title_date_str}) with label {label!r}"
    )

    issues = client.paginate_json(
        f"/repos/{client.owner}/{client.repo}/issues",
        params={
            "labels": label,
            "state": "all",
            "sort": "created",
            "direction": "desc",
        },
    )

    filtered = [
        issue
        for issue in issues
        if "pull_request" not in issue and title_date_str in issue.get("title", "")
    ]
    print(f"Found {len(filtered)} member issue(s)")
    return filtered, date_str, title_date_str


def enrich_user_info(
    client: GitHubClient, issues: list[dict[str, Any]]
) -> list[dict[str, Any]]:
    cache: dict[str, dict[str, Any]] = {}
    enriched: list[dict[str, Any]] = []

    for issue in issues:
        login = issue.get("user", {}).get("login", "")
        profile = cache.get(login)
        if profile is None and login:
            try:
                profile = client.request_json(
                    "GET", f"/users/{parse.quote(login, safe='')}"
                )
            except RuntimeError as exc:
                print(f"Warning: failed to fetch user info for {login}: {exc}")
                profile = {}
            cache[login] = profile

        user = dict(issue.get("user") or {})
        user["display_name"] = (profile or {}).get("name") or login
        user["avatar_url"] = (profile or {}).get("avatar_url") or user.get(
            "avatar_url", ""
        )

        enriched_issue = dict(issue)
        enriched_issue["user"] = user
        enriched.append(enriched_issue)

    return enriched


def create_or_reuse_report_issue(
    client: GitHubClient,
    title_date_str: str,
    markdown_body: str,
    report_label: str,
) -> dict[str, Any]:
    title = f"[Daily Report] {title_date_str}"
    report_issues = client.paginate_json(
        f"/repos/{client.owner}/{client.repo}/issues",
        params={"labels": report_label, "state": "all"},
    )

    matches = [
        issue
        for issue in report_issues
        if "pull_request" not in issue and issue.get("title") == title
    ]
    if matches:
        matches.sort(key=lambda issue: issue.get("number", 0), reverse=True)
        report_issue = matches[0]
        print(
            f"Reusing report issue #{report_issue['number']}: {report_issue['html_url']}"
        )
        return report_issue

    created = client.request_json(
        "POST",
        f"/repos/{client.owner}/{client.repo}/issues",
        body={
            "title": title,
            "body": markdown_body,
            "labels": [report_label],
        },
    )
    print(f"Created report issue #{created['number']}: {created['html_url']}")
    return created


def update_issue_body(client: GitHubClient, issue_number: int, body: str) -> None:
    client.request_json(
        "PATCH",
        f"/repos/{client.owner}/{client.repo}/issues/{issue_number}",
        body={"body": body},
    )


def close_individual_issues(
    client: GitHubClient,
    issues: list[dict[str, Any]],
    report_issue_url: str,
) -> tuple[int, int]:
    comment = (
        f"Processed and included in the [Daily Report]({report_issue_url}).\n\n"
        "Closing this individual issue."
    )

    success_count = 0
    error_count = 0
    for issue in issues:
        number = issue.get("number")
        login = issue.get("user", {}).get("login", "unknown")
        try:
            if not issue_comment_exists(client, number, report_issue_url):
                client.request_json(
                    "POST",
                    f"/repos/{client.owner}/{client.repo}/issues/{number}/comments",
                    body={"body": comment},
                )

            if issue.get("state", "").lower() != "closed":
                client.request_json(
                    "PATCH",
                    f"/repos/{client.owner}/{client.repo}/issues/{number}",
                    body={"state": "closed", "state_reason": "completed"},
                )

            print(f"Processed member issue #{number} by @{login}")
            success_count += 1
        except RuntimeError as exc:
            print(f"Warning: failed to process issue #{number}: {exc}")
            error_count += 1

    return success_count, error_count


def issue_comment_exists(
    client: GitHubClient, issue_number: int, report_issue_url: str
) -> bool:
    comments = client.paginate_json(
        f"/repos/{client.owner}/{client.repo}/issues/{issue_number}/comments"
    )
    for comment in comments:
        body = comment.get("body", "")
        if report_issue_url in body or report_issue_url in unescape(body):
            return True
    return False


def parse_issue_body(body: str) -> dict[str, Any]:
    sections = {
        "yesterday": "",
        "today": "",
        "blockers": "",
        "parking_lot": False,
        "parking_lot_details": "",
        "additional_comments": "",
    }
    if not body:
        return sections

    field_sections = split_sections_by_field_id(body)

    sections["yesterday"] = extract_field_section(
        body,
        field_sections=field_sections,
        field_id="yesterday",
        patterns=[
            r"### ✅ What did you do yesterday\?\s*([\s\S]*?)(?=###|##|$)",
            r"## ✅ What did you do yesterday\?\s*([\s\S]*?)(?=##|$)",
        ],
    )
    sections["today"] = extract_field_section(
        body,
        field_sections=field_sections,
        field_id="today",
        patterns=[
            r"### 🎯 What will you do today\?\s*([\s\S]*?)(?=###|##|$)",
            r"## 🎯 What will you do today\?\s*([\s\S]*?)(?=##|$)",
        ],
    )
    sections["blockers"] = extract_field_section(
        body,
        field_sections=field_sections,
        field_id="blockers",
        patterns=[
            r"### 🚧 Any blockers\?\s*([\s\S]*?)(?=###|##|$)",
            r"## 🚧 Any blockers\?\s*([\s\S]*?)(?=##|$)",
        ],
        clean_empty=True,
    )
    sections["parking_lot_details"] = extract_field_section(
        body,
        field_sections=field_sections,
        field_id="parking_details",
        patterns=[
            r"### 📝 Parking Lot Details\s*([\s\S]*?)(?=###|##|$)",
            r"## 📝 Parking Lot Details\s*([\s\S]*?)(?=##|$)",
        ],
        clean_empty=True,
    )
    sections["additional_comments"] = extract_field_section(
        body,
        field_sections=field_sections,
        field_id="comments",
        patterns=[
            r"### 💬 Additional Comments\s*([\s\S]*?)(?=###|##|$)",
            r"## 💬 Additional Comments\s*([\s\S]*?)(?=##|$)",
        ],
        clean_empty=True,
    )

    parking_section = field_sections.get("parking_lot", "") or extract_section(
        body,
        [
            r"### 🚨 Do you request a Parking Lot or escalation\?\s*([\s\S]*?)(?=###|##|$)",
            r"## 🚨 Do you request a Parking Lot or escalation\?\s*([\s\S]*?)(?=##|$)",
            r"### 🚨 Parking Lot / Escalation\s*([\s\S]*?)(?=###|##|$)",
            r"## 🚨 Parking Lot / Escalation\s*([\s\S]*?)(?=##|$)",
        ],
    )
    sections["parking_lot"] = bool(
        re.search(r"^- \[x\]", parking_section, flags=re.IGNORECASE | re.MULTILINE)
        or re.search(
            r"- ✅ Yes, I need a Parking Lot", parking_section, flags=re.IGNORECASE
        )
        or sections["parking_lot_details"]
    )
    return sections


def extract_field_section(
    body: str,
    *,
    field_sections: dict[str, str],
    field_id: str,
    patterns: list[str],
    clean_empty: bool = False,
) -> str:
    value = field_sections.get(field_id, "")
    if value:
        return clean_empty_response(value) if clean_empty else value
    return extract_section(body, patterns, clean_empty=clean_empty)


def split_sections_by_field_id(body: str) -> dict[str, str]:
    sections: dict[str, str] = {}
    current_id = ""
    current_lines: list[str] = []
    heading_pattern = re.compile(
        r"^#{2,6} .*?<!--\s*pad:id:([A-Za-z0-9._-]+)\s*-->\s*$"
    )

    for line in body.splitlines():
        match = heading_pattern.match(line.strip())
        if match:
            if current_id:
                sections[current_id] = "\n".join(current_lines).strip()
            current_id = match.group(1)
            current_lines = []
            continue

        if current_id:
            current_lines.append(line)

    if current_id:
        sections[current_id] = "\n".join(current_lines).strip()

    return sections


def extract_section(
    body: str, patterns: list[str], *, clean_empty: bool = False
) -> str:
    for pattern in patterns:
        match = re.search(pattern, body, flags=re.MULTILINE)
        if not match:
            continue
        value = match.group(1).strip()
        return clean_empty_response(value) if clean_empty else value
    return ""


def clean_empty_response(text: str) -> str:
    trimmed = text.strip()
    if trimmed in ("", "_No response_", "_None._") or trimmed.lower() == "none":
        return ""
    return trimmed


def generate_reports(
    daily_issues: list[dict[str, Any]],
    title_date_str: str,
    client: GitHubClient,
) -> tuple[str, list[dict[str, Any]]]:
    parking_lot_items: list[dict[str, Any]] = []
    markdown_body = generate_markdown_report(
        daily_issues, title_date_str, client, parking_lot_items
    )
    return markdown_body, parking_lot_items


def generate_markdown_report(
    daily_issues: list[dict[str, Any]],
    title_date_str: str,
    client: GitHubClient,
    parking_lot_items: list[dict[str, Any]],
) -> str:
    markdown_body = f"# Daily Update Summary - {title_date_str}\n\n"
    markdown_body += (
        f"**Team Updates:** {len(daily_issues)} member(s) reported today\n\n"
    )
    markdown_body += "---\n\n"

    for issue in daily_issues:
        author = issue.get("user", {}).get("login", "unknown")
        display_name = issue.get("user", {}).get("display_name") or author
        avatar_url = issue.get("user", {}).get("avatar_url") or issue.get(
            "user", {}
        ).get("avatar_url", "")
        issue_number = issue.get("number")
        issue_url = issue.get("html_url")
        sections = parse_issue_body(issue.get("body", ""))

        parking_flag = " 🚨 **PARKING LOT**" if sections["parking_lot"] else ""
        markdown_body += (
            f'## <img src="{avatar_url}" width="24" height="24" '
            f'style="border-radius: 50%; vertical-align: middle;"> '
            f"{display_name} (@{author}){parking_flag} | #{issue_number}\n\n"
        )

        markdown_body += "### ✅ What did you do yesterday?\n\n"
        markdown_body += section_or_default(sections["yesterday"])
        markdown_body += "\n\n### 🎯 What will you do today?\n\n"
        markdown_body += section_or_default(sections["today"])
        markdown_body += "\n\n"

        if sections["blockers"]:
            markdown_body += f"### 🚧 Any blockers?\n\n{sections['blockers']}\n\n"
        if sections["parking_lot_details"]:
            markdown_body += (
                f"### 📝 Parking Lot Details\n\n{sections['parking_lot_details']}\n\n"
            )
        if sections["additional_comments"]:
            markdown_body += (
                f"### 💬 Additional Comments\n\n{sections['additional_comments']}\n\n"
            )

        markdown_body += "---\n\n"

        if sections["parking_lot"]:
            parking_lot_items.append(
                {
                    "author": author,
                    "display_name": display_name,
                    "avatar_url": avatar_url,
                    "issue_url": issue_url,
                    "issue_number": issue_number,
                }
            )

    if parking_lot_items:
        markdown_body += f"## 🚨 Parking Lot Items ({len(parking_lot_items)})\n\n"
        markdown_body += (
            "The following team members requested follow-up or escalation:\n\n"
        )
        for item in parking_lot_items:
            markdown_body += (
                f'- <img src="{item["avatar_url"]}" width="20" height="20" '
                f'style="border-radius: 50%; vertical-align: middle;"> '
                f"**{item['display_name']} (@{item['author']})** - "
                f"[Issue #{item['issue_number']}]({item['issue_url']})\n"
            )
        markdown_body += "\n"

    markdown_body += "---\n\n"
    markdown_body += "_Generated automatically from GitHub issues._\n"
    markdown_body += f"_Repository: {client.owner}/{client.repo}_\n"
    return markdown_body


def section_or_default(value: str) -> str:
    return value if value else "_No information provided_"


def write_summary(
    summary_path: str,
    report_issue: dict[str, Any],
    parking_lot_items: list[dict[str, Any]],
    issues: list[dict[str, Any]],
    *,
    date_str: str,
    message: str | None = None,
) -> None:
    if not summary_path:
        return

    lines: list[str] = []
    if message:
        lines.append(f"## {date_str}")
        lines.append("")
        lines.append(message)
    else:
        lines.append(f"👉 [**Full Daily Report**]({report_issue['html_url']})")
        lines.append("")
        if parking_lot_items:
            lines.append(f"Parking lot requests: **{len(parking_lot_items)}**")
            lines.append("")
        lines.append("## Team Members")
        lines.append("")
        for issue in issues:
            display_name = issue.get("user", {}).get("display_name") or issue.get(
                "user", {}
            ).get("login", "unknown")
            login = issue.get("user", {}).get("login", "unknown")
            lines.append(
                f"- {display_name} (@{login}) - [#{issue['number']}]({issue['html_url']})"
            )

    with open(summary_path, "a", encoding="utf-8") as handle:
        handle.write("\n".join(lines).rstrip() + "\n")
