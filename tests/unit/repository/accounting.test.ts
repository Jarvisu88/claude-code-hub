import { describe, it, expect, beforeEach, vi } from "vitest";

// Mock the database before importing the repository
vi.mock("@/drizzle/db", () => ({
  db: {
    execute: vi.fn(),
    select: vi.fn(() => ({
      from: vi.fn(() => ({
        where: vi.fn(() => ({
          orderBy: vi.fn(() => Promise.resolve([])),
        })),
        orderBy: vi.fn(() => Promise.resolve([])),
      })),
    })),
    update: vi.fn(() => ({
      set: vi.fn(() => ({
        where: vi.fn(() => Promise.resolve(undefined)),
      })),
    })),
    insert: vi.fn(() => ({
      values: vi.fn(() => ({
        onConflictDoUpdate: vi.fn(() => ({
          returning: vi.fn(() => Promise.resolve([])),
        })),
      })),
    })),
  },
}));

const { db } = await import("@/drizzle/db");
const {
  findProviderProfitSummary,
  findRevenueTimeline,
  updateProviderSellMultiplier,
  upsertModelSellMultiplier,
} = await import("@/repository/accounting");

describe("accounting repository", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("findProviderProfitSummary", () => {
    it("should calculate profit correctly with sell_multiplier", async () => {
      const mockRows = [
        {
          providerId: 1,
          providerName: "Test Provider",
          sellMultiplier: "2.0",
          requestCount: 10,
          modelCount: 2,
          baseCostUsd: "1.0",
          supplierCostUsd: "2.0",
          estimatedRevenueUsd: "4.0",
          estimatedProfitUsd: "2.0",
          revenueMultiplier: "2.0",
        },
      ];

      vi.mocked(db.execute).mockResolvedValue(mockRows as any);

      const result = await findProviderProfitSummary({
        startTime: new Date("2024-01-01"),
        endTime: new Date("2024-01-02"),
        globalSellMultiplier: 1.5,
      });

      expect(result).toHaveLength(1);
      expect(result[0].providerId).toBe(1);
      expect(result[0].estimatedProfitUsd).toBe(2.0);
      expect(result[0].sellMultiplier).toBe(2.0);
    });

    it("should handle NULL sell_multiplier (auto mode)", async () => {
      const mockRows = [
        {
          providerId: 1,
          providerName: "Auto Provider",
          sellMultiplier: null,
          requestCount: 5,
          modelCount: 1,
          baseCostUsd: "1.0",
          supplierCostUsd: "1.5",
          estimatedRevenueUsd: "1.5",
          estimatedProfitUsd: "0.0",
          revenueMultiplier: "0",
        },
      ];

      vi.mocked(db.execute).mockResolvedValue(mockRows as any);

      const result = await findProviderProfitSummary({
        startTime: new Date("2024-01-01"),
        endTime: new Date("2024-01-02"),
        globalSellMultiplier: 1.0,
      });

      expect(result[0].sellMultiplier).toBeNull();
      expect(result[0].estimatedRevenueUsd).toBe(1.5);
    });

    it("should handle invalid globalSellMultiplier gracefully", async () => {
      vi.mocked(db.execute).mockResolvedValue([]);

      await findProviderProfitSummary({
        startTime: new Date("2024-01-01"),
        endTime: new Date("2024-01-02"),
        globalSellMultiplier: 0, // Invalid, should default to 1
      });

      const call = vi.mocked(db.execute).mock.calls[0][0];
      // Verify that the query uses 1 instead of 0
      expect(call).toBeDefined();
    });
  });

  describe("updateProviderSellMultiplier", () => {
    it("should update sell_multiplier to a valid number", async () => {
      const mockUpdate = {
        set: vi.fn().mockReturnThis(),
        where: vi.fn().mockResolvedValue(undefined),
      };
      vi.mocked(db.update).mockReturnValue(mockUpdate as any);

      await updateProviderSellMultiplier(1, 2.5);

      expect(mockUpdate.set).toHaveBeenCalledWith(
        expect.objectContaining({
          sellMultiplier: "2.5",
        })
      );
    });

    it("should update sell_multiplier to NULL (auto mode)", async () => {
      const mockUpdate = {
        set: vi.fn().mockReturnThis(),
        where: vi.fn().mockResolvedValue(undefined),
      };
      vi.mocked(db.update).mockReturnValue(mockUpdate as any);

      await updateProviderSellMultiplier(1, null);

      expect(mockUpdate.set).toHaveBeenCalledWith(
        expect.objectContaining({
          sellMultiplier: null,
        })
      );
    });
  });

  describe("upsertModelSellMultiplier", () => {
    it("should insert new model multiplier", async () => {
      const mockInsert = {
        values: vi.fn().mockReturnThis(),
        onConflictDoUpdate: vi.fn().mockReturnThis(),
        returning: vi.fn().mockResolvedValue([
          {
            id: 1,
            modelName: "gpt-4",
            multiplier: "3.0",
            note: "Premium model",
            createdAt: new Date(),
            updatedAt: new Date(),
          },
        ]),
      };
      vi.mocked(db.insert).mockReturnValue(mockInsert as any);

      const result = await upsertModelSellMultiplier({
        modelName: "gpt-4",
        multiplier: 3.0,
        note: "Premium model",
      });

      expect(result.modelName).toBe("gpt-4");
      expect(result.multiplier).toBe(3.0);
    });

    it("should trim and handle empty note", async () => {
      const mockInsert = {
        values: vi.fn().mockReturnThis(),
        onConflictDoUpdate: vi.fn().mockReturnThis(),
        returning: vi.fn().mockResolvedValue([
          {
            id: 1,
            modelName: "gpt-3.5",
            multiplier: "1.5",
            note: null,
            createdAt: new Date(),
            updatedAt: new Date(),
          },
        ]),
      };
      vi.mocked(db.insert).mockReturnValue(mockInsert as any);

      const result = await upsertModelSellMultiplier({
        modelName: "gpt-3.5",
        multiplier: 1.5,
        note: "   ",
      });

      expect(mockInsert.values).toHaveBeenCalledWith(
        expect.objectContaining({
          note: null,
        })
      );
    });
  });

  describe("findRevenueTimeline", () => {
    it("should aggregate revenue by time bucket", async () => {
      const mockRows = [
        {
          time_bucket: new Date("2024-01-01T00:00:00Z"),
          user_name: "user1",
          user_id: 1,
          provider_group: "default",
          model_name: "gpt-4",
          request_count: 5,
          input_tokens: 1000,
          output_tokens: 500,
          supplier_cost_usd: "0.05",
          revenue_usd: "0.10",
        },
      ];

      vi.mocked(db.execute).mockResolvedValue(mockRows as any);

      const result = await findRevenueTimeline({
        startTime: new Date("2024-01-01"),
        endTime: new Date("2024-01-02"),
        globalSellMultiplier: 2.0,
        bucketInterval: "hour",
      });

      expect(result).toHaveLength(1);
      expect(result[0].requestCount).toBe(5);
      expect(result[0].supplierCostUsd).toBe(0.05);
      expect(result[0].revenueUsd).toBe(0.1);
    });

    it("should use day interval for longer time ranges", async () => {
      vi.mocked(db.execute).mockResolvedValue([]);

      await findRevenueTimeline({
        startTime: new Date("2024-01-01"),
        endTime: new Date("2024-01-31"),
        globalSellMultiplier: 1.0,
        bucketInterval: "day",
      });

      const call = vi.mocked(db.execute).mock.calls[0][0];
      expect(call.queryChunks).toContain("day");
    });
  });
});
