CREATE TABLE IF NOT EXISTS "provider_upstream_rate_sync_configs" (
	"id" serial PRIMARY KEY NOT NULL,
	"provider_id" integer NOT NULL,
	"source" varchar(20) NOT NULL,
	"is_enabled" boolean DEFAULT true NOT NULL,
	"base_url" text NOT NULL,
	"api_key" text,
	"key_name" varchar(255),
	"access_token" text,
	"refresh_token" text,
	"token_expires_at" bigint,
	"user_id" varchar(128),
	"sync_interval_minutes" integer DEFAULT 60 NOT NULL,
	"last_synced_at" timestamp with time zone,
	"last_sync_ok" boolean,
	"last_sync_rate" numeric(10, 4),
	"last_sync_error" text,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
ALTER TABLE "provider_upstream_rate_sync_configs" ADD CONSTRAINT "provider_upstream_rate_sync_configs_provider_id_providers_id_fk" FOREIGN KEY ("provider_id") REFERENCES "public"."providers"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
CREATE UNIQUE INDEX IF NOT EXISTS "uniq_provider_upstream_rate_sync_provider" ON "provider_upstream_rate_sync_configs" USING btree ("provider_id");--> statement-breakpoint
CREATE INDEX IF NOT EXISTS "idx_provider_upstream_rate_sync_enabled" ON "provider_upstream_rate_sync_configs" USING btree ("is_enabled","last_synced_at");
