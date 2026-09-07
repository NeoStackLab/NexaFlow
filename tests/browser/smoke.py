import os
import re
from pathlib import Path

from playwright.sync_api import expect, sync_playwright


base_url = os.environ.get("NEXAFLOW_WEB_URL", "http://127.0.0.1:3000").rstrip("/")
artifact_dir = Path(os.environ.get("NEXAFLOW_BROWSER_ARTIFACTS", "artifacts"))
artifact_dir.mkdir(parents=True, exist_ok=True)

with sync_playwright() as playwright:
    browser = playwright.chromium.launch(headless=True)
    page = browser.new_page(viewport={"width": 1440, "height": 1000})
    page_errors: list[str] = []
    page.on("pageerror", lambda error: page_errors.append(str(error)))
    try:
        health = page.request.get(f"{base_url}/api/health")
        assert health.ok, f"web health endpoint returned HTTP {health.status}"
        assert health.json()["status"] == "ok", "web health endpoint is not ready"

        page.goto(f"{base_url}/install", wait_until="networkidle")
        expect(page).to_have_title(re.compile("NexaFlow"))
        expect(page.get_by_text("01 / WELCOME TO NEXAFLOW")).to_be_visible()
        continue_button = page.get_by_role("button", name=re.compile("Continue|下一步"))
        expect(continue_button).to_be_enabled()
        continue_button.click()

        expect(page.get_by_text(re.compile("Service readiness|容器服务就绪"))).to_be_visible()
        expect(page.locator(".install-status.pass")).to_have_count(4, timeout=30_000)
        expect(page.locator(".install-status.fail")).to_have_count(0)
        assert not page_errors, f"browser page errors: {page_errors}"
    except Exception:
        page.screenshot(path=str(artifact_dir / "browser-smoke-failure.png"), full_page=True)
        raise
    finally:
        browser.close()
