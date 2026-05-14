CREATE TABLE IF NOT EXISTS "model_sell_multipliers" (
	"id" serial PRIMARY KEY NOT NULL,
	"model_name" varchar(128) NOT NULL,
	"multiplier" numeric(10, 4) DEFAULT '1.0' NOT NULL,
	"note" varchar(200),
	"created_at" timestamp with time zone DEFAULT now(),
	"updated_at" timestamp with time zone DEFAULT now(),
	CONSTRAINT "model_sell_multipliers_model_name_unique" UNIQUE("model_name")
);
--> statement-breakpoint
ALTER TABLE "system_settings" ADD COLUMN IF NOT EXISTS "global_sell_multiplier" numeric(10, 4) DEFAULT '1.0' NOT NULL;--> statement-breakpoint
CREATE INDEX IF NOT EXISTS "idx_model_sell_multiplier_name" ON "model_sell_multipliers" USING btree ("model_name");
