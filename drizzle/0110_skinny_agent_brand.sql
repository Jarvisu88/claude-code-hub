ALTER TABLE "keys" ADD COLUMN "price_provider_group" varchar(200);--> statement-breakpoint
ALTER TABLE "keys" ADD COLUMN "latency_provider_group" varchar(200);--> statement-breakpoint
ALTER TABLE "providers" ADD COLUMN "latency_probe_enabled" boolean;--> statement-breakpoint
ALTER TABLE "providers" ADD COLUMN "latency_probe_model" varchar(128);--> statement-breakpoint
ALTER TABLE "providers" ADD COLUMN "latency_probe_interval_ms" integer;--> statement-breakpoint
ALTER TABLE "providers" ADD COLUMN "latency_probe_time_start" varchar(5);--> statement-breakpoint
ALTER TABLE "providers" ADD COLUMN "latency_probe_time_end" varchar(5);--> statement-breakpoint
ALTER TABLE "providers" ADD COLUMN "latency_probe_last_avg_ms" integer;--> statement-breakpoint
ALTER TABLE "providers" ADD COLUMN "latency_probe_last_status" varchar(20);--> statement-breakpoint
ALTER TABLE "providers" ADD COLUMN "latency_probe_last_run_at" timestamp with time zone;--> statement-breakpoint
ALTER TABLE "users" ADD COLUMN "price_provider_group" varchar(200);--> statement-breakpoint
ALTER TABLE "users" ADD COLUMN "latency_provider_group" varchar(200);