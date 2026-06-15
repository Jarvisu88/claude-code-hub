CREATE TYPE "public"."provider_sort_strategy" AS ENUM('none', 'price', 'latency');--> statement-breakpoint
ALTER TABLE "keys" ADD COLUMN "sort_strategy" "provider_sort_strategy" DEFAULT 'none' NOT NULL;--> statement-breakpoint
ALTER TABLE "users" ADD COLUMN "sort_strategy" "provider_sort_strategy" DEFAULT 'none' NOT NULL;