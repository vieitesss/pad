from __future__ import annotations

import os
import sys

from daily_update_workflow import (
    GitHubClient,
    close_individual_issues,
    collect_daily_update_issues,
    create_or_reuse_report_issue,
    enrich_user_info,
    generate_reports,
    require_env,
    update_issue_body,
    write_summary,
)


def main() -> int:
    client = GitHubClient(require_env("GITHUB_TOKEN"), require_env("GITHUB_REPOSITORY"))
    report_date = os.getenv("REPORT_DATE", "")
    daily_label = (
        os.getenv("DAILY_UPDATE_LABEL", "daily-update").strip() or "daily-update"
    )
    report_label = (
        os.getenv("DAILY_REPORT_LABEL", "daily-update/report").strip()
        or "daily-update/report"
    )
    summary_path = os.getenv("GITHUB_STEP_SUMMARY", "")

    issues, date_str, title_date_str = collect_daily_update_issues(
        client, report_date, daily_label
    )
    if not issues:
        message = f"No daily update issues found for {date_str}."
        print(message)
        write_summary(summary_path, {}, [], [], date_str=date_str, message=message)
        return 0

    issues = enrich_user_info(client, issues)
    issues.sort(key=lambda issue: issue.get("user", {}).get("login", "").lower())

    report_issue = create_or_reuse_report_issue(
        client, title_date_str, "Generating report...", report_label
    )
    markdown_body, parking_lot_items = generate_reports(issues, title_date_str, client)
    update_issue_body(client, report_issue["number"], markdown_body)
    _, error_count = close_individual_issues(client, issues, report_issue["html_url"])
    write_summary(
        summary_path, report_issue, parking_lot_items, issues, date_str=date_str
    )

    print(report_issue["html_url"])
    if error_count:
        print(f"Completed with {error_count} member issue error(s)")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:  # pragma: no cover
        print(f"Error: {exc}", file=sys.stderr)
        raise SystemExit(1) from exc
