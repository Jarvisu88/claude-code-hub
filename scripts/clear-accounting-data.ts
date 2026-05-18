import { drizzle } from "drizzle-orm/postgres-js";
import postgres from "postgres";
import { sql } from "drizzle-orm";
import { usageLedger, providers, modelSellMultipliers } from "../src/drizzle/schema";

const connectionString = process.env.DSN;
if (!connectionString) {
  console.error("❌ DSN environment variable is not set");
  process.exit(1);
}

const client = postgres(connectionString);
const db = drizzle(client);

async function clearAccountingData() {
  console.log("Starting to clear accounting data...");

  // 1. 清空 usage_ledger 表
  const ledgerResult = await db.execute(sql`
    DELETE FROM ${usageLedger}
  `);
  console.log(`✓ Cleared usage_ledger: ${ledgerResult.rowCount ?? 0} rows deleted`);

  // 2. 重置 providers 的 sell_multiplier
  const providerResult = await db.execute(sql`
    UPDATE ${providers}
    SET sell_multiplier = NULL
    WHERE sell_multiplier IS NOT NULL
  `);
  console.log(`✓ Reset provider sell_multiplier: ${providerResult.rowCount ?? 0} rows updated`);

  // 3. 清空 model_sell_multipliers 表
  const modelResult = await db.execute(sql`
    DELETE FROM ${modelSellMultipliers}
  `);
  console.log(`✓ Cleared model_sell_multipliers: ${modelResult.rowCount ?? 0} rows deleted`);

  // 4. 显示统计信息
  const stats = await db.execute(sql`
    SELECT
      (SELECT COUNT(*) FROM ${usageLedger}) as ledger_count,
      (SELECT COUNT(*) FROM ${providers} WHERE deleted_at IS NULL) as provider_count,
      (SELECT COUNT(*) FROM ${modelSellMultipliers}) as model_multiplier_count
  `);

  console.log("\nCurrent state:");
  console.log(`  - usage_ledger: ${stats[0]?.ledger_count ?? 0} rows`);
  console.log(`  - providers: ${stats[0]?.provider_count ?? 0} active`);
  console.log(`  - model_sell_multipliers: ${stats[0]?.model_multiplier_count ?? 0} rows`);

  console.log("\n✅ Accounting data cleared successfully!");
  await client.end();
  process.exit(0);
}

clearAccountingData().catch(async (error) => {
  console.error("❌ Error clearing accounting data:", error);
  await client.end();
  process.exit(1);
});
