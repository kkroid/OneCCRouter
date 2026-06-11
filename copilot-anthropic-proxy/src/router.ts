import { readFile } from "node:fs/promises";
import { writeFileSync } from "node:fs";

// ---------- Types ----------
export interface Provider {
  name: string;
  prefix: string;
  baseUrl: string;
  apiKey: string;
  models: string[];
}

let providers: Provider[] = [];
let modelMap = new Map<string, Provider>(); // "cp/claude-opus-4.8" → Provider

// ---------- Env parsing ----------
// Format: PROVIDER_<PREFIX>_<FIELD>  e.g. PROVIDER_DS_BASE_URL
export function parseProviders(): Provider[] {
  const group: Record<string, any> = {};
  for (const [k, v] of Object.entries(process.env)) {
    const m = k.match(/^PROVIDER_([A-Z0-9]+)_(.+)$/);
    if (!m) continue;
    const prefix = m[1].toLowerCase();
    const field = m[2].toLowerCase();
    if (!group[prefix]) group[prefix] = { prefix };
    group[prefix][field] = v;
  }

  const result: Provider[] = [];
  for (const p of Object.values(group)) {
    if (!p.models) continue;
    result.push({
      name: p.name || p.prefix.toUpperCase(),
      prefix: p.prefix,
      baseUrl: p.base_url || "",
      apiKey: p.api_key || "",
      models: (p.models || "").split(",").map((s: string) => s.trim()).filter(Boolean),
    });
  }

  providers = result;
  modelMap.clear();
  for (const p of providers) {
    for (const m of p.models) {
      modelMap.set(p.prefix + "/" + m, p);
    }
  }
  return result;
}

// ---------- Lookup ----------
export function getProviders(): Provider[] {
  return providers;
}

export function resolveModel(fullName: string): { provider: Provider; model: string } | null {
  // Try exact match "cp/claude-opus-4.8"
  const p = modelMap.get(fullName);
  if (p) return { provider: p, model: fullName.split("/").slice(1).join("/") };

  // Try prefix-only match "cp" — use first model of that provider
  for (const provider of providers) {
    if (fullName === provider.prefix && provider.models.length > 0) {
      return { provider, model: provider.models[0] };
    }
  }

  return null;
}

export function getAllModelIds(): string[] {
  return [...modelMap.keys()];
}

export function getCopilotProvider(): Provider | undefined {
  return providers.find(p => p.prefix === "cp");
}

// ---------- Claude Code settings generation ----------
export function generateSettings(outDir: string) {
  const ids = getAllModelIds();

  const settings = {
    env: {
      ANTHROPIC_BASE_URL: "http://localhost:3456/v1",
      ANTHROPIC_AUTH_TOKEN: "x",
      ANTHROPIC_MODEL: process.env.ANTHROPIC_MODEL || ids[0] || "",
      ANTHROPIC_DEFAULT_OPUS_MODEL: process.env.ANTHROPIC_DEFAULT_OPUS_MODEL || ids[0] || "",
      ANTHROPIC_DEFAULT_SONNET_MODEL: process.env.ANTHROPIC_DEFAULT_SONNET_MODEL || ids[1] || ids[0] || "",
      ANTHROPIC_DEFAULT_HAIKU_MODEL: process.env.ANTHROPIC_DEFAULT_HAIKU_MODEL || ids[2] || ids[0] || "",
      ANTHROPIC_DEFAULT_FABLE_MODEL: process.env.ANTHROPIC_DEFAULT_FABLE_MODEL || ids[3] || ids[0] || "",
      CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC: "1",
      CLAUDE_CODE_EFFORT_LEVEL: "max",
    },
    theme: "dark",
    skipWorkflowUsageWarning: true,
    _availableModels: ids.map(id => {
      const p = modelMap.get(id);
      return { id, name: (p?.name || "") + " " + id.split("/")[1] };
    }),
  };

  const path = outDir.replace(/\/$/, "") + "/claude-code-settings.json";
  writeFileSync(path, JSON.stringify(settings, null, 2) + "\n");
  console.log("Generated", path, "with", ids.length, "models");
}

// ---------- Startup check ----------
export async function requiresToken(): Promise<boolean> {
  const hasCp = providers.some(p => p.prefix === "cp");
  if (!hasCp) return false;

  const tokenFile = process.env.GITHUB_TOKEN_FILE;
  if (!tokenFile) return true;

  try {
    const content = await readFile(tokenFile, "utf-8");
    return content.trim().length === 0;
  } catch {
    return true;
  }
}
