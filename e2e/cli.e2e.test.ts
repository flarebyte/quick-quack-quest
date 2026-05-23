import { describe, expect, test } from "bun:test";
import { spawnSync } from "node:child_process";
import { join } from "node:path";

const repoRoot = join(import.meta.dir, "..");
const configPath = "doc/design-meta/examples/config/cli-config.cue";

type RunResult = {
  code: number;
  stdout: string;
  stderr: string;
};

function runCLI(args: string[], env: Record<string, string> = {}): RunResult {
  const cmdArgs = ["run", "./cmd/quick-quack-quest", ...args];
  const proc = spawnSync("go", cmdArgs, {
    cwd: repoRoot,
    encoding: "utf8",
    env: {
      ...process.env,
      GOTOOLCHAIN: "local",
      GOCACHE: join(repoRoot, ".gocache"),
      GOMODCACHE: join(repoRoot, ".gomodcache"),
      ...env,
    },
  });
  return {
    code: proc.status ?? 1,
    stdout: proc.stdout ?? "",
    stderr: proc.stderr ?? "",
  };
}

describe("quick-quack-quest e2e", () => {
  test("version json includes output schema version", () => {
    const res = runCLI(["version", "--format", "json"]);
    expect(res.code).toBe(0);
    const out = JSON.parse(res.stdout);
    expect(out.output_schema_version).toBe("v1");
    expect(out.name).toBe("quick-quack-quest");
  });

  test("config validate succeeds", () => {
    const res = runCLI(["config", "validate", "--format", "json", "--config", configPath]);
    expect(res.code).toBe(0);
    const out = JSON.parse(res.stdout);
    expect(out.status).toBe("ok");
  });

  test("dataset validate duckdb succeeds", () => {
    const res = runCLI([
      "dataset",
      "validate",
      "customers_master",
      "--format",
      "json",
      "--config",
      configPath,
      "--validation-engine",
      "duckdb",
    ]);
    expect(res.code).toBe(0);
    const out = JSON.parse(res.stdout);
    expect(out.status).toBe("ok");
    expect(out.output_schema_version).toBe("v1");
  });

  test("dataset validate native succeeds", () => {
    const res = runCLI([
      "dataset",
      "validate",
      "events_stream",
      "--format",
      "json",
      "--config",
      configPath,
      "--validation-engine",
      "native",
    ]);
    expect(res.code).toBe(0);
    const out = JSON.parse(res.stdout);
    expect(out.status).toBe("ok");
  });

  test("dataset inspect returns columns", () => {
    const res = runCLI([
      "dataset",
      "inspect",
      "sales_daily",
      "--format",
      "json",
      "--config",
      configPath,
      "--sample-size",
      "10",
    ]);
    expect(res.code).toBe(0);
    const out = JSON.parse(res.stdout);
    expect(out.status).toBe("ok");
    expect(Array.isArray(out.observed_columns)).toBe(true);
    expect(out.observed_columns.length).toBeGreaterThan(0);
  });

  test("query list and explain work", () => {
    const listRes = runCLI(["query", "list", "--format", "json", "--config", configPath]);
    expect(listRes.code).toBe(0);
    const list = JSON.parse(listRes.stdout);
    expect(list.length).toBeGreaterThan(0);

    const explainRes = runCLI([
      "query",
      "explain",
      "sales_by_country",
      "--format",
      "json",
      "--config",
      configPath,
    ]);
    expect(explainRes.code).toBe(0);
    const explain = JSON.parse(explainRes.stdout);
    expect(explain.query_id).toBe("sales_by_country");
  });

  test("query run jsonl streaming works", () => {
    const res = runCLI([
      "query",
      "run",
      "sales_by_country",
      "--format",
      "jsonl",
      "--stream",
      "--config",
      configPath,
      "--param",
      "start_date=2026-01-01",
      "--param",
      "end_date=2026-01-31",
      "--limit",
      "2",
      "--max-rows",
      "10",
    ]);
    expect(res.code).toBe(0);
    const lines = res.stdout.trim().split("\n").filter(Boolean);
    expect(lines.length).toBeGreaterThan(0);
    const row = JSON.parse(lines[0] ?? "{}");
    expect(row.country).toBeDefined();
    expect(res.stderr).toContain("\"query_id\": \"sales_by_country\"");
  });

  test("query run guardrail fails when limit exceeds max rows", () => {
    const res = runCLI([
      "query",
      "run",
      "sales_by_country",
      "--format",
      "json",
      "--config",
      configPath,
      "--param",
      "start_date=2026-01-01",
      "--param",
      "end_date=2026-01-31",
      "--limit",
      "100",
      "--max-rows",
      "1",
    ]);
    expect(res.code).not.toBe(0);
    expect(res.stderr).toContain("QQQ_QUERY_LIMIT_EXCEEDS_MAX_ROWS");
  });
});

