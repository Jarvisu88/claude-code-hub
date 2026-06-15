import type { ProviderType } from "@/types/provider";

/**
 * Default test models for each provider type.
 * Shared by the single-provider API test button and the latency probe service
 * to ensure consistent behavior.
 */
export const DEFAULT_TEST_MODELS: Record<ProviderType, string> = {
  claude: "claude-haiku-4-5-20251001",
  "claude-auth": "claude-haiku-4-5-20251001",
  codex: "gpt-5.5",
  "openai-compatible": "gpt-4.1-mini",
  gemini: "gemini-2.5-flash",
  "gemini-cli": "gemini-2.5-flash",
};
