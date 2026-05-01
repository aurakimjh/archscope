import { _electron as electron, expect, test } from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const appRoot = path.resolve(__dirname, "..");

test("Demo Data Center runs stubbed scenario and hands JSON to Export Center", async () => {
  const app = await electron.launch({
    args: ["."],
    cwd: appRoot,
    env: {
      ...process.env,
      ARCHSCOPE_E2E_DEMO_STUB: "1",
      ARCHSCOPE_E2E_RENDERER_DIST: "1",
    },
  });

  try {
    const page = await app.firstWindow();
    await page.waitForLoadState("domcontentloaded");
    await page.getByTestId("nav-demo-data").click();

    await expect(page.getByText("e2e-demo-root")).toBeVisible();
    await expect(page.getByText("1 scenarios")).toBeVisible();

    await page.getByTestId("demo-run-button").click();

    await expect(page.getByRole("heading", { name: "synthetic/e2e-smoke" })).toBeVisible();
    await expect(page.getByText("access-log-result.json", { exact: true })).toBeVisible();

    await page.getByTestId("demo-send-export").first().click();

    await expect(page.getByText("/tmp/archscope-e2e/access-log-result.json")).toBeVisible();
  } finally {
    await app.close();
  }
});
